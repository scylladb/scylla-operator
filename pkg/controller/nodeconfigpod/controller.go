// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "NodeConfigPodController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "nodeconfigpod"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	maxSyncDuration = 30 * time.Second
)

var (
	podControllerGVK = corev1.SchemeGroupVersion.WithKind("Pod")
)

// Controller keeps the runtime ConfigMap of every ScyllaDB Pod in sync with the NodeConfigs selecting its Node.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption.
	apiReader client.Reader

	eventRecorder record.EventRecorder
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
) *Controller {
	return &Controller{
		client:        c,
		apiReader:     apiReader,
		eventRecorder: eventRecorder,
	}
}

// ControllerOptions returns the controller options the controller runs with on top of the caller's concurrency: a
// bounded sync duration.
func ControllerOptions(maxConcurrentReconciles int) controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve the Pods to sync through
// the manager's cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (ncpc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&corev1.Pod{}, ctrlbuilder.WithPredicates(
			predicate.NewPredicateFuncs(isScyllaPod),
			// Deletions of Pods don't re-run the sync: the ConfigMap is owned and goes with the Pod.
			predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return false }},
		)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapConfigMapToScyllaPodOwner(cache))).
		// Only updates of Nodes re-run the sync of the Pods on them.
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(mapNodeToScyllaPodsOnIt(cache)), ctrlbuilder.WithPredicates(predicate.Funcs{
			CreateFunc:  func(event.CreateEvent) bool { return false },
			UpdateFunc:  func(event.UpdateEvent) bool { return true },
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		})).
		Watches(&scyllav1alpha1.NodeConfig{}, handler.EnqueueRequestsFromMapFunc(mapNodeConfigToScyllaPodsOnSelectedNodes(cache))).
		WithOptions(options).
		Complete(ncpc)
}

func isScyllaPod(obj client.Object) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}

	return controllerhelpers.IsScyllaPod(pod)
}

func requestFor(pod *corev1.Pod) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
	}
}

// mapConfigMapToScyllaPodOwner enqueues the ScyllaDB Pod controlling the ConfigMap.
func mapConfigMapToScyllaPodOwner(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		controllerRef := metav1.GetControllerOf(obj)
		if controllerRef == nil || controllerRef.Kind != podControllerGVK.Kind {
			return nil
		}

		pod, err := ctrlclient.Get[corev1.Pod](ctx, cache, obj.GetNamespace(), controllerRef.Name)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(obj.GetNamespace(), controllerRef.Name), err))
			return nil
		}

		if pod.UID != controllerRef.UID || !controllerhelpers.IsScyllaPod(pod) {
			return nil
		}

		return []reconcile.Request{requestFor(pod)}
	}
}

func scyllaPodsOnNode(ctx context.Context, cache client.Reader, nodeName string) ([]*corev1.Pod, error) {
	allPods, err := ctrlclient.List[corev1.Pod](ctx, cache, corev1.NamespaceAll, naming.ScyllaSelector())
	if err != nil {
		return nil, fmt.Errorf("can't list ScyllaDB Pods: %w", err)
	}

	var pods []*corev1.Pod
	for _, pod := range allPods {
		if pod.Spec.NodeName == nodeName && controllerhelpers.IsScyllaPod(pod) {
			pods = append(pods, pod)
		}
	}

	return pods, nil
}

// mapNodeToScyllaPodsOnIt enqueues every ScyllaDB Pod scheduled on the Node.
func mapNodeToScyllaPodsOnIt(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pods, err := scyllaPodsOnNode(ctx, cache, obj.GetName())
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return nil
		}

		klog.V(4).InfoS("Enqueuing all pods on Node", "Pods", len(pods), "Node", klog.KObj(obj))
		requests := make([]reconcile.Request, 0, len(pods))
		for _, pod := range pods {
			requests = append(requests, requestFor(pod))
		}

		return requests
	}
}

// mapNodeConfigToScyllaPodsOnSelectedNodes enqueues every ScyllaDB Pod scheduled on a Node the NodeConfig selects.
func mapNodeConfigToScyllaPodsOnSelectedNodes(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		nodeConfig, ok := obj.(*scyllav1alpha1.NodeConfig)
		if !ok {
			return nil
		}

		allNodes, err := ctrlclient.List[corev1.Node](ctx, cache, corev1.NamespaceAll, labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list Nodes: %w", err))
			return nil
		}

		var requests []reconcile.Request
		nodeCount := 0
		for _, node := range allNodes {
			matching, err := controllerhelpers.IsNodeConfigSelectingNode(nodeConfig, node)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				return nil
			}

			if !matching {
				continue
			}
			nodeCount++

			pods, err := scyllaPodsOnNode(ctx, cache, node.Name)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				return nil
			}

			for _, pod := range pods {
				requests = append(requests, requestFor(pod))
			}
		}

		klog.V(4).InfoS("Enqueuing all Scylla Pods for NodeConfig", "NodeConfig", klog.KObj(nodeConfig), "NodeCount", nodeCount)
		return requests
	}
}
