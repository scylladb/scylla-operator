// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	scyllav2models "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/models"
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
	case interface {
		GetPayload() *scyllav2models.ErrorModel
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Code)
		}
	}
	return 0
}
