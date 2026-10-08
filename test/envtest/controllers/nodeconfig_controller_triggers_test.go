//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// nodeConfigTriggerFixture is the state the trigger rows change: two NodeConfigs and the objects the controller
// created for them, those owned by one NodeConfig and those shared by all of them.
type nodeConfigTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	nodeConfig      *scyllav1alpha1.NodeConfig
	otherNodeConfig *scyllav1alpha1.NodeConfig
}

// isSharedNodeConfigObject tells whether obj carries the label of the objects shared by all NodeConfigs.
func isSharedNodeConfigObject(obj client.Object) bool {
	return obj.GetLabels()[naming.NodeConfigNameLabel] == naming.NodeConfigAppName
}

// findObject returns the first object of list's kind in namespace, or cluster-wide if namespace is empty, for which
// match returns true, or nil.
func (f *nodeConfigTriggerFixture) findObject(ctx context.Context, list client.ObjectList, namespace string, match func(client.Object) bool) (client.Object, error) {
	var opts []client.ListOption
	if len(namespace) != 0 {
		opts = append(opts, client.InNamespace(namespace))
	}
	err := f.env.KubeClient().List(ctx, list, opts...)
	if err != nil {
		return nil, err
	}

	items, err := apimeta.ExtractList(list)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		obj := item.(client.Object)
		if match(obj) {
			return obj, nil
		}
	}

	return nil, nil
}

// getAnyOwnedBy returns any object of list's kind in the node tuning namespace controlled by nc.
func (f *nodeConfigTriggerFixture) getAnyOwnedBy(ctx context.Context, list client.ObjectList, nc *scyllav1alpha1.NodeConfig) client.Object {
	g.GinkgoHelper()

	obj, err := f.findObject(ctx, list, naming.ScyllaOperatorNodeTuningNamespace, func(obj client.Object) bool {
		return metav1.IsControlledBy(obj, nc)
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(obj).NotTo(o.BeNil(), fmt.Sprintf("no object of %T is controlled by %q", list, nc.Name))

	return obj
}

// getAnyManaged returns any object of list's kind carrying the label of the objects shared by all NodeConfigs.
func (f *nodeConfigTriggerFixture) getAnyManaged(ctx context.Context, list client.ObjectList) client.Object {
	g.GinkgoHelper()

	obj, err := f.findObject(ctx, list, "", isSharedNodeConfigObject)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(obj).NotTo(o.BeNil(), fmt.Sprintf("no object of %T is managed by the NodeConfig controller", list))

	return obj
}

// These rows check which NodeConfigs each kind of change reconciles. The rows run in order against one fixture,
// and a failing row doesn't stop the rest, so every broken watch is reported at once.
var _ = g.Describe("NodeConfig controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		nodeConfigName      = "envtest-nc"
		otherNodeConfigName = "envtest-other-nc"
	)

	var f *nodeConfigTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running NodeConfig controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runNodeConfigController(controllerCtx, env, nodeconfig.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating ScyllaOperatorConfig singleton with the ScyllaDB utils image in its status")
		soc := createScyllaOperatorConfig(ctx, env)
		soc.Status.ScyllaDBUtilsImage = pointer.Ptr(soc.Spec.ScyllaUtilsImage)
		_, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating two NodeConfigs")
		var nodeConfigs []*scyllav1alpha1.NodeConfig
		for _, name := range []string{nodeConfigName, otherNodeConfigName} {
			nc, err := env.ScyllaClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, &scyllav1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: scyllav1alpha1.NodeConfigSpec{
					Placement: scyllav1alpha1.NodeConfigPlacement{
						NodeSelector: map[string]string{
							"envtest-node-config": name,
						},
					},
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeConfigs = append(nodeConfigs, nc)
		}

		f = &nodeConfigTriggerFixture{
			triggerActions:  triggerActions{client: env.KubeClient()},
			env:             env,
			recorder:        recorder,
			nodeConfig:      nodeConfigs[0],
			otherNodeConfig: nodeConfigs[1],
		}

		g.By("Waiting for the controller to create the shared objects and those of both NodeConfigs")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, list := range []client.ObjectList{
				&corev1.NamespaceList{},
				&corev1.ServiceAccountList{},
				&rbacv1.ClusterRoleList{},
				&rbacv1.ClusterRoleBindingList{},
				&rbacv1.RoleList{},
				&rbacv1.RoleBindingList{},
			} {
				obj, err := f.findObject(ctx, list, "", isSharedNodeConfigObject)
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(obj).NotTo(o.BeNil(), "no managed object of %T", list)
			}

			for _, nc := range nodeConfigs {
				for _, list := range []client.ObjectList{&appsv1.DaemonSetList{}, &corev1.ConfigMapList{}} {
					obj, err := f.findObject(ctx, list, naming.ScyllaOperatorNodeTuningNamespace, func(obj client.Object) bool {
						return metav1.IsControlledBy(obj, nc)
					})
					eo.Expect(err).NotTo(o.HaveOccurred())
					eo.Expect(obj).NotTo(o.BeNil(), "no object of %T controlled by %q", list, nc.Name)
				}
			}
		}).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.DescribeTable("reconciles the NodeConfigs a change concerns",
		func(ctx g.SpecContext, row triggerRow[*nodeConfigTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		g.Entry("NodeConfig update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.nodeConfig.DeepCopy())
			},
			expected: []string{nodeConfigName},
		}),
		g.Entry("other NodeConfig update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.otherNodeConfig.DeepCopy())
			},
			expected: []string{otherNodeConfigName},
		}),

		// The rows below cover the objects a NodeConfig controls: the handlers enqueue their owner.
		g.Entry("owned DaemonSet update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &appsv1.DaemonSetList{}, f.nodeConfig))
			},
			expected: []string{nodeConfigName},
		}),
		g.Entry("owned ConfigMap update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &corev1.ConfigMapList{}, f.nodeConfig))
			},
			expected: []string{nodeConfigName},
		}),
		g.Entry("unowned ConfigMap creation", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unowned",
						Namespace: naming.ScyllaOperatorNodeTuningNamespace,
					},
				})
			},
		}),

		// The rows below cover the objects shared by all NodeConfigs: the handlers enqueue every NodeConfig for an
		// object carrying the controller's label, and none for any other.
		g.Entry("managed Namespace update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &corev1.NamespaceList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("managed ServiceAccount update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &corev1.ServiceAccountList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("managed ClusterRole update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &rbacv1.ClusterRoleList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("managed ClusterRoleBinding update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &rbacv1.ClusterRoleBindingList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("managed Role update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &rbacv1.RoleList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("managed RoleBinding update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, f.getAnyManaged(ctx, &rbacv1.RoleBindingList{}))
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("unmanaged ClusterRole creation", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.create(ctx, &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "envtest-unmanaged",
					},
				})
			},
		}),

		// The controller has a Node handler, but doesn't register it: a Node change reconciles nothing.
		g.Entry("Node creation", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "envtest-node",
						Labels: map[string]string{
							"envtest-node-config": nodeConfigName,
						},
					},
				})
			},
		}),

		// The ScyllaOperatorConfig handlers enqueue every NodeConfig.
		g.Entry("ScyllaOperatorConfig update", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.annotate(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: naming.SingletonName,
					},
				})
			},
			expected: []string{nodeConfigName, otherNodeConfigName},
		}),
		g.Entry("other ScyllaOperatorConfig creation", triggerRow[*nodeConfigTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigTriggerFixture) {
				f.create(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other",
					},
				})
			},
		}),
	)
})
