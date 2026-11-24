//go:build envtest

package controllers

import (
	"context"
	"maps"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nodeConfigPodTriggerNodeLabel is the Node label the trigger fixture's NodeConfigs select Nodes by.
const nodeConfigPodTriggerNodeLabel = "envtest-node"

// makeNodeConfigPodTriggerPod returns a Pod scheduled to nodeName, or unscheduled if nodeName is empty, that the
// controller takes for a ScyllaDB node if scylla is true.
func makeNodeConfigPodTriggerPod(namespace, name, nodeName string, scylla bool) *corev1.Pod {
	var labels map[string]string
	if scylla {
		labels = maps.Clone(naming.ScyllaLabels())
		maps.Copy(labels, naming.ScyllaDBNodePodLabels())
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:  naming.ScyllaContainerName,
					Image: "docker.io/scylladb/scylla:envtest",
				},
			},
		},
	}
}

// createNodeConfigPodTriggerPod creates pod and reports its ScyllaDB container in its status in place of the
// kubelet, as the controller needs the container ID.
func createNodeConfigPodTriggerPod(ctx context.Context, e *envtest.Environment, pod *corev1.Pod) *corev1.Pod {
	g.GinkgoHelper()

	pod, err := e.TypedKubeClient().CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name:        naming.ScyllaContainerName,
			ContainerID: "containerd://" + pod.Name,
		},
	}
	pod, err = e.TypedKubeClient().CoreV1().Pods(pod.Namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return pod
}

// makeNodeConfigPodTriggerNodeConfig returns a NodeConfig selecting the Nodes labelled with nodeLabelValue.
func makeNodeConfigPodTriggerNodeConfig(name, nodeLabelValue string) *scyllav1alpha1.NodeConfig {
	return &scyllav1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: scyllav1alpha1.NodeConfigSpec{
			Placement: scyllav1alpha1.NodeConfigPlacement{
				NodeSelector: map[string]string{
					nodeConfigPodTriggerNodeLabel: nodeLabelValue,
				},
			},
		},
	}
}

// nodeConfigPodTriggerFixture is the state the trigger rows change: two Nodes with a ScyllaDB Pod each, a
// non-ScyllaDB Pod next to the first one, a NodeConfig selecting the first Node and one selecting none.
type nodeConfigPodTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	node *corev1.Node
	// scyllaPod runs on node, otherScyllaPod on the other Node.
	scyllaPod      *corev1.Pod
	otherScyllaPod *corev1.Pod
	// plainPod runs on node, but isn't a ScyllaDB Pod.
	plainPod *corev1.Pod
	// nodeConfig selects node, idleNodeConfig no Node.
	nodeConfig     *scyllav1alpha1.NodeConfig
	idleNodeConfig *scyllav1alpha1.NodeConfig
}

// tuningConfigMap returns the tuning ConfigMap the controller created for pod.
func (f *nodeConfigPodTriggerFixture) tuningConfigMap(ctx context.Context, pod *corev1.Pod) *corev1.ConfigMap {
	g.GinkgoHelper()

	cm, err := f.env.TypedKubeClient().CoreV1().ConfigMaps(pod.Namespace).Get(ctx, naming.GetTuningConfigMapNameForPod(pod), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return cm
}

// These rows check which Pods each kind of change reconciles. The rows run in order against one fixture, and a
// failing row doesn't stop the rest, so every broken watch is reported at once. The rows creating an object come
// before the rows changing or deleting it.
var _ = g.Describe("NodeConfigPod controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		scyllaPodName         = "scylla"
		otherScyllaPodName    = "other-scylla"
		unscheduledScyllaName = "unscheduled-scylla"
		otherNodeName         = "envtest-other-node"
		emptyNodeName         = "envtest-empty-node"
		otherNodeConfigName   = "envtest-other-nc"
	)

	var f *nodeConfigPodTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running NodeConfigPod controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runNodeConfigPodController(controllerCtx, env, nodeconfigpod.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		// The API server taints a new Node as not ready, which no NodeConfig tolerates. Nothing in envtest would
		// remove the taint once the Node is ready, so the fixture does.
		g.By("Creating two ready Nodes")
		var nodes []*corev1.Node
		for _, node := range []struct{ name, labelValue string }{
			{name: "envtest-node", labelValue: "a"},
			{name: otherNodeName, labelValue: "b"},
		} {
			node, err := env.TypedKubeClient().CoreV1().Nodes().Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: node.name,
					Labels: map[string]string{
						nodeConfigPodTriggerNodeLabel: node.labelValue,
					},
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			node.Spec.Taints = nil
			node, err = env.TypedKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			nodes = append(nodes, node)
		}

		g.By("Creating a NodeConfig selecting the first Node and one selecting none")
		nodeConfig, err := env.ScyllaClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, makeNodeConfigPodTriggerNodeConfig("envtest-nc", "a"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		idleNodeConfig, err := env.ScyllaClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, makeNodeConfigPodTriggerNodeConfig("envtest-idle-nc", "none"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDB Pod on each Node and a non-ScyllaDB Pod on the first one")
		scyllaPod := createNodeConfigPodTriggerPod(ctx, env, makeNodeConfigPodTriggerPod(env.Namespace(), scyllaPodName, nodes[0].Name, true))
		otherScyllaPod := createNodeConfigPodTriggerPod(ctx, env, makeNodeConfigPodTriggerPod(env.Namespace(), otherScyllaPodName, nodes[1].Name, true))
		plainPod := createNodeConfigPodTriggerPod(ctx, env, makeNodeConfigPodTriggerPod(env.Namespace(), "plain", nodes[0].Name, false))

		f = &nodeConfigPodTriggerFixture{
			triggerActions: triggerActions{client: env.KubeClient()},
			env:            env,
			recorder:       recorder,
			node:           nodes[0],
			scyllaPod:      scyllaPod,
			otherScyllaPod: otherScyllaPod,
			plainPod:       plainPod,
			nodeConfig:     nodeConfig,
			idleNodeConfig: idleNodeConfig,
		}

		g.By("Waiting for the tuning ConfigMaps of the ScyllaDB Pods")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, pod := range []*corev1.Pod{scyllaPod, otherScyllaPod} {
				_, err := env.TypedKubeClient().CoreV1().ConfigMaps(pod.Namespace).Get(ctx, naming.GetTuningConfigMapNameForPod(pod), metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
			}
		}).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.DescribeTable("reconciles the Pods a change concerns",
		func(ctx g.SpecContext, row triggerRow[*nodeConfigPodTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		// The rows below cover the Pod handlers: they enqueue the Pod itself, if it is a ScyllaDB Pod.
		g.Entry("ScyllaDB Pod update", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.scyllaPod.DeepCopy())
			},
			expected: []string{scyllaPodName},
		}),
		g.Entry("non-ScyllaDB Pod update", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.plainPod.DeepCopy())
			},
		}),
		// An unscheduled Pod needs no tuning ConfigMap, so the controller reconciles it without waiting for its
		// container's status.
		g.Entry("ScyllaDB Pod creation", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, makeNodeConfigPodTriggerPod(f.env.Namespace(), unscheduledScyllaName, "", true))
			},
			expected: []string{unscheduledScyllaName},
		}),
		g.Entry("non-ScyllaDB Pod creation", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, makeNodeConfigPodTriggerPod(f.env.Namespace(), "unscheduled-plain", "", false))
			},
		}),
		// The controller registers no Pod deletion handler, but the API server marks a Pod for deletion before
		// removing it, even with no grace period, and the controller reconciles that update.
		g.Entry("ScyllaDB Pod deletion", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.delete(ctx, makeNodeConfigPodTriggerPod(f.env.Namespace(), unscheduledScyllaName, "", true))
			},
			expected: []string{unscheduledScyllaName},
		}),

		// The rows below cover the ConfigMap handlers: they enqueue the controlling Pod, if it is a ScyllaDB Pod.
		g.Entry("tuning ConfigMap update", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.tuningConfigMap(ctx, f.scyllaPod))
			},
			expected: []string{scyllaPodName},
		}),
		g.Entry("creation of a ConfigMap controlled by a non-ScyllaDB Pod", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "plain",
						Namespace: f.env.Namespace(),
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(f.plainPod, corev1.SchemeGroupVersion.WithKind("Pod")),
						},
					},
				})
			},
		}),
		g.Entry("unowned ConfigMap creation", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unowned",
						Namespace: f.env.Namespace(),
					},
				})
			},
		}),

		// The rows below cover the Node handler: it enqueues the ScyllaDB Pods on the Node, and only on an update.
		g.Entry("Node update", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.node.DeepCopy())
			},
			expected: []string{scyllaPodName},
		}),
		g.Entry("Node creation", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: emptyNodeName,
					},
				})
			},
		}),
		g.Entry("update of a Node without ScyllaDB Pods", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: emptyNodeName,
					},
				})
			},
		}),

		// The rows below cover the NodeConfig handlers: they enqueue the ScyllaDB Pods on the Nodes the NodeConfig
		// selects.
		g.Entry("NodeConfig update", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.nodeConfig.DeepCopy())
			},
			expected: []string{scyllaPodName},
		}),
		g.Entry("update of a NodeConfig selecting no Node", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.annotate(ctx, f.idleNodeConfig.DeepCopy())
			},
		}),
		g.Entry("NodeConfig creation", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.create(ctx, makeNodeConfigPodTriggerNodeConfig(otherNodeConfigName, "b"))
			},
			expected: []string{otherScyllaPodName},
		}),
		g.Entry("NodeConfig deletion", triggerRow[*nodeConfigPodTriggerFixture]{
			change: func(ctx context.Context, f *nodeConfigPodTriggerFixture) {
				f.delete(ctx, &scyllav1alpha1.NodeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: otherNodeConfigName,
					},
				})
			},
			expected: []string{otherScyllaPodName},
		}),
	)
})
