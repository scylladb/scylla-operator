//go:build envtest

// Copyright (c) 2026 ScyllaDB.

package controllers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	sidecarcontroller "github.com/scylladb/scylla-operator/pkg/controller/sidecar"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
)

var _ = g.Describe("SidecarController", func() {
	const (
		nodeServiceName  = "dc1-rack1-0"
		localhostAddress = "127.0.0.1"
		localIP          = "10.0.0.1"
		hostID           = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	)

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	// waitForServiceAnnotations polls the Service until it satisfies the given assertion and returns it at that point.
	waitForServiceAnnotations := func(ctx context.Context, svcName string, assert func(o.Gomega, *corev1.Service)) *corev1.Service {
		g.GinkgoHelper()

		var svc *corev1.Service
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			var err error
			svc, err = env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			assert(eo, svc)
		}).WithTimeout(30 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())

		return svc
	}

	// consistentlyAssertServiceAnnotations asserts the Service keeps satisfying the given assertion over time.
	consistentlyAssertServiceAnnotations := func(ctx context.Context, svcName string, assert func(o.Gomega, *corev1.Service)) {
		g.GinkgoHelper()

		o.Consistently(func(eo o.Gomega, ctx context.Context) {
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			assert(eo, svc)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())
	}

	// expectedTokenRingHash returns the token ring hash the controller is expected to annotate for the given ring.
	expectedTokenRingHash := func(ringTokens []string) string {
		g.GinkgoHelper()

		h, err := hash.HashObjects(ringTokens)
		o.Expect(err).NotTo(o.HaveOccurred())

		return h
	}

	// annotationTestCase covers a sidecar sync against a ScyllaDB API whose responses don't change during the spec.
	type annotationTestCase struct {
		// existingAnnotations is what the Service carries before the controller starts.
		existingAnnotations map[string]string
		// fake is the ScyllaDB API the sidecar runs against.
		fake fakeScyllaDBTokenMetadata
		// expectedAnnotations must all eventually be present with the given values.
		expectedAnnotations map[string]string
		// expectedTokenRingHashOf, when set, additionally expects the token ring hash annotation to hold the hash of
		// these tokens. It's kept separate from expectedAnnotations because the hash has to be computed at run time.
		expectedTokenRingHashOf []string
		// absentAnnotations must never be set.
		absentAnnotations []string
	}

	syncsAnnotations := func(ctx g.SpecContext, tc annotationTestCase) {
		g.By("Creating a member Service")
		createNodeService(ctx, env, nodeServiceName, tc.existingAnnotations)

		g.By("Running the SidecarController")
		newScyllaClient := newFakeScyllaDBClientFactory(newFakeScyllaDBTokenMetadataHandler(tc.fake))
		runSidecarController(ctx, env, nodeServiceName, localhostAddress, newScyllaClient)

		if len(tc.expectedAnnotations) != 0 || len(tc.expectedTokenRingHashOf) != 0 {
			g.By("Waiting for the expected annotations to be set")
			waitForServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
				for k, v := range tc.expectedAnnotations {
					eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(k, v))
				}
				if len(tc.expectedTokenRingHashOf) != 0 {
					eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.CurrentTokenRingHashAnnotation, expectedTokenRingHash(tc.expectedTokenRingHashOf)))
				}
			})
		}

		if len(tc.absentAnnotations) != 0 {
			g.By("Ensuring the annotations which must not be set are never set")
			consistentlyAssertServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
				for _, k := range tc.absentAnnotations {
					eo.Expect(svc.Annotations).NotTo(o.HaveKey(k))
				}
			})
		}
	}

	g.DescribeTable("annotates the node's membership in the ScyllaDB cluster", syncsAnnotations,
		g.Entry("when the node owns normal tokens", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			expectedAnnotations: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
				naming.HostIDAnnotation:                    hostID,
			},
			expectedTokenRingHashOf: []string{"-1", "0", "1"},
		}),
		g.Entry("when the node owns no normal tokens", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				// A bootstrapping node holds only pending tokens, which the endpoint doesn't report.
				nodeTokens: map[string][]string{localIP: {}},
				ringTokens: []string{},
			},
			expectedAnnotations: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueFalse,
				naming.HostIDAnnotation:                    hostID,
			},
			// The token ring hash is not written for a non-member.
			absentAnnotations: []string{naming.CurrentTokenRingHashAnnotation},
		}),
		g.Entry("when the token ring hash can't be fetched", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:    hostID,
				ipToHostIDMap:  []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:     map[string][]string{localIP: {"-1", "0", "1"}},
				failRingTokens: true,
			},
			expectedAnnotations: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
			},
			absentAnnotations: []string{naming.CurrentTokenRingHashAnnotation},
		}),
	)

	g.DescribeTable("annotates the HostID", syncsAnnotations,
		g.Entry("when the host ID mapping can't be fetched", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:       hostID,
				failIPToHostIDMap: true,
				ringTokens:        []string{"-1", "0", "1"},
			},
			expectedAnnotations: map[string]string{
				naming.HostIDAnnotation: hostID,
			},
			absentAnnotations: []string{naming.NodeJoinedScyllaDBClusterAnnotation, naming.CurrentTokenRingHashAnnotation},
		}),
		g.Entry("when the node's tokens can't be fetched", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID: hostID,
				// The node is present in the cluster's token metadata, but its tokens can't be read.
				ipToHostIDMap:  []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				failNodeTokens: true,
				ringTokens:     []string{"-1", "0", "1"},
			},
			expectedAnnotations: map[string]string{
				naming.HostIDAnnotation: hostID,
			},
			absentAnnotations: []string{naming.NodeJoinedScyllaDBClusterAnnotation, naming.CurrentTokenRingHashAnnotation},
		}),
	)

	g.DescribeTable("leaves the annotations untouched", syncsAnnotations,
		g.Entry("when the node is absent from the cluster's token metadata", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID: hostID,
				// The node's own host ID is absent from the mapping, so its membership can't be determined.
				ipToHostIDMap: []scyllaNodeResponse{{Key: "10.0.0.2", Value: "ffffffff-ffff-ffff-ffff-ffffffffffff"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			expectedAnnotations: map[string]string{
				naming.HostIDAnnotation: hostID,
			},
			absentAnnotations: []string{naming.NodeJoinedScyllaDBClusterAnnotation},
		}),
		g.Entry("when the ScyllaDB API is failing and a membership was already observed", annotationTestCase{
			existingAnnotations: map[string]string{
				naming.HostIDAnnotation:                    hostID,
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
				naming.CurrentTokenRingHashAnnotation:      "previous-hash",
			},
			fake: fakeScyllaDBTokenMetadata{
				localHostID:       hostID,
				failIPToHostIDMap: true,
				ringTokens:        []string{"-1", "0", "1"},
			},
			expectedAnnotations: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
				naming.CurrentTokenRingHashAnnotation:      "previous-hash",
			},
		}),
		g.Entry("when the local HostID can't be fetched", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				failLocalHostID: true,
				ipToHostIDMap:   []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:      map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:      []string{"-1", "0", "1"},
			},
			absentAnnotations: []string{naming.HostIDAnnotation, naming.NodeJoinedScyllaDBClusterAnnotation, naming.CurrentTokenRingHashAnnotation},
		}),
	)

	g.DescribeTable("annotates the token ring hash", syncsAnnotations,
		g.Entry("when the cluster has a token ring", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			expectedTokenRingHashOf: []string{"-1", "0", "1"},
		}),
		// The token ring is hashed as returned by the ScyllaDB API, without sorting, so the hash is order-sensitive.
		// Expecting the hash of the reordered ring asserts exactly that: the in-order hash would not match.
		g.Entry("when the same tokens are returned in a different order", annotationTestCase{
			fake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    []string{"1", "-1", "0"},
			},
			expectedTokenRingHashOf: []string{"1", "-1", "0"},
		}),
	)

	g.It("doesn't update the Service on subsequent syncs when nothing changed", func(ctx g.SpecContext) {
		g.By("Creating a member Service")
		createNodeService(ctx, env, nodeServiceName, nil)

		ringTokens := []string{"-1", "0", "1"}

		g.By("Running the SidecarController against a ScyllaDB owning normal tokens")
		newScyllaClient := newFakeScyllaDBClientFactory(newFakeScyllaDBTokenMetadataHandler(fakeScyllaDBTokenMetadata{
			localHostID:   hostID,
			ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
			nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
			ringTokens:    ringTokens,
		}))
		runSidecarController(ctx, env, nodeServiceName, localhostAddress, newScyllaClient)

		g.By("Waiting for the annotations to settle")
		svc := waitForServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.NodeJoinedScyllaDBClusterAnnotation, naming.LabelValueTrue))
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.CurrentTokenRingHashAnnotation, expectedTokenRingHash(ringTokens)))
		})
		settledResourceVersion := svc.ResourceVersion

		// A ResourceVersion bump would mean the controller issued an Update despite computing identical annotations.
		g.By("Ensuring the Service is not updated again")
		consistentlyAssertServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			eo.Expect(svc.ResourceVersion).To(o.Equal(settledResourceVersion))
		})
	})

	// transitionTestCase covers a sidecar sync against a ScyllaDB whose state changes mid-spec, where the change is
	// expected to be picked up and reflected in the annotations.
	type transitionTestCase struct {
		// firstFake is the ScyllaDB API served before the state change.
		firstFake fakeScyllaDBTokenMetadata
		// secondFake is the ScyllaDB API served after it.
		secondFake fakeScyllaDBTokenMetadata
		// expectedAnnotationsBefore must be set before the state change.
		expectedAnnotationsBefore map[string]string
		// absentAnnotationsBefore must not be set before the state change.
		absentAnnotationsBefore []string
		// expectedAnnotationsAfter must be set once the state change has been observed.
		expectedAnnotationsAfter map[string]string
		// expectedTokenRingHashOfAfter is the token ring whose hash is expected once the state change is observed.
		expectedTokenRingHashOfAfter []string
	}

	g.DescribeTable("annotates the node as a member", func(ctx g.SpecContext, tc transitionTestCase) {
		g.By("Creating a member Service")
		createNodeService(ctx, env, nodeServiceName, nil)

		g.By("Running the SidecarController")
		switcher, newScyllaClient := newSwitchableFakeScyllaDBClientFactory(
			newFakeScyllaDBTokenMetadataHandler(tc.firstFake),
			newFakeScyllaDBTokenMetadataHandler(tc.secondFake),
		)
		runSidecarController(ctx, env, nodeServiceName, localhostAddress, newScyllaClient)

		g.By("Waiting for the annotations expected before the state change")
		waitForServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			for k, v := range tc.expectedAnnotationsBefore {
				eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(k, v))
			}
			for _, k := range tc.absentAnnotationsBefore {
				eo.Expect(svc.Annotations).NotTo(o.HaveKey(k))
			}
		})

		g.By("Changing the state of the ScyllaDB the sidecar runs against")
		switcher.SwitchToPhaseTwo()
		o.Eventually(switcher.PhaseTwoServedCh()).WithTimeout(30 * time.Second).Should(o.BeClosed())

		g.By("Waiting for the annotations to reflect the new state")
		waitForServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			for k, v := range tc.expectedAnnotationsAfter {
				eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(k, v))
			}
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.CurrentTokenRingHashAnnotation, expectedTokenRingHash(tc.expectedTokenRingHashOfAfter)))
		})
	},
		g.Entry("when the node finishes bootstrapping", transitionTestCase{
			firstFake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				// A bootstrapping node holds only pending tokens, which the endpoint doesn't report.
				nodeTokens: map[string][]string{localIP: {}},
				ringTokens: []string{},
			},
			secondFake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			expectedAnnotationsBefore: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueFalse,
			},
			expectedAnnotationsAfter: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
			},
			expectedTokenRingHashOfAfter: []string{"-1", "0", "1"},
		}),
		g.Entry("when the node shows up in the cluster's token metadata", transitionTestCase{
			firstFake: fakeScyllaDBTokenMetadata{
				localHostID: hostID,
				// The node's own host ID is absent from the mapping, so its membership can't be determined.
				ipToHostIDMap: []scyllaNodeResponse{{Key: "10.0.0.2", Value: "ffffffff-ffff-ffff-ffff-ffffffffffff"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			secondFake: fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    []string{"-1", "0", "1"},
			},
			expectedAnnotationsBefore: map[string]string{
				naming.HostIDAnnotation: hostID,
			},
			absentAnnotationsBefore: []string{naming.NodeJoinedScyllaDBClusterAnnotation},
			expectedAnnotationsAfter: map[string]string{
				naming.NodeJoinedScyllaDBClusterAnnotation: naming.LabelValueTrue,
			},
			expectedTokenRingHashOfAfter: []string{"-1", "0", "1"},
		}),
	)

	g.It("retains the observed membership and token ring hash when the ScyllaDB API starts failing", func(ctx g.SpecContext) {
		g.By("Creating a member Service")
		createNodeService(ctx, env, nodeServiceName, nil)

		ringTokens := []string{"-1", "0", "1"}

		g.By("Running the SidecarController against a ScyllaDB owning normal tokens")
		switcher, newScyllaClient := newSwitchableFakeScyllaDBClientFactory(
			newFakeScyllaDBTokenMetadataHandler(fakeScyllaDBTokenMetadata{
				localHostID:   hostID,
				ipToHostIDMap: []scyllaNodeResponse{{Key: localIP, Value: hostID}},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
				ringTokens:    ringTokens,
			}),
			newFakeScyllaDBTokenMetadataHandler(fakeScyllaDBTokenMetadata{
				localHostID:       hostID,
				failIPToHostIDMap: true,
				ringTokens:        ringTokens,
			}),
		)
		runSidecarController(ctx, env, nodeServiceName, localhostAddress, newScyllaClient)

		g.By("Waiting for the node to be annotated as a member")
		waitForServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.NodeJoinedScyllaDBClusterAnnotation, naming.LabelValueTrue))
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.CurrentTokenRingHashAnnotation, expectedTokenRingHash(ringTokens)))
		})

		g.By("Making the ScyllaDB API fail the host ID mapping request")
		switcher.SwitchToPhaseTwo()
		o.Eventually(switcher.PhaseTwoServedCh()).WithTimeout(30 * time.Second).Should(o.BeClosed())

		g.By("Ensuring the membership and the token ring hash retain their last observed values")
		consistentlyAssertServiceAnnotations(ctx, nodeServiceName, func(eo o.Gomega, svc *corev1.Service) {
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.NodeJoinedScyllaDBClusterAnnotation, naming.LabelValueTrue))
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.CurrentTokenRingHashAnnotation, expectedTokenRingHash(ringTokens)))
		})
	})
})

// createNodeService creates a member Service with the given annotations in the test namespace.
func createNodeService(ctx context.Context, env *envtest.Environment, name string, annotations map[string]string) *corev1.Service {
	g.GinkgoHelper()

	if annotations == nil {
		annotations = map[string]string{}
	}

	svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   env.Namespace(),
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Name: "cql",
					Port: 9042,
				},
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create Service %q", naming.ManualRef(env.Namespace(), name))

	return svc
}

// fakeScyllaDBTokenMetadata describes the fake ScyllaDB API responses for the token metadata surface the sidecar
// controller reads.
type fakeScyllaDBTokenMetadata struct {
	// localHostID is returned for the local host ID request.
	localHostID string
	// ipToHostIDMap is the cluster's IP to host ID mapping.
	ipToHostIDMap []scyllaNodeResponse
	// nodeTokens maps an endpoint to the normal tokens it owns.
	nodeTokens map[string][]string
	// ringTokens is the cluster's token ring.
	ringTokens []string
	// failLocalHostID makes the local host ID request fail.
	failLocalHostID bool
	// failIPToHostIDMap makes the host ID mapping request fail.
	failIPToHostIDMap bool
	// failNodeTokens makes the per-endpoint tokens request fail.
	failNodeTokens bool
	// failRingTokens makes the token ring request fail.
	failRingTokens bool
}

// newFakeScyllaDBTokenMetadataHandler returns a handler serving the given fake ScyllaDB API responses.
func newFakeScyllaDBTokenMetadataHandler(fake fakeScyllaDBTokenMetadata) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// failWith serves a ScyllaDB-shaped JSON error so the client fails on the status code rather than on
		// decoding a plain-text body.
		failWith := func(message string) {
			w.WriteHeader(http.StatusInternalServerError)
			encodeJSON(w, r, map[string]any{"message": message, "code": http.StatusInternalServerError})
		}

		switch {
		case r.URL.Path == "/storage_service/hostid/local":
			if fake.failLocalHostID {
				failWith("local host id is unavailable")
				return
			}
			encodeJSON(w, r, fake.localHostID)

		case r.URL.Path == "/storage_service/operation_mode":
			encodeJSON(w, r, scyllaclient.OperationalModeNormal)

		case r.URL.Path == "/storage_service/host_id":
			if fake.failIPToHostIDMap {
				failWith("host id mapping is unavailable")
				return
			}
			encodeJSON(w, r, fake.ipToHostIDMap)

		// The token ring and the per-endpoint tokens share a path prefix, so match the exact path first.
		case r.URL.Path == "/storage_service/tokens":
			if fake.failRingTokens {
				failWith("token ring is unavailable")
				return
			}
			encodeJSON(w, r, fake.ringTokens)

		case strings.HasPrefix(r.URL.Path, "/storage_service/tokens/"):
			if fake.failNodeTokens {
				failWith("node tokens are unavailable")
				return
			}

			endpoint := strings.TrimPrefix(r.URL.Path, "/storage_service/tokens/")
			tokens, ok := fake.nodeTokens[endpoint]
			if !ok {
				http.NotFound(w, r)
				return
			}
			encodeJSON(w, r, tokens)

		default:
			http.NotFound(w, r)
		}
	})
}

func runSidecarController(ctx context.Context, env *envtest.Environment, serviceName, localhostAddress string, newScyllaClient func() (*scyllaclient.Client, error)) *sidecarcontroller.Controller {
	g.GinkgoHelper()

	kubeInformers := kubeinformers.NewSharedInformerFactoryWithOptions(
		env.TypedKubeClient(),
		0,
		kubeinformers.WithNamespace(env.Namespace()),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", serviceName).String()
		}),
	)

	c, err := sidecarcontroller.NewController(
		env.Namespace(),
		serviceName,
		localhostAddress,
		env.TypedKubeClient(),
		kubeInformers.Core().V1().Services(),
		newScyllaClient,
	)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create sidecar controller")

	kubeInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()

	g.DeferCleanup(func() {
		kubeInformers.Shutdown()
		wg.Wait()
	})

	return c
}
