// Copyright (c) 2026 ScyllaDB.

package controllertools

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
