//go:build envtest

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controller/statusreport"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
)

type scyllaNodeResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var _ = g.Describe("StatusReportController", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	newBasicPod := func(namespace string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  naming.ScyllaContainerName,
						Image: unit.ScyllaDBImage,
					},
				},
			},
		}
	}

	// waitForPodToHaveNodeStatusReportAnnotation polls the Pod until it has the expected node status report annotation value and returns the Pod at that point.
	waitForPodToHaveNodeStatusReportAnnotation := func(ctx context.Context, namespace, podName, expectedAnnotation string) *corev1.Pod {
		g.GinkgoHelper()

		var pod *corev1.Pod
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			var err error
			pod, err = env.TypedKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(pod.GetAnnotations()).To(o.HaveKeyWithValue(naming.NodeStatusReportAnnotation, expectedAnnotation))
		}).WithTimeout(30 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())

		return pod
	}

	// consistentlyAnnotationAndRV asserts that the Pod's node status report annotation value and ResourceVersion both remain stable over time.
	consistentlyAnnotationAndRV := func(ctx context.Context, namespace, podName, expectedAnnotation, expectedRV string) {
		g.GinkgoHelper()

		o.Consistently(func(eo o.Gomega) {
			pod, err := env.TypedKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(pod.GetAnnotations()).To(o.HaveKeyWithValue(naming.NodeStatusReportAnnotation, expectedAnnotation))
			eo.Expect(pod.ResourceVersion).To(o.Equal(expectedRV))
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())
	}

	type statusStabilityTestCase struct {
		// firstCallHostIDs is the list returned by the fake ScyllaDB API on the first sync.
		firstCallHostIDs []scyllaNodeResponse
		// subsequentCallHostIDs is the list returned on all subsequent syncs.
		subsequentCallHostIDs []scyllaNodeResponse
		// liveEndpoints is the set of live endpoints (same across all calls).
		liveEndpoints []string
		// expectedAnnotation is the expected value of the node status report annotation.
		expectedAnnotation string
	}

	g.DescribeTable("keeps the Pod annotation stable with respect to the ordering of status reports", func(ctx g.SpecContext, tc statusStabilityTestCase) {
		// Liveness is intentionally kept the same across both phases to focus on reordering changes across subsequent API calls.
		scyllaClientFactory := NewSwitchableScyllaClientFactory(
			tc.firstCallHostIDs, tc.liveEndpoints,
			tc.subsequentCallHostIDs, tc.liveEndpoints,
		)
		g.DeferCleanup(scyllaClientFactory.Close)

		g.By("Creating the Pod")
		pod := newBasicPod(env.Namespace())
		pod, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Starting status report controller")
		ctrl := runStatusReportController(ctx, env, pod.Name, scyllaClientFactory.NewClient)

		g.By("Waiting for the Pod to get the expected node status report annotation")
		pod = waitForPodToHaveNodeStatusReportAnnotation(ctx, pod.Namespace, pod.Name, tc.expectedAnnotation)
		initialResourceVersion := pod.ResourceVersion

		// Switch to phase two and explicitly enqueue a sync so the controller re-runs with the reordered API response.
		// phaseTwoCalled is then used to confirm the sync actually executed before asserting stability.
		g.By("Switching to phase two and triggering a sync")
		scyllaClientFactory.SwitchToPhaseTwo()
		ctrl.Enqueue()

		o.Eventually(scyllaClientFactory.PhaseTwoCalledCh()).WithTimeout(30 * time.Second).Should(o.BeClosed())

		// If the controller produces a different JSON encoding on the second call (e.g., due to non-deterministic node ordering),
		// the annotation value will change and the Pod will be re-patched, bumping ResourceVersion.
		// A stable annotation value and ResourceVersion together confirm no spurious patch occurred.
		g.By("Ensuring the Pod annotation remains stable and the Pod is not spuriously re-patched on subsequent syncs")
		consistentlyAnnotationAndRV(ctx, pod.Namespace, pod.Name, tc.expectedAnnotation, initialResourceVersion)
	},
		g.Entry("when all nodes are UP and the API returns them in the same order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			liveEndpoints:      []string{"10.0.0.1", "10.0.0.2"},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"UP"}]}` + "\n",
		}),
		g.Entry("when all nodes are UP and the API returns them in a different order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.2", Value: "host-id-2"},
				{Key: "10.0.0.1", Value: "host-id-1"},
			},
			liveEndpoints:      []string{"10.0.0.1", "10.0.0.2"},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"UP"}]}` + "\n",
		}),
		g.Entry("when all nodes are DOWN and the API returns them in the same order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			liveEndpoints:      []string{},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"DOWN"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
		g.Entry("when all nodes are DOWN and the API returns them in a different order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.2", Value: "host-id-2"},
				{Key: "10.0.0.1", Value: "host-id-1"},
			},
			liveEndpoints:      []string{},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"DOWN"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
		g.Entry("when nodes have mixed statuses and the API returns them in the same order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			liveEndpoints:      []string{"10.0.0.1"},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
		g.Entry("when nodes have mixed statuses and the API returns them in a different order on subsequent calls", statusStabilityTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.2", Value: "host-id-2"},
				{Key: "10.0.0.1", Value: "host-id-1"},
			},
			liveEndpoints:      []string{"10.0.0.1"},
			expectedAnnotation: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
	)

	type statusChangeTestCase struct {
		// firstCallHostIDs is the list returned by the fake ScyllaDB API on the first sync.
		firstCallHostIDs []scyllaNodeResponse
		// firstCallLiveEndpoints is the set of live endpoints on the first sync.
		firstCallLiveEndpoints []string
		// subsequentCallHostIDs is the list returned on all subsequent syncs.
		subsequentCallHostIDs []scyllaNodeResponse
		// subsequentCallLiveEndpoints is the set of live endpoints on all subsequent syncs.
		subsequentCallLiveEndpoints []string
		// expectedAnnotationAfterFirstCall is the expected annotation value after the first sync.
		expectedAnnotationAfterFirstCall string
		// expectedAnnotationAfterStatusChange is the expected annotation value after the status change is observed.
		expectedAnnotationAfterStatusChange string
	}

	g.DescribeTable("updates the Pod annotation", func(ctx g.SpecContext, tc statusChangeTestCase) {
		scyllaClientFactory := NewSwitchableScyllaClientFactory(
			tc.firstCallHostIDs, tc.firstCallLiveEndpoints,
			tc.subsequentCallHostIDs, tc.subsequentCallLiveEndpoints,
		)
		g.DeferCleanup(scyllaClientFactory.Close)

		g.By("Creating the Pod")
		pod := newBasicPod(env.Namespace())
		var err error
		pod, err = env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Starting status report controller")
		controller := runStatusReportController(ctx, env, pod.Name, scyllaClientFactory.NewClient)

		g.By("Waiting for the Pod to get the initial node status report annotation")
		waitForPodToHaveNodeStatusReportAnnotation(ctx, pod.Namespace, pod.Name, tc.expectedAnnotationAfterFirstCall)

		// Switch to phase two and explicitly enqueue a sync so the controller picks up the status change.
		g.By("Switching to phase two and triggering a sync")
		scyllaClientFactory.SwitchToPhaseTwo()
		controller.Enqueue()

		g.By("Waiting for the Pod annotation to reflect the status change")
		pod = waitForPodToHaveNodeStatusReportAnnotation(ctx, pod.Namespace, pod.Name, tc.expectedAnnotationAfterStatusChange)
	},
		g.Entry("when a node transitions from UP to DOWN and the API returns nodes in the same order on subsequent calls", statusChangeTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			firstCallLiveEndpoints: []string{"10.0.0.1", "10.0.0.2"},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			subsequentCallLiveEndpoints:         []string{"10.0.0.1"},
			expectedAnnotationAfterFirstCall:    `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"UP"}]}` + "\n",
			expectedAnnotationAfterStatusChange: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
		g.Entry("when a node transitions from UP to DOWN and the API returns nodes in a different order on subsequent calls", statusChangeTestCase{
			firstCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.1", Value: "host-id-1"},
				{Key: "10.0.0.2", Value: "host-id-2"},
			},
			firstCallLiveEndpoints: []string{"10.0.0.1", "10.0.0.2"},
			subsequentCallHostIDs: []scyllaNodeResponse{
				{Key: "10.0.0.2", Value: "host-id-2"},
				{Key: "10.0.0.1", Value: "host-id-1"},
			},
			subsequentCallLiveEndpoints:         []string{"10.0.0.1"},
			expectedAnnotationAfterFirstCall:    `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"UP"}]}` + "\n",
			expectedAnnotationAfterStatusChange: `{"observedNodes":[{"hostID":"host-id-1","status":"UP"},{"hostID":"host-id-2","status":"DOWN"}]}` + "\n",
		}),
	)
})

// SwitchableScyllaClientFactory is a two-phase Scylla client factory with the below semantics:
//   - Can be in Phase 1 or Phase 2; on initialization is in Phase 1.
//   - In Phase 1, NewClient serves firstHostIDs and firstLiveEndpoints.
//   - In Phase 2, NewClient serves subsequentHostIDs and subsequentLiveEndpoints.
//   - Upon call to SwitchToPhaseTwo, moves from Phase 1 to 2.
//   - On the first NewClient call in Phase 2, closes the channel returned by PhaseTwoCalledCh.
type SwitchableScyllaClientFactory struct {
	firstHostIDs            []scyllaNodeResponse
	firstLiveEndpoints      []string
	subsequentHostIDs       []scyllaNodeResponse
	subsequentLiveEndpoints []string

	inPhaseTwo       atomic.Bool
	phaseTwoCalledCh chan struct{}
	phaseTwoOnce     sync.Once

	mu        sync.Mutex
	teardowns []func()
}

func NewSwitchableScyllaClientFactory(
	firstHostIDs []scyllaNodeResponse, firstLiveEndpoints []string,
	subsequentHostIDs []scyllaNodeResponse, subsequentLiveEndpoints []string,
) *SwitchableScyllaClientFactory {
	return &SwitchableScyllaClientFactory{
		firstHostIDs:            firstHostIDs,
		firstLiveEndpoints:      firstLiveEndpoints,
		subsequentHostIDs:       subsequentHostIDs,
		subsequentLiveEndpoints: subsequentLiveEndpoints,
		phaseTwoCalledCh:        make(chan struct{}),
	}
}

// NewClient creates a new ScyllaDB client backed by a fake HTTP server.
// The response data depends on the current phase.
func (f *SwitchableScyllaClientFactory) NewClient() (*scyllaclient.Client, error) {
	var client *scyllaclient.Client
	var teardown func()
	var err error

	if f.inPhaseTwo.Load() {
		f.phaseTwoOnce.Do(func() {
			close(f.phaseTwoCalledCh)
		})

		client, teardown, err = newStaticScyllaClient(f.subsequentHostIDs, f.subsequentLiveEndpoints)
	} else {
		client, teardown, err = newStaticScyllaClient(f.firstHostIDs, f.firstLiveEndpoints)
	}

	if teardown != nil {
		f.mu.Lock()
		f.teardowns = append(f.teardowns, teardown)
		f.mu.Unlock()
	}

	return client, err
}

// SwitchToPhaseTwo transitions the factory from Phase 1 to Phase 2.
func (f *SwitchableScyllaClientFactory) SwitchToPhaseTwo() {
	f.inPhaseTwo.Store(true)
}

// PhaseTwoCalledCh returns a channel that is closed on the first NewClient call in Phase 2.
func (f *SwitchableScyllaClientFactory) PhaseTwoCalledCh() <-chan struct{} {
	return f.phaseTwoCalledCh
}

// Close tears down all HTTP servers created by NewClient calls.
func (f *SwitchableScyllaClientFactory) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, td := range f.teardowns {
		td()
	}
}

// newStaticScyllaClient creates a ScyllaDB client backed by httptest.Server that serves the given host IDs and live endpoints.
// It returns the client, a teardown function that closes the server, and any error.
// The caller is responsible for invoking the teardown function at an appropriate point.
func newStaticScyllaClient(hostIDNodes []scyllaNodeResponse, liveEndpoints []string) (*scyllaclient.Client, func(), error) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/storage_service/host_id":
			if err := json.NewEncoder(w).Encode(hostIDNodes); err != nil {
				g.GinkgoWriter.Printf("failed to encode host_id response: %v\n", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		case "/gossiper/endpoint/live/":
			if err := json.NewEncoder(w).Encode(liveEndpoints); err != nil {
				g.GinkgoWriter.Printf("failed to encode live nodes response: %v\n", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	})

	server := httptest.NewServer(handler)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		server.Close()
		return nil, nil, fmt.Errorf("can't parse server URL: %w", err)
	}

	cfg := &scyllaclient.Config{
		Hosts:   []string{parsedURL.Hostname()},
		Port:    parsedURL.Port(),
		Scheme:  "http",
		Timeout: 5 * time.Second,
	}

	client, err := scyllaclient.NewClient(cfg)
	if err != nil {
		server.Close()
		return nil, nil, err
	}

	return client, server.Close, nil
}

func runStatusReportController(ctx context.Context, env *envtest.Environment, podName string, newScyllaClient func() (*scyllaclient.Client, error)) *statusreport.Controller {
	g.GinkgoHelper()

	kubeInformers := kubeinformers.NewSharedInformerFactoryWithOptions(env.TypedKubeClient(), 0, kubeinformers.WithNamespace(env.Namespace()))

	c, err := statusreport.NewController(
		env.Namespace(),
		podName,
		env.TypedKubeClient(),
		kubeInformers.Core().V1().Pods(),
		newScyllaClient,
	)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create status report controller")

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
