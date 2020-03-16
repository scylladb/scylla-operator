// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	agentModels "github.com/scylladb/mermaid/scyllaclient/internal/agent/models"
	scylla2Models "github.com/scylladb/mermaid/scyllaclient/internal/scylla_v2/models"
)

// StatusCodeOf returns HTTP status code carried by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeOf(err error) int {
	err = errors.Cause(err)
	switch v := err.(type) {
	case *runtime.APIError:
		return v.Code
	case interface {
		GetPayload() *agentModels.ErrorResponse
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Status)
		}
	case interface {
		GetPayload() *scylla2Models.ErrorModel
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Result)
		}
	}
	return 0
}
