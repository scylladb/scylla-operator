//go:build envtest

package controllers

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilintstr "k8s.io/apimachinery/pkg/util/intstr"
)

// scyllaClusterTriggerFixture is the state the trigger rows change: a ScyllaCluster with the ScyllaDBDatacenter the
// controller created for it, and another ScyllaCluster in the same namespace that no row but its own may enqueue.
type scyllaClusterTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	cluster      *scyllav1.ScyllaCluster
	datacenter   *scyllav1alpha1.ScyllaDBDatacenter
	otherCluster *scyllav1.ScyllaCluster
}

// controlledObjectMeta returns the metadata of an object of the given name controlled by the fixture's cluster.
func (f *scyllaClusterTriggerFixture) controlledObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: f.env.Namespace(),
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(f.cluster, scyllav1.GroupVersion.WithKind("ScyllaCluster")),
		},
	}
}

// These rows check which ScyllaClusters each kind of change reconciles. The rows run in order against one fixture,
// and a failing row doesn't stop the rest, so every broken watch is reported at once.
var _ = g.Describe("ScyllaClusterController triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		clusterName      = "envtest-sc"
		otherClusterName = "envtest-other-sc"
	)

	var f *scyllaClusterTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaCluster controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaClusterController(controllerCtx, env, scyllacluster.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		cluster, datacenter := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, newBasicScyllaCluster(clusterName, env.Namespace()))
		otherCluster, _ := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, newBasicScyllaCluster(otherClusterName, env.Namespace()))

		f = &scyllaClusterTriggerFixture{
			triggerActions: triggerActions{client: env.KubeClient()},
			env:            env,
			recorder:       recorder,
			cluster:        cluster,
			datacenter:     datacenter,
			otherCluster:   otherCluster,
		}
	})

	g.DescribeTable("reconciles the ScyllaClusters a change concerns",
		func(ctx g.SpecContext, row triggerRow[*scyllaClusterTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		g.Entry("ScyllaCluster update", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.annotate(ctx, f.cluster.DeepCopy())
			},
			expected: []string{clusterName},
		}),
		g.Entry("other ScyllaCluster update", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.annotate(ctx, f.otherCluster.DeepCopy())
			},
			expected: []string{otherClusterName},
		}),

		// The rows below cover the handlers that enqueue the controller of the object. They feed every event of an
		// object through the same function, so one change per kind is enough. The objects carry no ScyllaCluster
		// labels, so the controller doesn't release them.
		g.Entry("controlled ScyllaDBDatacenter update", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.annotate(ctx, f.datacenter.DeepCopy())
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled Service creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &corev1.Service{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					Spec: corev1.ServiceSpec{
						ClusterIP: corev1.ClusterIPNone,
					},
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled Secret creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &corev1.Secret{
					ObjectMeta: f.controlledObjectMeta("trigger"),
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled ConfigMap creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: f.controlledObjectMeta("trigger"),
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled ServiceAccount creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &corev1.ServiceAccount{
					ObjectMeta: f.controlledObjectMeta("trigger"),
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled RoleBinding creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &rbacv1.RoleBinding{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "envtest",
					},
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled StatefulSet creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				sts := makeEnvtestForeignStatefulSet(f.env.Namespace(), "trigger", nil)
				sts.ObjectMeta = f.controlledObjectMeta(sts.Name)
				f.create(ctx, sts)
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled PodDisruptionBudget creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &policyv1.PodDisruptionBudget{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					Spec:       makeEnvtestPodDisruptionBudgetSpec(new(apimachineryutilintstr.FromInt32(1))),
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled Ingress creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &networkingv1.Ingress{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					Spec:       makeEnvtestIngressSpec("trigger.envtest.scylladb.local"),
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("controlled Job creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &batchv1.Job{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "trigger",
										Image: "trigger:envtest",
									},
								},
							},
						},
					},
				})
			},
			expected: []string{clusterName},
		}),
		// The controller prunes the task it doesn't want, which reconciles the cluster again.
		g.Entry("controlled ScyllaDBManagerTask creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &scyllav1alpha1.ScyllaDBManagerTask{
					ObjectMeta: f.controlledObjectMeta("trigger"),
					Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
						ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
							Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
							Name: f.datacenter.Name,
						},
						Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
						Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
					},
				})
			},
			expected: []string{clusterName},
		}),
		g.Entry("ConfigMap controlled by another controller creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "foreign",
						Namespace:       f.env.Namespace(),
						OwnerReferences: []metav1.OwnerReference{makeEnvtestForeignControllerRef()},
					},
				})
			},
		}),

		// The ScyllaDBManagerClusterRegistration handler enqueues the ScyllaClusters whose ScyllaDBDatacenter the
		// registration is named for.
		g.Entry("ScyllaDBManagerClusterRegistration of the datacenter creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				name, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(f.datacenter)
				o.Expect(err).NotTo(o.HaveOccurred())

				f.create(ctx, makeScyllaClusterTriggerClusterRegistration(f.env.Namespace(), name, f.datacenter.Name))
			},
			expected: []string{clusterName},
		}),
		g.Entry("unrelated ScyllaDBManagerClusterRegistration creation", triggerRow[*scyllaClusterTriggerFixture]{
			change: func(ctx context.Context, f *scyllaClusterTriggerFixture) {
				f.create(ctx, makeScyllaClusterTriggerClusterRegistration(f.env.Namespace(), "unrelated", "unrelated"))
			},
		}),
	)
})

func makeScyllaClusterTriggerClusterRegistration(namespace, name, datacenterName string) *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				naming.GlobalScyllaDBManagerLabel: naming.LabelValueTrue,
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: datacenterName,
			},
		},
	}
}
