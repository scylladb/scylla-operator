//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// triggerQuietWindow is how long the controller has to go without a reconciliation for the fixture to count as
	// settled, and how long a change that must not trigger one is watched. The resync period is hours away, so
	// once the controller stops reacting to its own writes nothing but the row's change can wake it.
	triggerQuietWindow = 2 * time.Second

	// triggerAnnotation is an annotation no controller manages, so setting it changes an object without changing
	// what the controller wants it to be.
	triggerAnnotation = "internal.scylla-operator.scylladb.com/envtest-trigger"

	// triggerPodOrdinal is the ordinal of the member Pod the Pod rows create, update and delete. The fixture holds
	// the rack's first Pod for the cleanup Job.
	triggerPodOrdinal = 1
)

// reconcileRecorder records the ScyllaDBDatacenters the controller reconciles.
type reconcileRecorder struct {
	mu         sync.Mutex
	reconciled []types.NamespacedName
	last       time.Time
}

func (r *reconcileRecorder) observe(key types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reconciled = append(r.reconciled, key)
	r.last = time.Now()
}

// datacenters returns the names of the ScyllaDBDatacenters reconciled since the last reset, each once.
func (r *reconcileRecorder) datacenters() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var names []string
	for _, key := range r.reconciled {
		if !slices.Contains(names, key.Name) {
			names = append(names, key.Name)
		}
	}

	return names
}

func (r *reconcileRecorder) sinceLast() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	return time.Since(r.last)
}

// waitUntilIdle waits until the controller has run no reconciliation for the quiet window, then forgets the
// reconciliations so far.
func (r *reconcileRecorder) waitUntilIdle(ctx context.Context) {
	g.GinkgoHelper()

	o.Eventually(r.sinceLast).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.BeNumerically(">=", triggerQuietWindow))

	r.mu.Lock()
	defer r.mu.Unlock()
	r.reconciled = nil
}

// triggerFixture is the state the trigger rows change: a rolled-out datacenter with one of every kind the
// controller owns, a bystander datacenter in the same namespace that no row but its own may enqueue, and objects
// that resemble the datacenter's but aren't its.
type triggerFixture struct {
	env      *envtest.Environment
	recorder *reconcileRecorder

	// datacenter refers to overrideSecret as its ScyllaDB Manager agent auth token override, exposes CQL through
	// an Ingress, and has a cleanup Job pending for its first node.
	datacenter     *scyllav1alpha1.ScyllaDBDatacenter
	overrideSecret *corev1.Secret
	bystander      *scyllav1alpha1.ScyllaDBDatacenter
	// foreignStatefulSet is controlled by no ScyllaDBDatacenter.
	foreignStatefulSet *appsv1.StatefulSet
}

// rackStatefulSet returns the datacenter's rack StatefulSet.
func (f *triggerFixture) rackStatefulSet(ctx context.Context) *appsv1.StatefulSet {
	g.GinkgoHelper()

	return waitForStatefulSet(ctx, f.env, naming.StatefulSetNameForRack(f.datacenter.Spec.Racks[0], f.datacenter), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
}

// ownedBy returns an object of list's kind controlled by sdc.
func (f *triggerFixture) ownedBy(ctx context.Context, list client.ObjectList, sdc *scyllav1alpha1.ScyllaDBDatacenter) client.Object {
	g.GinkgoHelper()

	err := f.env.KubeClient().List(ctx, list, client.InNamespace(f.env.Namespace()))
	o.Expect(err).NotTo(o.HaveOccurred())

	items, err := apimeta.ExtractList(list)
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, item := range items {
		obj := item.(client.Object)
		if metav1.IsControlledBy(obj, sdc) {
			return obj
		}
	}

	g.Fail(fmt.Sprintf("no object of %T is controlled by %q", list, sdc.Name))
	return nil
}

// memberPod returns the member Pod of the Pod rows, created by the member Pod creation row.
func (f *triggerFixture) memberPod(ctx context.Context) *corev1.Pod {
	g.GinkgoHelper()

	pod, err := f.env.TypedKubeClient().CoreV1().Pods(f.env.Namespace()).Get(ctx, naming.MemberServiceName(f.datacenter.Spec.Racks[0], f.datacenter, triggerPodOrdinal), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return pod
}

// annotate sets the trigger annotation on obj to a fresh value.
func (f *triggerFixture) annotate(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[triggerAnnotation] = apimachineryutilrand.String(8)
	obj.SetAnnotations(annotations)

	err := f.env.KubeClient().Patch(ctx, obj, patch)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (f *triggerFixture) create(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	err := f.env.KubeClient().Create(ctx, obj)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (f *triggerFixture) delete(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	err := f.env.KubeClient().Delete(ctx, obj)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// triggeredDatacenter names a fixture datacenter a row expects to be reconciled.
type triggeredDatacenter int

const (
	theDatacenter triggeredDatacenter = iota
	theBystander
)

type triggerRow struct {
	// change is what the row does to the cluster.
	change func(ctx context.Context, f *triggerFixture)
	// expected are the datacenters the change must have reconciled, and nothing else; none means the change must
	// not reconcile anything.
	expected []triggeredDatacenter
}

// These rows check which ScyllaDBDatacenters each kind of change reconciles. The rows run in order against one
// fixture, and a failing row doesn't stop the rest, so every broken watch is reported at once. The Pod rows create,
// update and delete the same Pod.
var _ = g.Describe("ScyllaDBDatacenter controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const rackName = "rack-a"

	var f *triggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaDBDatacenter controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaDBDatacenterControllerWithOptions(controllerCtx, env, scyllaDBDatacenterControllerRunOptions{
			controllerOptions: []scylladbdatacenter.ControllerOption{
				scylladbdatacenter.WithReconcileObserver(recorder.observe),
			},
		})
		g.DeferCleanup(cancel)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating an agent auth token override Secret")
		overrideTokenConfig, err := helpers.GetAgentAuthTokenConfig("envtest-override-token")
		o.Expect(err).NotTo(o.HaveOccurred())
		overrideSecret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-auth-token-override",
				Namespace: env.Namespace(),
			},
			Data: map[string][]byte{
				naming.ScyllaAgentAuthTokenFileName: overrideTokenConfig,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter referring to the override Secret and exposing CQL through an Ingress, and a bystander ScyllaDBDatacenter")
		datacenter := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Annotations = map[string]string{
				naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation: overrideSecret.Name,
			}
			sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
				CQL: &scyllav1alpha1.CQLExposeOptions{
					Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
						IngressClassName: "envtest",
					},
				},
			}
		})
		datacenter, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, datacenter, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		bystander := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Name = "envtest-bystander"
			sdc.Spec.ClusterName = "envtest-bystander-cluster"
		})
		bystander, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, bystander, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSets and marking them as rolled out")
		for _, sdc := range []*scyllav1alpha1.ScyllaDBDatacenter{datacenter, bystander} {
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			waitForService(ctx, env, naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)
		}

		// The cleanup Job of a node is due while the token ring hash its sidecar reports differs from the last one
		// cleaned up, and needs the node's Pod. The Job never completes here: nothing runs it.
		g.By("Creating the first member Pod and reporting a token ring hash for it in place of the sidecar, so that its cleanup Job is created")
		rackStatefulSet := waitForStatefulSet(ctx, env, naming.StatefulSetNameForRack(datacenter.Spec.Racks[0], datacenter), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		createMemberPod(ctx, env, rackStatefulSet, 0, datacenter.Spec.ScyllaDB.Image)
		memberServiceName := naming.MemberServiceName(datacenter.Spec.Racks[0], datacenter, 0)
		setServiceAnnotations(ctx, env, memberServiceName, map[string]string{
			naming.HostIDAnnotation:               rackName + "-0",
			naming.CurrentTokenRingHashAnnotation: envtestTokenRingHash,
		})

		g.By("Waiting for the cleanup Job and the Ingresses to be created")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().BatchV1().Jobs(env.Namespace()).Get(ctx, naming.CleanupJobForService(memberServiceName), metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())

			ingresses, err := env.TypedKubeClient().NetworkingV1().Ingresses(env.Namespace()).List(ctx, metav1.ListOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(ingresses.Items).NotTo(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Creating a StatefulSet controlled by no ScyllaDBDatacenter")
		foreignStatefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Create(ctx, makeEnvtestForeignStatefulSet(env.Namespace(), "foreign", nil), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		f = &triggerFixture{
			env:                env,
			recorder:           recorder,
			datacenter:         datacenter,
			overrideSecret:     overrideSecret,
			bystander:          bystander,
			foreignStatefulSet: foreignStatefulSet,
		}
	})

	g.DescribeTable("reconciles the ScyllaDBDatacenters a change concerns",
		func(ctx g.SpecContext, row triggerRow) {
			var expected []string
			for _, d := range row.expected {
				switch d {
				case theDatacenter:
					expected = append(expected, f.datacenter.Name)
				case theBystander:
					expected = append(expected, f.bystander.Name)
				}
			}

			g.By("Waiting for the controller to go idle")
			f.recorder.waitUntilIdle(ctx)

			g.By("Applying the change")
			row.change(ctx, f)

			if len(expected) == 0 {
				g.By("Verifying no ScyllaDBDatacenter is reconciled")
				o.Consistently(f.recorder.datacenters).WithContext(ctx).WithTimeout(triggerQuietWindow).WithPolling(100 * time.Millisecond).Should(o.BeEmpty())
				return
			}

			g.By("Waiting for the expected ScyllaDBDatacenters to be reconciled, and no other")
			o.Eventually(f.recorder.datacenters).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.ConsistOf(expected))
			o.Consistently(f.recorder.datacenters).WithContext(ctx).WithTimeout(triggerQuietWindow).WithPolling(100 * time.Millisecond).Should(o.ConsistOf(expected))
		},

		// The rows below cover the watches declared in SetupWithManager with For and Owns: the datacenter itself and
		// the objects it controls. Owns feeds every event of an object through the same handler, so one change per
		// kind is enough.
		g.Entry("ScyllaDBDatacenter update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.datacenter.DeepCopy())
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("bystander ScyllaDBDatacenter update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.bystander.DeepCopy())
			},
			expected: []triggeredDatacenter{theBystander},
		}),

		g.Entry("owned Service update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &corev1.ServiceList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned ConfigMap update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &corev1.ConfigMapList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned Secret update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &corev1.SecretList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned ServiceAccount update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &corev1.ServiceAccountList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned RoleBinding update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &rbacv1.RoleBindingList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned StatefulSet update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &appsv1.StatefulSetList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned PodDisruptionBudget update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &policyv1.PodDisruptionBudgetList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned ScyllaDBDatacenterNodesStatusReport update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReportList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned Ingress update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &networkingv1.IngressList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("owned Job update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.ownedBy(ctx, &batchv1.JobList{}, f.datacenter))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),

		// The rows below exercise the map functions, the only enqueue logic written by hand: the Secret watch
		// reaching the datacenters that name the Secret in their annotation, the Pod watch reaching the datacenter
		// through the Pod's StatefulSet, and the ScyllaOperatorConfig watch reaching every datacenter.
		g.Entry("override Secret update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.overrideSecret.DeepCopy())
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("unrelated Secret creation", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated",
						Namespace: f.env.Namespace(),
					},
				})
			},
		}),
		// An orphan carries the datacenter's labels but no controllerRef: it is adopted by the next sync that runs
		// for another reason, not by one of its own.
		g.Entry("orphaned ConfigMap creation", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphan",
						Namespace: f.env.Namespace(),
						Labels:    naming.ScyllaDBDatacenterSelectorLabels(f.datacenter),
					},
				})
			},
		}),

		g.Entry("member Pod creation", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				createMemberPod(ctx, f.env, f.rackStatefulSet(ctx), triggerPodOrdinal, f.datacenter.Spec.ScyllaDB.Image)
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("member Pod update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, f.memberPod(ctx))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("member Pod deletion", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.delete(ctx, f.memberPod(ctx))
			},
			expected: []triggeredDatacenter{theDatacenter},
		}),
		g.Entry("Pod of a foreign StatefulSet creation", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.create(ctx, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foreign-0",
						Namespace: f.env.Namespace(),
						Labels:    f.foreignStatefulSet.Spec.Template.Labels,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(f.foreignStatefulSet, appsv1.SchemeGroupVersion.WithKind("StatefulSet")),
						},
					},
					Spec: f.foreignStatefulSet.Spec.Template.Spec,
				})
			},
		}),

		g.Entry("ScyllaOperatorConfig update", triggerRow{
			change: func(ctx context.Context, f *triggerFixture) {
				f.annotate(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: naming.SingletonName,
					},
				})
			},
			expected: []triggeredDatacenter{theDatacenter, theBystander},
		}),
	)
})
