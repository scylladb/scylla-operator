// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "RemoteKubernetesClusterController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "remotekubernetescluster"
)

// Controller keeps the remote cluster handlers (clients, caches) registered for every RemoteKubernetesCluster whose
// kubeconfig Secret is present, and reports the health of the connections into its status.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: the last
	// check before a finalizer is removed.
	apiReader client.Reader

	clusterKubeClient      remoteclient.ClusterClientInterface[kubernetes.Interface]
	clusterScyllaClient    remoteclient.ClusterClientInterface[scyllaclient.Interface]
	dynamicClusterHandlers []remoteclient.DynamicClusterInterface

	eventRecorder record.EventRecorder
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
	dynamicClusterHandlers []remoteclient.DynamicClusterInterface,
	clusterKubeClient remoteclient.ClusterClientInterface[kubernetes.Interface],
	clusterScyllaClient remoteclient.ClusterClientInterface[scyllaclient.Interface],
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		clusterKubeClient:      clusterKubeClient,
		clusterScyllaClient:    clusterScyllaClient,
		dynamicClusterHandlers: dynamicClusterHandlers,

		eventRecorder: eventRecorder,
	}
}

// SetupWithManager registers the controller with the manager: its RemoteKubernetesClusters, the Secrets they take
// their kubeconfig from and the ScyllaDBClusters referring to them re-run the sync.
func (rkcc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.RemoteKubernetesCluster{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapSecretToRemoteKubernetesClusters(cache))).
		Watches(&scyllav1alpha1.ScyllaDBCluster{}, handler.EnqueueRequestsFromMapFunc(mapScyllaDBClusterToRemoteKubernetesClusters(cache))).
		WithOptions(options).
		Complete(rkcc)
}

func requestFor(name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}
}

// mapSecretToRemoteKubernetesClusters enqueues the RemoteKubernetesClusters taking their kubeconfig from the Secret.
func mapSecretToRemoteKubernetesClusters(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		rkcs, err := ctrlclient.List[scyllav1alpha1.RemoteKubernetesCluster](ctx, cache, "", labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list RemoteKubernetesClusters: %w", err))
			return nil
		}

		var requests []reconcile.Request
		for _, rkc := range rkcs {
			if rkc.Spec.KubeconfigSecretRef.Namespace == obj.GetNamespace() && rkc.Spec.KubeconfigSecretRef.Name == obj.GetName() {
				requests = append(requests, requestFor(rkc.Name))
			}
		}

		return requests
	}
}

// mapScyllaDBClusterToRemoteKubernetesClusters enqueues the RemoteKubernetesClusters the ScyllaDBCluster's
// datacenters refer to.
func mapScyllaDBClusterToRemoteKubernetesClusters(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sc, ok := obj.(*scyllav1alpha1.ScyllaDBCluster)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("expected a ScyllaDBCluster, got %T", obj))
			return nil
		}

		var requests []reconcile.Request
		for _, dc := range sc.Spec.Datacenters {
			_, err := ctrlclient.Get[scyllav1alpha1.RemoteKubernetesCluster](ctx, cache, "", dc.RemoteKubernetesClusterName)
			if err != nil {
				apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get RemoteKubernetesCluster %q: %w", dc.RemoteKubernetesClusterName, err))
				continue
			}

			klog.V(4).InfoS("Enqueuing RemoteKubernetesCluster referenced by ScyllaDBCluster", "ScyllaDBCluster", klog.KObj(sc), "RemoteKubernetesCluster", dc.RemoteKubernetesClusterName)
			requests = append(requests, requestFor(dc.RemoteKubernetesClusterName))
		}

		return requests
	}
}
