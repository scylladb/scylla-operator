// Copyright (C) 2017 ScyllaDB

package parallel

import (
	"go.uber.org/atomic"
	"go.uber.org/multierr"
)

// NoLimit means full parallelism mode.
const NoLimit = 0

// ErrAbort is a special kind of error that aborts all further execution.
// Function calls that are in progress will continue to execute but no new
// functions will be called.
type ErrAbort struct {
	error
}

// Abort is special kind of error that aborts all further execution.
func Abort(err error) ErrAbort {
	return ErrAbort{error: err}
}

func isErrAbort(err error) (bool, error) {
	a, ok := err.(ErrAbort)
	if !ok {
		return false, nil
	}
	return true, a.error
}

// Run executes function f with arguments ranging from 0 to n-1 executing at
// most limit in parallel.
// If limit is 0 it runs f(0),f(1),...,f(n-1) in parallel.
func Run(n, limit int, f func(i int) error) error {
	if limit <= 0 || limit > n {
		limit = n
	}

	var (
		idx  = atomic.NewInt32(0)
		out  = make(chan error)
		abrt = atomic.NewBool(false)
	)
	for j := 0; j < limit; j++ {
		go func() {
			for {
				// Exit when there is nothing to do
				i := int(idx.Inc()) - 1
				if i >= n {
					return
				}

				// Exit if aborted
				if abrt.Load() {
					out <- nil
					continue
				}

				// Execute
				err := f(i)
				if ok, inner := isErrAbort(err); ok {
					abrt.Store(true)
					err = inner
				}
				out <- err
			}
		}()
	}

	var errs error
	for i := 0; i < n; i++ {
		errs = multierr.Append(errs, <-out)
	}
	return errs
}
