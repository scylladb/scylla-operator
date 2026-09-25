// Copyright (c) 2026 ScyllaDB.

package controllertools

import (
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRequeue(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		afters   []time.Duration
		expected reconcile.Result
	}{
		{
			name:     "no requeue requested",
			afters:   nil,
			expected: reconcile.Result{},
		},
		{
			name:     "the earliest requeue wins",
			afters:   []time.Duration{5 * time.Second, 2 * time.Second, 10 * time.Second},
			expected: reconcile.Result{RequeueAfter: 2 * time.Second},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rq := &Requeue{}
			for _, d := range tc.afters {
				rq.After(d)
			}

			got := rq.Result()
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
