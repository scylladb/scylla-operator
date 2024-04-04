// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-manager/v3/pkg/util/httpx"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
)

// requestLogger logs requests and responses.
func requestLogger(next http.RoundTripper, logger log.Logger) http.RoundTripper {
	return httpx.RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		start := timeutc.Now()
		resp, err = next.RoundTrip(req)
		logReqResp(logger, timeutc.Since(start), req, resp)
		return
	})
}

func logReqResp(logger log.Logger, elapsed time.Duration, req *http.Request, resp *http.Response) {
	f := []interface{}{
		"host", req.Host,
		"method", req.Method,
		"uri", req.URL.RequestURI(),
		"duration", fmt.Sprintf("%dms", elapsed.Milliseconds()),
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
}
