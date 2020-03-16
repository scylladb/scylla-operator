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
	ctxCustomTimeout
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

// customTimeout allows to pass a custom timeout to timeout middleware.
//
// WARNING: Usually this is a workaround for Scylla or other API slowness
// in field condition i.e. with tons of data. This is the last resort of
// defense please use with care.
func customTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, ctxCustomTimeout, d)
}
