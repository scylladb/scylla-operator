// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "ScyllaDBManagerTaskController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scylladbmanagertask"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	// Unfortunately, Scylla Manager calls are synchronous, internally retried and can take ages.
	// Contrary to what it should be, this needs to be quite high.
	// FIXME: https://github.com/scylladb/scylla-operator/issues/2686
	maxSyncDuration = 2 * time.Minute
)

// Controller keeps the ScyllaDB Manager tasks in line with the ScyllaDBManagerTasks. It reads through a
// controller-runtime client with read-your-writes consistency.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache.
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
		client:    c,
		apiReader: apiReader,

		eventRecorder: eventRecorder,
	}
}

// ControllerOptions returns the controller options the controller runs with, including the bounded sync duration.
func ControllerOptions(maxConcurrentReconciles int) controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve the tasks through the
// manager's cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (smtc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBManagerTask{}).
		Watches(&scyllav1alpha1.ScyllaDBManagerClusterRegistration{}, handler.EnqueueRequestsFromMapFunc(mapRegistrationToTasks(mgr.GetCache()))).
		WithOptions(options).
		Complete(smtc)
}

// mapRegistrationToTasks enqueues every ScyllaDBManagerTask in the registration's namespace that refers to it.
func mapRegistrationToTasks(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
		if !ok {
			return nil
		}

		smts, err := ctrlclient.List[scyllav1alpha1.ScyllaDBManagerTask](ctx, cache, smcr.Namespace, labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaDBManagerTasks in namespace %q: %w", smcr.Namespace, err))
			return nil
		}

		var requests []reconcile.Request
		for _, smt := range smts {
			smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				continue
			}

			if smcrName != smcr.Name {
				continue
			}

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: smt.Namespace,
					Name:      smt.Name,
				},
			})
		}

		return requests
	}
}
