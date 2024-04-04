// Copyright (C) 2017 ScyllaDB

package trigger

import (
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
)

type legacy struct {
	startDate time.Time
	interval  time.Duration
}

// NewLegacy returns Trigger based on interval duration that was used in
// Scylla Manager 2.x and before.
func NewLegacy(startDate time.Time, interval time.Duration) scheduler.Trigger {
	return legacy{startDate: startDate, interval: interval}
}

func (l legacy) Next(now time.Time) time.Time {
	if l.startDate.After(now) {
		return l.startDate
	}
	if l.interval == 0 {
		return time.Time{}
	}
	lastStart := l.startDate.Add(now.Sub(l.startDate).Round(l.interval))
	for lastStart.Before(now) {
		lastStart = lastStart.Add(l.interval)
	}
	return lastStart
}
