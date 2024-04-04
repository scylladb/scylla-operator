// Copyright (C) 2017 ScyllaDB

package trigger

import (
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
)

type multi []scheduler.Trigger

// NewMulti returns a trigger that joins multiple triggers.
func NewMulti(t ...scheduler.Trigger) scheduler.Trigger {
	return multi(t)
}

func (m multi) Next(now time.Time) time.Time {
	var min time.Time
	for _, t := range m {
		next := t.Next(now)
		if min.IsZero() {
			min = next
		} else if !next.IsZero() && next.Before(min) {
			min = next
		}
	}
	return min
}
