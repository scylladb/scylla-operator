//go:build envtest

package controllers

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// orphanedPVTriggerFixture is the state the trigger rows change: two datacenters and a Node.
type orphanedPVTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	datacenter      *scyllav1alpha1.ScyllaDBDatacenter
	otherDatacenter *scyllav1alpha1.ScyllaDBDatacenter
	node            *corev1.Node
}

// These rows check which ScyllaDBDatacenters each kind of change reconciles. The rows run in order against one
// fixture, and a failing row doesn't stop the rest. The ScyllaDBDatacenter creation row adds a third datacenter
// that the rows after it see.
var _ = g.Describe("OrphanedPVController triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		datacenterName      = "envtest-sdc"
		otherDatacenterName = "envtest-other-sdc"
		newDatacenterName   = "envtest-new-sdc"
	)

	var f *orphanedPVTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running OrphanedPVController with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runOrphanedPVController(controllerCtx, env, orphanedpv.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating two ScyllaDBDatacenters and a Node")
		datacenter, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, makeOrphanedPVTriggerDatacenter(env.Namespace(), datacenterName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		otherDatacenter, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, makeOrphanedPVTriggerDatacenter(env.Namespace(), otherDatacenterName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		node, err := env.TypedKubeClient().CoreV1().Nodes().Create(ctx, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "envtest-node",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// The recorder counts as idle before the first reconciliation, so the first row would otherwise see the
		// reconciliations of the datacenters' creation.
		g.By("Waiting for both ScyllaDBDatacenters to be reconciled")
		o.Eventually(recorder.names).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.ContainElements(datacenterName, otherDatacenterName))

		f = &orphanedPVTriggerFixture{
			triggerActions:  triggerActions{client: env.KubeClient()},
			env:             env,
			recorder:        recorder,
			datacenter:      datacenter,
			otherDatacenter: otherDatacenter,
			node:            node,
		}
	})

	g.DescribeTable("reconciles the ScyllaDBDatacenters a change concerns",
		func(ctx g.SpecContext, row triggerRow[*orphanedPVTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		// The ScyllaDBDatacenter handlers enqueue the datacenter of the event.
		g.Entry("ScyllaDBDatacenter update", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.annotate(ctx, f.datacenter.DeepCopy())
			},
			expected: []string{datacenterName},
		}),
		g.Entry("other ScyllaDBDatacenter update", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.annotate(ctx, f.otherDatacenter.DeepCopy())
			},
			expected: []string{otherDatacenterName},
		}),
		g.Entry("ScyllaDBDatacenter creation", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.create(ctx, makeOrphanedPVTriggerDatacenter(f.env.Namespace(), newDatacenterName))
			},
			expected: []string{newDatacenterName},
		}),

		// A Node deletion enqueues every datacenter. A Node update only counts when it replaces the Node with one of
		// the same name, and a Node creation not at all.
		g.Entry("Node creation", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "envtest-other-node",
					},
				})
			},
		}),
		g.Entry("Node update", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.annotate(ctx, f.node.DeepCopy())
			},
		}),
		g.Entry("Node deletion", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.delete(ctx, f.node.DeepCopy())
			},
			expected: []string{datacenterName, otherDatacenterName, newDatacenterName},
		}),

		// The controller lists PersistentVolumes and PersistentVolumeClaims but doesn't watch them.
		g.Entry("PersistentVolume creation", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.create(ctx, &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "envtest-pv",
					},
					Spec: corev1.PersistentVolumeSpec{
						Capacity: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						PersistentVolumeSource: corev1.PersistentVolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/tmp/envtest",
							},
						},
					},
				})
			},
		}),
		g.Entry("PersistentVolumeClaim creation", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.create(ctx, &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "envtest-pvc",
						Namespace: f.env.Namespace(),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				})
			},
		}),

		g.Entry("ScyllaDBDatacenter deletion", triggerRow[*orphanedPVTriggerFixture]{
			change: func(ctx context.Context, f *orphanedPVTriggerFixture) {
				f.delete(ctx, f.otherDatacenter.DeepCopy())
			},
			expected: []string{otherDatacenterName},
		}),
	)
})

// makeOrphanedPVTriggerDatacenter returns a ScyllaDBDatacenter with automatic orphaned node replacement left
// disabled, so that its reconciliation ends without requeuing for the PersistentVolumeClaims nothing creates here.
func makeOrphanedPVTriggerDatacenter(namespace, name string) *scyllav1alpha1.ScyllaDBDatacenter {
	return makeEnvtestScyllaDBDatacenter(namespace, []string{"rack-a"}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Name = name
	})
}
