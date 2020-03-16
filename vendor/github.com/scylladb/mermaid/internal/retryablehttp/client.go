// Copyright (C) 2017 ScyllaDB
//
// Modified version of github.com/hashicorp/go-retryablehttp.

package retryablehttp

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/scylladb/go-log"
)

var (
	// Default retry configuration
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
	defaultRetryMax     = 4

	// We need to consume response bodies to maintain http connections, but
	// limit the size we consume to respReadLimit.
	respReadLimit = int64(4096)
)

// ReaderFunc is the type of function that can be given natively to NewRequest
type ReaderFunc func() (io.Reader, error)

// LenReader is an interface implemented by many in-memory io.Reader's. Used
// for automatically sending the right Content-Length header when possible.
type LenReader interface {
	Len() int
}

// CheckRetry specifies a policy for handling retries. It is called
// following each request with the response and error values returned by
// the http.Client. If CheckRetry returns false, the Client stops retrying
// and returns the response to the caller. If CheckRetry returns an error,
// that error value is returned in lieu of the error from the request. The
// Client will close any response body when retrying, but if the retry is
// aborted it is up to the CheckResponse callback to properly close any
// response body before returning.
type CheckRetry func(req *http.Request, resp *http.Response, err error) (bool, error)

// Backoff specifies a policy for how long to wait between retries.
// It is called after a failing request to determine the amount of time
// that should pass before trying again.
type Backoff func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration

// ErrorHandler is called if retries are expired, containing the last status
// from the http library. If not specified, default behavior for the library is
// to close the body and return an error indicating how many tries were
// attempted. If overriding this, be sure to close the body if needed.
type ErrorHandler func(resp *http.Response, err error, numTries int) (*http.Response, error)

// Transport is used to make HTTP requests. It adds additional functionality
// like automatic retries to tolerate minor outages.
type Transport struct {
	parent http.RoundTripper
	logger log.Logger

	RetryWaitMin time.Duration // Minimum time to wait
	RetryWaitMax time.Duration // Maximum time to wait
	RetryMax     int           // Maximum number of retries

	// CheckRetry specifies the policy for handling retries, and is called
	// after each request. The default policy is DefaultRetryPolicy.
	CheckRetry CheckRetry

	// Backoff specifies the policy for how long to wait between retries
	Backoff Backoff

	// ErrorHandler specifies the custom error handler to use, if any
	ErrorHandler ErrorHandler
}

// NewTransport creates a new Transport with default settings.
func NewTransport(parent http.RoundTripper, logger log.Logger) *Transport {
	return &Transport{
		parent:       parent,
		logger:       logger,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   DefaultRetryPolicy,
		Backoff:      DefaultBackoff,
	}
}

// DefaultRetryPolicy provides a default callback for Client.CheckRetry, which
// will retry on connection errors and server errors.
func DefaultRetryPolicy(req *http.Request, resp *http.Response, err error) (bool, error) {
	if req != nil {
		if err := req.Context().Err(); err != nil {
			return false, err
		}
	}

	if err != nil {
		// RoundTripper can't handle connection resets so we are testing for
		// such errors
		switch t := err.(type) {
		case *net.OpError:
			if t.Op == "read" {
				return false, err
			}
		default:
			if err == io.EOF {
				return false, err
			}
		}
		return true, err
	}
	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != 501) {
		return true, nil
	}

	return false, nil
}

// DefaultBackoff provides a default callback for Client.Backoff which
// will perform exponential backoff based on the attempt number and limited
// by the provided minimum and maximum durations.
func DefaultBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}

// LinearJitterBackoff provides a callback for Client.Backoff which will
// perform linear backoff based on the attempt number and with jitter to
// prevent a thundering herd.
//
// min and max here are *not* absolute values. The number to be multiplied by
// the attempt number will be chosen at random from between them, thus they are
// bounding the jitter.
//
// For instance:
// * To get strictly linear backoff of one second increasing each retry, set
// both to one second (1s, 2s, 3s, 4s, ...)
// * To get a small amount of jitter centered around one second increasing each
// retry, set to around one second, such as a min of 800ms and max of 1200ms
// (892ms, 2102ms, 2945ms, 4312ms, ...)
// * To get extreme jitter, set to a very wide spread, such as a min of 100ms
// and a max of 20s (15382ms, 292ms, 51321ms, 35234ms, ...)
func LinearJitterBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	// attemptNum always starts at zero but we want to start at 1 for multiplication
	attemptNum++

	if max <= min {
		// Unclear what to do here, or they are the same, so return min *
		// attemptNum
		return min * time.Duration(attemptNum)
	}

	// Seed rand; doing this every time is fine
	rand := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	// Pick a random number that lies somewhere between the min and max and
	// multiply by the attemptNum. attemptNum starts at zero so we always
	// increment here. We first get a random percentage, then apply that to the
	// difference between min and max, and add to min.
	jitter := rand.Float64() * float64(max-min)
	jitterMin := int64(jitter) + int64(min)
	return time.Duration(jitterMin * int64(attemptNum))
}

// PassthroughErrorHandler is an ErrorHandler that directly passes through the
// values from the net/http library for the final request. The body is not
// closed.
func PassthroughErrorHandler(resp *http.Response, err error, _ int) (*http.Response, error) {
	return resp, err
}

// RoundTrip wraps calling parent RoundTrip method with retries.
func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		r    *http.Request
		resp *http.Response
		err  error
	)

	// Copy the request if contains body
	if req.Body != nil {
		r = new(http.Request)
		*r = *req
		r.Body = body{req.Body, false}
	} else {
		r = req
	}

loop:
	for i := 0; ; i++ {
		// Attempt the request
		resp, err = t.parent.RoundTrip(r)
		if err != nil {
			t.logger.Info(r.Context(), "request failed",
				"host", r.Host,
				"method", r.Method,
				"uri", r.URL.RequestURI(),
				"error", err,
			)
		}

		// Check if we should continue with retries.
		checkOK, checkErr := t.CheckRetry(r, resp, err)

		// Now decide if we should continue.
		if !checkOK {
			if checkErr != nil {
				err = checkErr
			}
			return resp, err
		}

		// If body was read do not continue.
		if b, ok := r.Body.(body); ok && b.read {
			return resp, err
		}

		// We do this before drainBody because there's no need for the I/O if
		// we're breaking out
		remain := t.RetryMax - i
		if remain <= 0 {
			break
		}

		// We're going to retry, consume any response to reuse the connection.
		if err == nil && resp != nil {
			t.drainBody(resp)
		}

		wait := t.Backoff(t.RetryWaitMin, t.RetryWaitMax, i, resp)
		t.logger.Info(r.Context(), "retrying",
			"host", r.Host,
			"method", r.Method,
			"uri", r.URL.RequestURI(),
			"wait", wait,
			"left", remain,
		)

		select {
		case <-req.Context().Done():
			// If we hit this point that means we are waiting for another
			// retry. break allows to preserve previous err after returning.
			// It was just coincidence that context was canceled while there
			// were errors.
			break loop
		case <-time.After(wait):
		}
	}

	if t.ErrorHandler != nil {
		return t.ErrorHandler(resp, err, t.RetryMax+1)
	}

	// By default, we close the response body and return an error without
	// returning the response
	if resp != nil {
		resp.Body.Close()
	}
	return nil, fmt.Errorf("giving up after %d attempts: %s", t.RetryMax+1, err)
}

// Try to read the response body so we can reuse this connection.
func (t *Transport) drainBody(resp *http.Response) {
	defer resp.Body.Close()
	_, err := io.Copy(ioutil.Discard, io.LimitReader(resp.Body, respReadLimit))
	if err != nil {
		t.logger.Info(resp.Request.Context(), "error reading response body", "error", err)
	}
}

type body struct {
	io.ReadCloser
	read bool
}

func (c body) Read(bs []byte) (int, error) {
	if !c.read {
		c.read = true
	}
	return c.ReadCloser.Read(bs)
}
