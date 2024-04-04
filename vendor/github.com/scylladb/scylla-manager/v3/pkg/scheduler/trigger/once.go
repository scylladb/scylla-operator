// Copyright (C) 2017 ScyllaDB

package trigger

import (
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
	"go.uber.org/atomic"
)

type once struct {
	v *atomic.Bool
}

// NewOnce creates a trigger that fires once at a specified time.
// There is a sub second threshold to enable starting once now.
func NewOnce() scheduler.Trigger {
	return once{
		v: atomic.NewBool(false),
	}
}

func (o once) Next(now time.Time) time.Time {
	if o.v.CAS(false, true) {
		return now
	}
	return time.Time{}
}
