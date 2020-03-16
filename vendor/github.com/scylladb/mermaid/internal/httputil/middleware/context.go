// Copyright (C) 2017 ScyllaDB

package middleware

import "context"

// ctxt is a context key type.
type ctxt byte

// ctxt enumeration.
const (
	ctxHost ctxt = iota
	ctxDontRetry
)

// ForceHost makes HostPool middleware use the given host instead of selecting
// one.
func ForceHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, ctxHost, host)
}

// DontRetry disables Retry middleware.
func DontRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxDontRetry, true)
}
