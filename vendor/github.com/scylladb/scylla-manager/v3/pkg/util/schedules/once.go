// Copyright (C) 2017 ScyllaDB

package schedules

import (
	"time"

	"go.uber.org/atomic"
)

type once struct {
	v *atomic.Bool
}

// NewOnce creates a trigger that fires once at a specified time.
// There is a sub second threshold to enable starting once now.
func NewOnce() Trigger {
	return once{
		v: atomic.NewBool(false),
	}
}

func (o once) Next(now time.Time) time.Time {
	if o.v.CompareAndSwap(false, true) {
		return now
	}
	return time.Time{}
}
