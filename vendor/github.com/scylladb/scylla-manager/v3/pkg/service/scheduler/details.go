// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
)

func details(t *Task) scheduler.Details {
	return scheduler.Details{
		Properties: t.Properties,
		Trigger:    t.Sched.trigger(),
		Backoff:    t.Sched.backoff(),
		Window:     t.Sched.Window.Window(),
		Location:   t.Sched.Timezone.Location(),
	}
}
