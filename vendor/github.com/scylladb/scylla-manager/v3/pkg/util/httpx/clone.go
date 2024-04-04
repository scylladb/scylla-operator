// Copyright (C) 2017 ScyllaDB

package httpx

import (
	"net/http"
	"net/url"
)

// CloneRequest creates a shallow copy of the request along with a deep copy
// of the Headers and URL.
func CloneRequest(req *http.Request) *http.Request {
	r := new(http.Request)

	// Shallow clone
	*r = *req

	// Copy ctx
	r = r.WithContext(req.Context())

	// Deep copy headers
	r.Header = CloneHeader(req.Header)

	// Deep copy URL
	r.URL = new(url.URL)
	*r.URL = *req.URL

	return r
}

// CloneHeader creates a deep copy of an http.Header.
func CloneHeader(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, values := range in {
		newValues := make([]string, len(values))
		copy(newValues, values)
		out[key] = newValues
	}
	return out
}
