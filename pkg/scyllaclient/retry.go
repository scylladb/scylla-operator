// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"net/url"
	"time"

	"github.com/scylladb/scylla-operator/pkg/util/retry"

	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
)

type retryableTransport struct {
	transport         runtime.ClientTransport
	config            BackoffConfig
	interactiveConfig BackoffConfig
	poolSize          int
	logger            log.Logger
}

type retryableOperation struct {
	retryableTransport
	operation *runtime.ClientOperation

	result   interface{}
	attempts int
}

// retryable wraps parent and adds retry capabilities.
func retryable(transport runtime.ClientTransport, config *Config, logger log.Logger) runtime.ClientTransport {
	return retryableTransport{
		transport:         transport,
		config:            config.Backoff,
		interactiveConfig: config.InteractiveBackoff,
		poolSize:          len(config.Hosts),
		logger:            logger,
	}
}

func (t retryableTransport) Submit(operation *runtime.ClientOperation) (interface{}, error) {
	if _, ok := operation.Context.Value(ctxNoRetry).(bool); ok {
		v, err := t.transport.Submit(operation)
		return v, unpackURLError(err)
	}

	o := retryableOperation{
		retryableTransport: t,
		operation:          operation,
	}
	return o.submit()
}

func (o *retryableOperation) submit() (interface{}, error) {
	err := retry.WithNotify(o.operation.Context, o.op, o.backoff(), o.notify)
	if err != nil {
		return nil, errors.Wrapf(unpackURLError(err), "giving up after %d attempts", o.attempts)
	}
	return o.result, nil
}

func (o *retryableOperation) op() (err error) {
	o.attempts++

	o.result, err = o.transport.Submit(o.operation)
	if err != nil && !o.shouldRetry(err) {
		err = retry.Permanent(err)
	}

	return
}

func (o *retryableOperation) backoff() retry.Backoff {
	if isForceHost(o.operation.Context) {
		if isInteractive(o.operation.Context) {
			return backoff(o.interactiveConfig)
		}
		return backoff(o.config)
	}

	// We want to send request to every host in the pool once.
	// The -1 is to avoid reaching out to the first node - that failed.
	maxRetries := o.poolSize - 1
	return noBackoff(maxRetries)
}

func (o *retryableOperation) shouldRetry(err error) bool {
	if o.operation.Context.Err() != nil {
		return false
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	c := StatusCodeOf(err)
	if c == 0 || (c >= 500 && c != 501) {
		return true
	}

	// Additionally if request can be resent to a different host retry
	// on Unauthorized or Forbidden.
	if !isForceHost(o.operation.Context) {
		if c == 401 || c == 403 {
			return true
		}
	}

	return false
}

func (o *retryableOperation) notify(err error, wait time.Duration) {
	if wait == 0 {
		o.logger.Info(o.operation.Context, "HTTP retry now",
			"operation", o.operation.ID,
			"error", unpackURLError(err),
		)
	} else {
		o.logger.Info(o.operation.Context, "HTTP retry backoff",
			"operation", o.operation.ID,
			"wait", wait,
			"error", unpackURLError(err),
		)
	}
}

func backoff(config BackoffConfig) retry.Backoff {
	return retry.WithMaxRetries(retry.NewExponentialBackoff(
		config.WaitMin,
		0,
		config.WaitMax,
		config.Multiplier,
		config.Jitter,
	), config.MaxRetries)
}

func noBackoff(maxRetries int) retry.Backoff {
	return retry.WithMaxRetries(retry.BackoffFunc(func() time.Duration { return 0 }), uint64(maxRetries))
}

func unpackURLError(err error) error {
	if e, ok := err.(*url.Error); ok {
		return e.Err
	}

	return err
}
