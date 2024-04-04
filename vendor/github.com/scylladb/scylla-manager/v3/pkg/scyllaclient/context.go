// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"time"
)

// ctxt is a context key type.
type ctxt byte

// ctxt enumeration.
const (
	ctxInteractive ctxt = iota
	ctxHost
	ctxNoRetry
	ctxNoTimeout
	ctxCustomTimeout
	ctxShouldRetryHandler
)

// Interactive context means that it should be processed fast without too much
// useless waiting.
func Interactive(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxInteractive, true)
}

func isInteractive(ctx context.Context) bool {
	_, ok := ctx.Value(ctxInteractive).(bool)
	return ok
}

// ClientContextWithSelectedHost is a public method that returns copy of the given context,
// but extended with the selected host that will be hit with client calls.
func ClientContextWithSelectedHost(ctx context.Context, host string) context.Context {
	return forceHost(ctx, host)
}

// forceHost makes hostPool middleware use the given host instead of selecting
// one.
func forceHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, ctxHost, host)
}

func isForceHost(ctx context.Context) bool {
	_, ok := ctx.Value(ctxHost).(string)
	return ok
}

// noRetry disables retries.
func noRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxNoRetry, true)
}

// noTimeout disables timeouts - if in doubt do not use it.
// This should only be used by functions that handle timeouts internally.
func noTimeout(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxNoTimeout, true)
}

// customTimeout allows to pass a custom timeout to timeout middleware.
//
// WARNING: Usually this is a workaround for Scylla or other API slowness
// in field condition i.e. with tons of data. This is the last resort of
// defense please use with care.
func customTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, ctxCustomTimeout, d)
}

func hasCustomTimeout(ctx context.Context) (time.Duration, bool) {
	v, ok := ctx.Value(ctxCustomTimeout).(time.Duration)
	return v, ok
}

// shouldRetryHandlerFunc returns
// true if error should be retried,
// false if error is permanent,
// nil if handler cannot decide.
type shouldRetryHandlerFunc func(err error) *bool

func withShouldRetryHandler(ctx context.Context, f shouldRetryHandlerFunc) context.Context {
	return context.WithValue(ctx, ctxShouldRetryHandler, f)
}

func shouldRetryHandler(ctx context.Context) shouldRetryHandlerFunc {
	f, _ := ctx.Value(ctxShouldRetryHandler).(shouldRetryHandlerFunc)
	return f
}
