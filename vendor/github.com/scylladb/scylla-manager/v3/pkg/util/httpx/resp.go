// Copyright (C) 2017 ScyllaDB

package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// MakeResponse creates a minimal response for a request and status code.
func MakeResponse(req *http.Request, statusCode int) *http.Response {
	return &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Request:    req,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
}

// MakeAgentErrorResponse creates an agent error response that can be used in testing.
func MakeAgentErrorResponse(req *http.Request, statusCode int, msg string) *http.Response {
	resp := MakeResponse(req, statusCode)
	resp.Header.Set("Content-type", "application/json")
	resp.Body = io.NopCloser(bytes.NewBufferString(fmt.Sprintf(`{"status": %d, "message": "%s"}`, statusCode, msg))) // nolint: gocritic
	return resp
}
