// Copyright (c) 2026 ScyllaDB.

package controllertools

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
