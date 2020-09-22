// Copyright (C) 2017 ScyllaDB

package mermaidclient

import (
	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
)

// StatusCodeOf returns HTTP status code carried by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeOf(err error) int {
	err = errors.Cause(err)
	switch v := err.(type) {
	case interface {
		Code() int
	}:
		return v.Code()
	case *runtime.APIError:
		return v.Code
	}
	return 0
}
