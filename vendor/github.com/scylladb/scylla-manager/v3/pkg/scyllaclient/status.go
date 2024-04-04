// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	agentModels "github.com/scylladb/scylla-manager/v3/swagger/gen/agent/models"
	scyllaModels "github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v1/models"
	scylla2Models "github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v2/models"
)

// StatusCodeAndMessageOf returns HTTP status code and it's message carried
// by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeAndMessageOf(err error) (status int, message string) {
	cause := errors.Cause(err)
	switch v := cause.(type) { // nolint: errorlint
	case *runtime.APIError:
		return v.Code, fmt.Sprint(v.Response)
	case interface {
		GetPayload() *agentModels.ErrorResponse
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Status), p.Message
		}
	case interface {
		GetPayload() *scylla2Models.ErrorModel
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Code), p.Message
		}
	case interface {
		GetPayload() *scyllaModels.ErrorModel
	}:
		p := v.GetPayload()
		if p != nil {
			return int(p.Code), p.Message
		}
	case interface { // nolint: gofumpt
		Code() int
	}:
		return v.Code(), ""
	}

	return 0, ""
}

// StatusCodeOf returns HTTP status code carried by the error or it's cause.
// If not status can be found it returns 0.
func StatusCodeOf(err error) int {
	s, _ := StatusCodeAndMessageOf(err)
	return s
}

// agentError replicates OpenAPI behaviour in situations where agent needs to
// be accessed manually i.e. with custom logic to handle HTTP request and response.
type agentError struct {
	payload *agentModels.ErrorResponse
}

func makeAgentError(resp *http.Response) error {
	if resp.StatusCode/100 == 2 {
		return nil
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read body")
	}
	ae := agentError{
		payload: new(agentModels.ErrorResponse),
	}
	if err := json.Unmarshal(b, ae.payload); err != nil {
		return errors.Errorf("agent [HTTP %d] cannot read response: %s", resp.StatusCode, err)
	}

	return ae
}

func (ae agentError) GetPayload() *agentModels.ErrorResponse {
	return ae.payload
}

func (ae agentError) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", ae.payload.Status, ae.payload.Message)
}
