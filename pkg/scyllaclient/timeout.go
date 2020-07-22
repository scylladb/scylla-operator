// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/util/httpx"
)

var (
	// ErrTimeout is returned when request times out.
	ErrTimeout = errors.New("timeout")
)

// body defers context cancellation until response body is closed.
type body struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b body) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}

// timeout sets request context timeout for individual requests.
func timeout(next http.RoundTripper, timeout time.Duration) http.RoundTripper {
	return httpx.RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		d, ok := hasCustomTimeout(req.Context())
		if !ok {
			d = timeout
		}

		ctx, cancel := context.WithTimeout(req.Context(), d)
		defer func() {
			if resp != nil {
				resp.Body = body{
					ReadCloser: resp.Body,
					cancel:     cancel,
				}
			}

			if errors.Cause(err) == context.DeadlineExceeded && ctx.Err() == context.DeadlineExceeded {
				err = errors.Wrapf(ErrTimeout, "after %s", d)
			}
		}()
		return next.RoundTrip(req.WithContext(ctx))
	})
}
