// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "NodeConfigController"

	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "nodeconfig"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	maxSyncDuration = 30 * time.Second
)

var (
	nodeConfigControllerGVK = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
)

// Controller reconciles NodeConfigs: it manages the node tuning namespace, RBAC, DaemonSets and ConfigMaps for them
// and aggregates the per-node status the daemons report. It reads through a controller-runtime client with
// read-your-writes consistency.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption.
	apiReader client.Reader

	eventRecorder record.EventRecorder

	operatorImage string
}

var _ reconcile.Reconciler = &Controller{}

func isManagedByNodeConfigController(obj client.Object) bool {
	return obj.GetLabels()[naming.NodeConfigNameLabel] == naming.NodeConfigAppName
}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
	operatorImage string,
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		eventRecorder: eventRecorder,

		operatorImage: operatorImage,
	}
}

// ControllerOptions returns the controller options the NodeConfig controller runs with on top of the given worker
// count: a bounded sync duration.
func ControllerOptions(maxConcurrentReconciles int) controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve owners through the manager's
// cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (ncc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	// The RBAC objects, ServiceAccounts and Namespaces the controller manages are shared by all NodeConfigs, so
	// every NodeConfig is enqueued when one of them changes.
	enqueueAllForManaged := handler.EnqueueRequestsFromMapFunc(mapToAllNodeConfigs(cache))
	managedOnly := ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(isManagedByNodeConfigController))

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.NodeConfig{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&scyllav1alpha1.ScyllaOperatorConfig{}, handler.EnqueueRequestsFromMapFunc(mapToAllNodeConfigs(cache))).
		Watches(&corev1.Namespace{}, enqueueAllForManaged, managedOnly).
		Watches(&corev1.ServiceAccount{}, enqueueAllForManaged, managedOnly).
		Watches(&rbacv1.ClusterRole{}, enqueueAllForManaged, managedOnly).
		Watches(&rbacv1.ClusterRoleBinding{}, enqueueAllForManaged, managedOnly).
		Watches(&rbacv1.Role{}, enqueueAllForManaged, managedOnly).
		Watches(&rbacv1.RoleBinding{}, enqueueAllForManaged, managedOnly).
		// TODO: react to label changes on nodes
		WithOptions(options).
		Complete(ncc)
}

func requestFor(name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}
}

// mapToAllNodeConfigs enqueues every NodeConfig, for the objects all of them depend on.
func mapToAllNodeConfigs(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		ncs, err := ctrlclient.List[scyllav1alpha1.NodeConfig](ctx, cache, corev1.NamespaceAll, labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list NodeConfigs: %w", err))
			return nil
		}

		klog.V(4).InfoS("Enqueuing all NodeConfigs", "Object", klog.KObj(obj), "Count", len(ncs))
		requests := make([]reconcile.Request, 0, len(ncs))
		for _, nc := range ncs {
			requests = append(requests, requestFor(nc.Name))
		}

		return requests
	}
}
