// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"errors"

	openapiruntime "github.com/go-openapi/runtime"
	pkgerrors "github.com/pkg/errors"
	agentModels "github.com/scylladb/scylla-operator/pkg/scyllaclient/internal/agent/models"
	scylla2Models "github.com/scylladb/scylla-operator/pkg/scyllaclient/internal/scylla_v2/models"
)

// StatusCodeOf returns HTTP status code carried by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeOf(sourceErr error) int {
	err := errors.Unwrap(sourceErr)
	if err == nil {
		err = sourceErr
	}
	err = pkgerrors.Cause(err)
	switch v := err.(type) {
	case interface {
		Code() int
	}:
		return v.Code()

	case *openapiruntime.APIError:
		return v.Code

	case interface {
		GetPayload() *agentModels.ErrorResponse
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Status)
		}
		return 0

	case interface {
		GetPayload() *scylla2Models.ErrorModel
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Code)
		}
		return 0

	default:
		return 0
	}
}
