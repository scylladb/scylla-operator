// Copyright (C) 2017 ScyllaDB

package schedules

import (
	"time"
)

type multi []Trigger

// NewMulti returns a trigger that joins multiple triggers.
func NewMulti(t ...Trigger) Trigger {
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
