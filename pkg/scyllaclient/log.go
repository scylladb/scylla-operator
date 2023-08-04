// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/util/httpx"
	"github.com/scylladb/scylla-operator/pkg/util/timeutc"
	"k8s.io/klog/v2"
)

// requestLogger logs requests and responses.
func requestLogger(next http.RoundTripper) http.RoundTripper {
	return httpx.RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		start := timeutc.Now()
		resp, err = next.RoundTrip(req)
		logReqResp(timeutc.Since(start), req, resp)
		return
	})
}

func logReqResp(elapsed time.Duration, req *http.Request, resp *http.Response) {
	details := []interface{}{
		"host", req.Host,
		"method", req.Method,
		"uri", req.URL.RequestURI(),
		"duration", fmt.Sprintf("%dms", elapsed.Milliseconds()),
	}
	if resp != nil {
		details = append(details,
			"status", resp.StatusCode,
			"bytes", resp.ContentLength,
		)

		// Dump body of failed requests, ignore 404s
		if c := resp.StatusCode; c >= 400 && c != http.StatusNotFound {
			if b, err := httputil.DumpResponse(resp, true); err != nil {
				details = append(details, "dump", errors.Wrap(err, "dump request"))
			} else {
				details = append(details, "dump", string(b))
			}
		}
	}
	klog.V(6).InfoS("scyllaclient request", details...)
}
