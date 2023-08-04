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
	ctxHost ctxt = iota
	ctxNoRetry
	ctxCustomTimeout
)

// forceHost makes hostPool middleware use the given host instead of selecting
// one.
func forceHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, ctxHost, host)
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

func hasCustomTimeout(ctx context.Context) (time.Duration, bool) {
	v, ok := ctx.Value(ctxCustomTimeout).(time.Duration)
	return v, ok
}
