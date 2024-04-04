package pacer

import "context"

// ctxt is a context key type.
type ctxt byte

// ctxt enumeration.
const (
	ctxRetries ctxt = iota
)

// RetriesCtx returns number of retries set for the context.
// If retries are not specified for the context it returns false.
func RetriesCtx(ctx context.Context) (int, bool) {
	retries, ok := ctx.Value(ctxRetries).(int)
	return retries, ok
}

// WithRetries sets number of retries for the context.
func WithRetries(ctx context.Context, count int) context.Context {
	return context.WithValue(ctx, ctxRetries, count)
}
