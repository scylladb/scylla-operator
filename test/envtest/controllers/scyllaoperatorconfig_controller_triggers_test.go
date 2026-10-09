//go:build envtest

package controllers

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// scyllaOperatorConfigTriggerFixture is the state the trigger rows change: the singleton the controller created.
type scyllaOperatorConfigTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder
}

// singleton returns the ScyllaOperatorConfig singleton.
func (f *scyllaOperatorConfigTriggerFixture) singleton(ctx context.Context) *scyllav1alpha1.ScyllaOperatorConfig {
	g.GinkgoHelper()

	soc, err := f.env.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Get(ctx, naming.SingletonName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return soc
}

// These rows check which changes reconcile the ScyllaOperatorConfig singleton. The rows run in order against one
// fixture, and a failing row doesn't stop the rest. The deletion row comes last but one, so that the rows before it
// change the singleton the controller created at start.
var _ = g.Describe("ScyllaOperatorConfig controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	var f *scyllaOperatorConfigTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaOperatorConfig controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaOperatorConfigController(
			controllerCtx,
			env,
			func(ctx context.Context) (string, error) {
				return "cluster.local", nil
			},
			scyllaoperatorconfig.WithOnReconcile(recorder.observe),
		)
		g.DeferCleanup(cancel)

		g.By("Waiting for the controller to create the singleton")
		waitForScyllaOperatorConfigSingleton(ctx, env)

		f = &scyllaOperatorConfigTriggerFixture{
			triggerActions: triggerActions{client: env.KubeClient()},
			env:            env,
			recorder:       recorder,
		}
	})

	g.DescribeTable("reconciles the ScyllaOperatorConfig singleton on the changes that concern it",
		func(ctx g.SpecContext, row triggerRow[*scyllaOperatorConfigTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		g.Entry("singleton spec update", triggerRow[*scyllaOperatorConfigTriggerFixture]{
			change: func(ctx context.Context, f *scyllaOperatorConfigTriggerFixture) {
				soc := f.singleton(ctx)
				patch := client.MergeFrom(soc.DeepCopy())
				soc.Spec.ScyllaDBNodeExporterImage = pointer.Ptr("docker.io/scylladb/node-exporter:envtest")
				err := f.env.KubeClient().Patch(ctx, soc, patch)
				o.Expect(err).NotTo(o.HaveOccurred())
			},
			expected: []string{naming.SingletonName},
		}),
		// The controller reverts the change, as the status is its own.
		g.Entry("singleton status update", triggerRow[*scyllaOperatorConfigTriggerFixture]{
			change: func(ctx context.Context, f *scyllaOperatorConfigTriggerFixture) {
				soc := f.singleton(ctx)
				patch := client.MergeFrom(soc.DeepCopy())
				soc.Status.GrafanaImage = pointer.Ptr("docker.io/grafana/grafana:envtest")
				err := f.env.KubeClient().Status().Patch(ctx, soc, patch)
				o.Expect(err).NotTo(o.HaveOccurred())
			},
			expected: []string{naming.SingletonName},
		}),
		g.Entry("singleton metadata update", triggerRow[*scyllaOperatorConfigTriggerFixture]{
			change: func(ctx context.Context, f *scyllaOperatorConfigTriggerFixture) {
				f.annotate(ctx, f.singleton(ctx))
			},
			expected: []string{naming.SingletonName},
		}),
		// The controller recreates the singleton.
		g.Entry("singleton deletion", triggerRow[*scyllaOperatorConfigTriggerFixture]{
			change: func(ctx context.Context, f *scyllaOperatorConfigTriggerFixture) {
				f.delete(ctx, f.singleton(ctx))
			},
			expected: []string{naming.SingletonName},
		}),
		g.Entry("other ScyllaOperatorConfig creation", triggerRow[*scyllaOperatorConfigTriggerFixture]{
			change: func(ctx context.Context, f *scyllaOperatorConfigTriggerFixture) {
				f.create(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other",
					},
				})
			},
		}),
	)
})
