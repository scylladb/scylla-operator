// Copyright (C) 2017 ScyllaDB

package middleware

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/hailocab/go-hostpool"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/internal/retryablehttp"
	"github.com/scylladb/mermaid/internal/timeutc"
)

// The RoundTripperFunc type is an adapter to allow the use of ordinary
// functions as RoundTrippers. If f is a function with the appropriate
// signature, RountTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// AuthToken sets authorization header. If token is empty it immediately returns
// the next handler.
func AuthToken(next http.RoundTripper, token string) http.RoundTripper {
	if token == "" {
		return next
	}

	return RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		r := cloneRequest(req)
		r.Header.Set("Authorization", "Bearer "+token)
		return next.RoundTrip(r)
	})
}

// FixContentType adjusts Scylla REST API response so that it can be consumed
// by Open API.
func FixContentType(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		defer func() {
			if resp != nil {
				// Force JSON, Scylla returns "text/plain" that misleads the
				// unmarshaller and breaks processing.
				resp.Header.Set("Content-Type", "application/json")
			}
		}()
		return next.RoundTrip(req)
	})
}

// Retry retries request if needed.
func Retry(next http.RoundTripper, poolSize int, logger log.Logger) http.RoundTripper {
	// Retry policy while using a specified host.
	hostRetry := retryablehttp.NewTransport(next, logger)

	// Retry policy while using host pool, no backoff wait time, switch host as
	// fast as possible.
	poolRetry := retryablehttp.NewTransport(next, logger)
	poolRetry.Backoff = func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration { return 0 }
	poolRetry.RetryMax = poolSize

	return RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		if _, ok := req.Context().Value(ctxDontRetry).(bool); ok {
			return next.RoundTrip(req)
		}

		if _, ok := req.Context().Value(ctxHost).(string); ok {
			return hostRetry.RoundTrip(req)
		}

		return poolRetry.RoundTrip(req)
	})
}

// HostPool sets request host from a pool.
func HostPool(next http.RoundTripper, pool hostpool.HostPool, port string) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		ctx := req.Context()

		var (
			h   string
			hpr hostpool.HostPoolResponse
		)

		// Get host from context
		h, ok := ctx.Value(ctxHost).(string)

		// Get host from pool
		if !ok {
			hpr = pool.Get()
			h = hpr.Host()
		}

		// Clone request
		r := cloneRequest(req)

		// Set host and port
		hp := net.JoinHostPort(h, port)
		r.Host = hp
		r.URL.Host = hp

		// RoundTrip shall not modify requests, here we modify it to fix error
		// messages see https://github.com/scylladb/mermaid/issues/266.
		// This is legit because we own the whole process. The modified request
		// is not being sent.
		req.Host = h
		req.URL.Host = h

		resp, err := next.RoundTrip(r)

		// Mark response
		if hpr != nil {
			hpr.Mark(err)
		}

		return resp, err
	})
}

// Logger logs requests and responses.
func Logger(next http.RoundTripper, logger log.Logger) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		start := timeutc.Now()
		defer func() {
			d := timeutc.Since(start) / 1000000
			f := []interface{}{
				"host", req.Host,
				"method", req.Method,
				"uri", req.URL.RequestURI(),
				"duration", fmt.Sprintf("%dms", d),
			}

			logFn := logger.Debug

			if resp != nil {
				f = append(f,
					"status", resp.StatusCode,
					"bytes", resp.ContentLength,
				)

				// Dump body of failed requests, ignore 404s
				if c := resp.StatusCode; c >= 400 && c != http.StatusNotFound {
					if b, err := httputil.DumpResponse(resp, true); err != nil {
						f = append(f, "dump", errors.Wrap(err, "dump request"))
					} else {
						f = append(f, "dump", string(b))
					}
					logFn = logger.Info
				}
			}

			logFn(req.Context(), "HTTP", f...)
		}()
		return next.RoundTrip(req)
	})
}

// body defers context cancellation until response body is closed.
type body struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b body) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}

// Timeout sets request context timeout for individual requests.
func Timeout(next http.RoundTripper, timeout time.Duration, logger log.Logger) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer func() {
			if resp != nil {
				resp.Body = body{resp.Body, cancel}
			}
			if ctx.Err() != nil {
				logger.Info(ctx, "Transport request timeout", "timeout", timeout, "err", ctx.Err())
			}
		}()
		return next.RoundTrip(req.WithContext(ctx))
	})
}

// cloneRequest creates a shallow copy of the request along with a deep copy
// of the Headers and URL.
func cloneRequest(req *http.Request) *http.Request {
	r := new(http.Request)

	// Shallow clone
	*r = *req

	// Deep copy headers
	r.Header = cloneHeader(req.Header)

	// Deep copy URL
	r.URL = new(url.URL)
	*r.URL = *req.URL

	return r
}

// cloneHeader creates a deep copy of an http.Header.
func cloneHeader(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, values := range in {
		newValues := make([]string, len(values))
		copy(newValues, values)
		out[key] = newValues
	}
	return out
}
