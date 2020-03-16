package scyllaclient

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/scylladb/scylla-operator/pkg/util/httpx"

	"github.com/pkg/errors"
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
		d, ok := req.Context().Value(ctxCustomTimeout).(time.Duration)
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
				err = errors.Errorf("timeout after %s", d)
			}
		}()
		return next.RoundTrip(req.WithContext(ctx))
	})
}
