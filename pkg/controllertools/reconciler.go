// Copyright (c) 2026 ScyllaDB.

package controllertools

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SingletonRequest is the one request an observer reconciles: observers don't reconcile an object, they re-derive one
// process-wide state from everything they watch.
func SingletonRequest(name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}
}

// EnqueueSingleton returns an event handler that enqueues the singleton request of name on every event.
func EnqueueSingleton(name string) handler.EventHandler {
	request := SingletonRequest(name)

	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{request}
	})
}

type observerReconciler struct {
	name     string
	syncFunc ObserverSyncFunc
}

var _ reconcile.Reconciler = observerReconciler{}

// NewObserverReconciler adapts an observer's sync function to a reconciler. Every request is the singleton one, so the
// sync is run for any of them. Conflicts and already-exists errors are retried quietly, and a NonRetriable error is
// dropped instead of retried.
func NewObserverReconciler(name string, syncFunc ObserverSyncFunc) reconcile.Reconciler {
	return observerReconciler{
		name:     name,
		syncFunc: syncFunc,
	}
}

func (r observerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	err := r.syncFunc(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		return reconcile.Result{}, nil

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Observer", r.name, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Observer", r.name, "Error", err)

	case IsNonRetriable(err):
		klog.InfoS("Hit non-retriable error. Dropping the item from the queue.", "Observer", r.name, "Error", err)
		return reconcile.Result{}, reconcile.TerminalError(err)
	}

	return reconcile.Result{}, fmt.Errorf("sync loop has failed: %w", err)
}

// Requeue collects the delay after which a reconciliation wants to run again, for the steps that poll an external
// state instead of waiting for a watch event. It replaces the direct queue access of the client-go controllers.
type Requeue struct {
	after time.Duration
}

// After requeues after d, or sooner if an earlier requeue was requested.
func (r *Requeue) After(d time.Duration) {
	if r.after == 0 || d < r.after {
		r.after = d
	}
}

// Result returns the reconcile result carrying the requested requeue.
func (r *Requeue) Result() reconcile.Result {
	return reconcile.Result{RequeueAfter: r.after}
}

// Trigger lets code outside the watches, e.g. a timer or a test, enqueue an observer's singleton request.
type Trigger struct {
	ch chan event.GenericEvent
}

func NewTrigger() *Trigger {
	return &Trigger{
		// One pending event is enough: the workqueue deduplicates the request anyway.
		ch: make(chan event.GenericEvent, 1),
	}
}

// Enqueue requests a sync. It never blocks; a request is dropped only when one is already pending.
func (t *Trigger) Enqueue() {
	select {
	case t.ch <- event.GenericEvent{Object: &metav1.PartialObjectMetadata{}}:
	default:
	}
}

// Source returns the event source to register with the controller of name.
func (t *Trigger) Source(name string) source.Source {
	return source.Channel(t.ch, EnqueueSingleton(name))
}

// PeriodicTrigger returns a manager runnable that requests a sync through trigger every interval.
func PeriodicTrigger(trigger *Trigger, interval time.Duration) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				trigger.Enqueue()
			}
		}
	})
}
