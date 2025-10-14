// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"errors"

	"github.com/go-openapi/runtime"
	scyllav2models "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/models"
)

// StatusCodeOf returns HTTP status code carried by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeOf(err error) int {
	type coder interface {
		Code() int
	}
	if v := new(coder); errors.As(err, v) {
		return (*v).Code()
	}

	type payloader interface {
		GetPayload() *scyllav2models.ErrorModel
	}
	if v := new(payloader); errors.As(err, v) {
		p := (*v).GetPayload()
		if p != nil {
			return int(p.Code)
		}
	}

	if v := new(runtime.APIError); errors.As(err, &v) {
		return v.Code
	}

	return 0
}
