// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"time"

	"github.com/scylladb/go-log"
)

// Listener specifies pluggable hooks for scheduler events.
// Parametrized by scheduler key type.
type Listener[K comparable] interface {
	OnSchedulerStart(context.Context)
	OnSchedulerStop(context.Context)
	OnRunStart(ctx *RunContext[K])
	OnRunSuccess(ctx *RunContext[K])
	OnRunStop(ctx *RunContext[K], err error)
	OnRunWindowEnd(ctx *RunContext[K], err error)
	OnRunError(ctx *RunContext[K], err error)
	OnSchedule(ctx context.Context, key K, begin, end time.Time, retno int8)
	OnUnschedule(ctx context.Context, key K)
	OnTrigger(ctx context.Context, key K, success bool)
	OnStop(ctx context.Context, key K)
	OnRetryBackoff(ctx context.Context, key K, backoff time.Duration, retno int8)
	OnNoTrigger(ctx context.Context, key K)
	OnSleep(ctx context.Context, key K, d time.Duration)
}

type nopListener[K comparable] struct{}

func (l nopListener[_]) OnSchedulerStart(context.Context) {
}

func (l nopListener[_]) OnSchedulerStop(context.Context) {
}

func (l nopListener[K]) OnRunStart(*RunContext[K]) {
}

func (l nopListener[K]) OnRunSuccess(*RunContext[K]) {
}

func (l nopListener[K]) OnRunStop(*RunContext[K], error) {
}

func (l nopListener[K]) OnRunWindowEnd(*RunContext[K], error) {
}

func (l nopListener[K]) OnRunError(*RunContext[K], error) {
}

func (l nopListener[K]) OnSchedule(_ context.Context, _ K, _, _ time.Time, _ int8) {
}

func (l nopListener[K]) OnUnschedule(context.Context, K) {
}

func (l nopListener[K]) OnTrigger(context.Context, K, bool) {
}

func (l nopListener[K]) OnStop(context.Context, K) {
}

func (l nopListener[K]) OnRetryBackoff(context.Context, K, time.Duration, int8) {
}

func (l nopListener[K]) OnNoTrigger(context.Context, K) {
}

func (l nopListener[K]) OnSleep(context.Context, K, time.Duration) {
}

// NopListener returns a Listener implementation that has no effects.
func NopListener[K comparable]() Listener[K] {
	return nopListener[K]{}
}

type errorLogListener[K comparable] struct {
	nopListener[K]
	logger log.Logger
}

func (l errorLogListener[K]) OnRunError(ctx *RunContext[K], err error) {
	l.logger.Error(ctx, "OnRunError", "key", ctx.Key, "retry", ctx.Retry, "error", err)
}

func (l errorLogListener[K]) OnRunWindowEnd(ctx *RunContext[K], err error) {
	l.logger.Info(ctx, "OnRunWindowEnd", "key", ctx.Key, "retry", ctx.Retry, "error", err)
}

func (l errorLogListener[K]) OnRunStop(ctx *RunContext[K], err error) {
	l.logger.Info(ctx, "OnRunStop", "key", ctx.Key, "retry", ctx.Retry, "error", err)
}

// ErrorLogListener returns listener that logs errors.
func ErrorLogListener[K comparable](logger log.Logger) Listener[K] {
	return errorLogListener[K]{
		logger: logger,
	}
}
