// Copyright (C) 2017 ScyllaDB

package retry

import (
	"context"

	"github.com/cenkalti/backoff"
)

// An Operation is executing by WithNotify().
// The operation will be retried using a backoff policy if it returns an error.
type Operation = backoff.Operation

// Notify is a notify-on-error function. It receives an operation error and
// backoff delay if the operation failed (with an error).
type Notify = backoff.Notify

// WithNotify calls notify function with the error and wait duration
// for each failed attempt before sleep.
func WithNotify(ctx context.Context, op Operation, b Backoff, n Notify) error {
	return backoff.RetryNotify(op, backoff.WithContext(b, ctx), n)
}

// Permanent wraps the given err in a *backoff.PermanentError.
// This error interrupts further retries and causes retrying mechanism.
func Permanent(err error) *backoff.PermanentError {
	return backoff.Permanent(err)
}
