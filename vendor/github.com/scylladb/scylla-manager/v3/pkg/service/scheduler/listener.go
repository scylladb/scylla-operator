// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"time"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
)

// Listener instantiated with Key type.
type Listener = scheduler.Listener[Key]

type schedulerListener struct {
	Listener
	find   func(key Key) (taskInfo, bool)
	logger log.Logger
}

func newSchedulerListener(find func(key Key) (taskInfo, bool), logger log.Logger) schedulerListener {
	return schedulerListener{
		Listener: scheduler.ErrorLogListener[Key](logger),
		find:     find,
		logger:   logger,
	}
}

func (l schedulerListener) OnSchedule(ctx context.Context, key Key, begin, end time.Time, retno int8) {
	in := begin.Sub(now()).Truncate(time.Second)
	if end.IsZero() {
		l.logKey(ctx, key, "Schedule",
			"in", in,
			"begin", begin,
			"retry", retno,
		)
	} else {
		l.logKey(ctx, key, "Schedule in window",
			"in", in,
			"begin", begin,
			"end", end,
			"retry", retno,
		)
	}
}

func (l schedulerListener) OnUnschedule(ctx context.Context, key Key) {
	l.logKey(ctx, key, "Unschedule")
}

func (l schedulerListener) Trigger(ctx context.Context, key Key, success bool) {
	l.logKey(ctx, key, "Trigger", "success", success)
}

func (l schedulerListener) OnStop(ctx context.Context, key Key) {
	l.logKey(ctx, key, "Stop")
}

func (l schedulerListener) OnRetryBackoff(ctx context.Context, key Key, backoff time.Duration, retno int8) {
	l.logKey(ctx, key, "Retry backoff", "backoff", backoff, "retry", retno)
}

func (l schedulerListener) OnNoTrigger(ctx context.Context, key Key) {
	l.logKey(ctx, key, "No trigger")
}

func (l schedulerListener) OnSleep(ctx context.Context, key Key, d time.Duration) {
	l.logger.Debug(ctx, "OnSleep", "task_id", key, "duration", d)
}

func (l schedulerListener) logKey(ctx context.Context, key Key, msg string, keyvals ...interface{}) {
	ti, ok := l.find(key)
	if !ok {
		return
	}
	if ti.TaskType == HealthCheckTask {
		l.logger.Debug(ctx, msg, prependTaskInfo(ti, keyvals)...)
	} else {
		l.logger.Info(ctx, msg, prependTaskInfo(ti, keyvals)...)
	}
}

func prependTaskInfo(ti taskInfo, i []interface{}) []interface{} {
	v := make([]interface{}, len(i)+2)
	v[0], v[1] = "task", ti
	copy(v[2:], i)
	return v
}
