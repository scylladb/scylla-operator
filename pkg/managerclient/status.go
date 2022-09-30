// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/managerclient/gen/models"
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

// MessageOf returns error message embedded in returned error.
func MessageOf(err error) string {
	err = errors.Cause(err)
	switch v := err.(type) {
	case interface {
		GetPayload() *models.ErrorResponse
	}:
		return v.GetPayload().Message
	}
	return err.Error()
}
