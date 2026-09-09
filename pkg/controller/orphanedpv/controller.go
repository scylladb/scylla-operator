package orphanedpv

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ControllerName = "OrphanedPVController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "orphanedpv"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	maxSyncDuration = 30 * time.Second

	// resyncPeriod is how often every ScyllaDBDatacenter is re-synced, to make sure to reconcile if we were to miss any
	// event given the current architecture of this controller.
	resyncPeriod = 30 * time.Minute
)

// Controller watches all PVs actively belonging to a ScyllaDBDatacenter and replace scylla node
// on any PV that is orphaned, if enabled on the ScyllaDBDatacenter.
// Orphaned PV is a volume that is hard bound to a node that doesn't exist anymore.
// The controller is based on a ScyllaDBDatacenter key and listing matching PVs because using a PV key and trying to
// find a corresponding ScyllaDBDatacenter would be quite hard, there are no ownerRefs and we can't
// propagate the "enabled" information from a ScyllaDBDatacenter to a PVC annotation because PVCs are not
// reconciled.
// TODO: When we support auto-replacing nodes, we could replace it with generic controller
//
//	deleting PVs bound to nodes that don't exists anymore, without knowing about ScyllaDBDatacenter.
//	It would also process PVs instead of ScyllaDBDatacenter which is currently complicating the logic
//	that has to handle multiple PVs at once, artificial requeues / not watching PVs and different error paths.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: verifying a
	// Node is gone before replacing the node on its volume.
	apiReader client.Reader

	eventRecorder record.EventRecorder

	// resyncCh carries the ScyllaDBDatacenters enqueued outside their watch: the periodic resync.
	resyncCh chan event.GenericEvent
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

		resyncCh: make(chan event.GenericEvent),
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

// SetupWithManager registers the controller with the manager: every ScyllaDBDatacenter event re-runs its sync, a Node
// going away re-runs every ScyllaDBDatacenter, and so does the periodic resync.
func (opc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBDatacenter{}).
		// Only a Node going away, or being replaced under the same name, can orphan a volume.
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(mapToAllScyllaDBDatacenters(cache)), ctrlbuilder.WithPredicates(predicate.Funcs{
			CreateFunc:  func(event.CreateEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return e.ObjectOld.GetUID() != e.ObjectNew.GetUID() },
			DeleteFunc:  func(event.DeleteEvent) bool { return true },
			GenericFunc: func(event.GenericEvent) bool { return false },
		})).
		WatchesRawSource(source.Channel(opc.resyncCh, &handler.EnqueueRequestForObject{})).
		WithOptions(options).
		Complete(opc)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	err = mgr.Add(ctrlmanager.RunnableFunc(func(ctx context.Context) error {
		opc.runPeriodicResync(ctx, cache)
		return nil
	}))
	if err != nil {
		return fmt.Errorf("can't add periodic resync: %w", err)
	}

	return nil
}

// runPeriodicResync enqueues every ScyllaDBDatacenter every resyncPeriod until ctx is done.
func (opc *Controller) runPeriodicResync(ctx context.Context, cache client.Reader) {
	ticker := time.NewTicker(resyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		klog.V(4).InfoS("Periodically enqueuing all ScyllaDBDatacenters")

		sdcs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, corev1.NamespaceAll, labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			continue
		}

		for _, sdc := range sdcs {
			select {
			case <-ctx.Done():
				return
			case opc.resyncCh <- event.GenericEvent{Object: sdc}:
			}
		}
	}
}

// Make sure we always have an aggregate to process and all nested errors are flattened.

// mapToAllScyllaDBDatacenters enqueues every ScyllaDBDatacenter.
func mapToAllScyllaDBDatacenters(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		klog.V(4).InfoS("Observed deletion of Node, enqueuing all ScyllaDBDatacenters", "Node", klog.KObj(obj))

		sdcs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, corev1.NamespaceAll, labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return nil
		}

		requests := make([]reconcile.Request, 0, len(sdcs))
		for _, sdc := range sdcs {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(sdc)})
		}

		return requests
	}
}
