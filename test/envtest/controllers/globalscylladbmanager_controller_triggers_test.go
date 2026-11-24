//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/globalscylladbmanager"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
)

const (
	globalScyllaDBManagerControllerResyncPeriod = 12 * time.Hour

	// globalScyllaDBManagerObserverKey is the only key the controller reconciles: the name of its observer.
	globalScyllaDBManagerObserverKey = "globalscylladbmanager-controller"
)

// runGlobalScyllaDBManagerController runs the global ScyllaDB Manager controller against the envtest API server.
func runGlobalScyllaDBManagerController(ctx context.Context, e *envtest.Environment, options ...globalscylladbmanager.ControllerOption) {
	g.GinkgoHelper()

	kubeInformers := informers.NewSharedInformerFactory(e.TypedKubeClient(), globalScyllaDBManagerControllerResyncPeriod)
	scyllaInformers := scyllainformers.NewSharedInformerFactory(e.ScyllaClient(), globalScyllaDBManagerControllerResyncPeriod)

	gsmc, err := globalscylladbmanager.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		kubeInformers.Core().V1().Namespaces(),
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	g.DeferCleanup(func() {
		cancel()
		wg.Wait()
		kubeInformers.Shutdown()
		scyllaInformers.Shutdown()
	})

	kubeInformers.Start(ctx.Done())
	scyllaInformers.Start(ctx.Done())
	wg.Go(func() {
		gsmc.Run(ctx)
	})
}

// globalScyllaDBManagerTriggerFixture is the state the trigger rows change: the global ScyllaDB Manager Namespace and
// a ScyllaDBDatacenter registered with the global ScyllaDB Manager, for which the controller created a
// ScyllaDBManagerClusterRegistration.
type globalScyllaDBManagerTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	datacenter   *scyllav1alpha1.ScyllaDBDatacenter
	registration *scyllav1alpha1.ScyllaDBManagerClusterRegistration
	namespace    *corev1.Namespace
}

// These rows check which changes reconcile the global ScyllaDB Manager controller, whose every reconciliation covers
// all ScyllaDBDatacenters. The rows run in order against one fixture, and a failing row doesn't stop the rest.
var _ = g.Describe("Global ScyllaDB Manager controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		otherDatacenterName = "envtest-other-sdc"
	)

	var f *globalScyllaDBManagerTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running global ScyllaDB Manager controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runGlobalScyllaDBManagerController(controllerCtx, env, globalscylladbmanager.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating the global ScyllaDB Manager Namespace")
		namespace, err := env.TypedKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: naming.ScyllaManagerNamespace,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter registered with the global ScyllaDB Manager")
		datacenter := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Labels = map[string]string{
				naming.GlobalScyllaDBManagerRegistrationLabel: naming.LabelValueTrue,
			}
		})
		datacenter, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, datacenter, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the controller to create the ScyllaDBManagerClusterRegistration")
		var registration *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			registrations, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(env.Namespace()).List(ctx, metav1.ListOptions{
				LabelSelector: naming.GlobalScyllaDBManagerClusterRegistrationSelector().String(),
			})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(registrations.Items).To(o.HaveLen(1))
			registration = &registrations.Items[0]
		}).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		f = &globalScyllaDBManagerTriggerFixture{
			triggerActions: triggerActions{client: env.KubeClient()},
			env:            env,
			recorder:       recorder,
			datacenter:     datacenter,
			registration:   registration,
			namespace:      namespace,
		}
	})

	g.DescribeTable("reconciles on the changes that concern the global ScyllaDB Manager",
		func(ctx g.SpecContext, row triggerRow[*globalScyllaDBManagerTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		// Every ScyllaDBDatacenter event reconciles, whether the datacenter is registered or not.
		g.Entry("registered ScyllaDBDatacenter update", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.annotate(ctx, f.datacenter.DeepCopy())
			},
			expected: []string{globalScyllaDBManagerObserverKey},
		}),
		g.Entry("unregistered ScyllaDBDatacenter creation", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.create(ctx, makeEnvtestScyllaDBDatacenter(f.env.Namespace(), []string{"rack-a"}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
					sdc.Name = otherDatacenterName
					sdc.Spec.ClusterName = "envtest-other-cluster"
				}))
			},
			expected: []string{globalScyllaDBManagerObserverKey},
		}),
		g.Entry("unregistered ScyllaDBDatacenter deletion", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.delete(ctx, &scyllav1alpha1.ScyllaDBDatacenter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      otherDatacenterName,
						Namespace: f.env.Namespace(),
					},
				})
			},
			expected: []string{globalScyllaDBManagerObserverKey},
		}),

		// The ScyllaDBManagerClusterRegistration and Namespace watches filter the objects by hand. The API admits only
		// ScyllaDBManagerClusterRegistrations of the global ScyllaDB Manager, so no row can exercise the former filter.
		g.Entry("global ScyllaDB Manager's ScyllaDBManagerClusterRegistration update", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.annotate(ctx, f.registration.DeepCopy())
			},
			expected: []string{globalScyllaDBManagerObserverKey},
		}),
		g.Entry("global ScyllaDB Manager Namespace update", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.annotate(ctx, f.namespace.DeepCopy())
			},
			expected: []string{globalScyllaDBManagerObserverKey},
		}),
		g.Entry("other Namespace creation", triggerRow[*globalScyllaDBManagerTriggerFixture]{
			change: func(ctx context.Context, f *globalScyllaDBManagerTriggerFixture) {
				f.create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "envtest-other-",
					},
				})
			},
		}),
	)
})
