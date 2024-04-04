// Copyright (C) 2017 ScyllaDB

package retry

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

// Backoff specifies a policy for how long to wait between retries.
// It is called after a failing request, to determine the amount of time
// that should pass before trying again.
type Backoff = backoff.BackOff

// Stop indicates that no more retries should be made.
const Stop time.Duration = -1

// NewExponentialBackoff returns Backoff implementation that increases each
// wait period exponentially.
// Multiplier controls how fast each wait period grows, and randomizationFactor
// allows to inject some jitter between periods.
func NewExponentialBackoff(initialInterval, maxElapsedTime, maxInterval time.Duration, multiplier, randomizationFactor float64) Backoff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = initialInterval
	b.MaxElapsedTime = maxElapsedTime
	b.MaxInterval = maxInterval

	b.Multiplier = multiplier
	b.RandomizationFactor = randomizationFactor
	b.Reset()
	return b
}

// WithMaxRetries allows to set maximum number of retries for given backoff strategy.
func WithMaxRetries(b Backoff, maxRetries uint64) Backoff {
	return backoff.WithMaxRetries(b, maxRetries)
}

// BackoffFunc type is an adapter to allow the use of ordinary
// functions as Backoff.
type BackoffFunc func() time.Duration

// NextBackOff returns the duration to wait before retrying the operation.
func (b BackoffFunc) NextBackOff() time.Duration {
	return b()
}

// Reset to initial state.
func (b BackoffFunc) Reset() {}

// Clone returns a copy of BackoffFunc.
func (b BackoffFunc) Clone() Backoff {
	return b
}
