// Copyright (C) 2025 ScyllaDB

package managerclienterrors

import (
	"errors"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
)

func IsBadRequest(err error) bool {
	return codeForError(err) == http.StatusBadRequest
}

func IsNotFound(err error) bool {
	return codeForError(err) == http.StatusNotFound
}

func codeForError(err error) int {
	type codeResponse interface {
		Code() int
	}

	if codeResponseErr, ok := err.(codeResponse); ok {
		return codeResponseErr.Code()
	}

	var runtimeAPIError *runtime.APIError
	if errors.As(err, &runtimeAPIError) {
		return runtimeAPIError.Code
	}

	return 0
}

// GetPayloadMessage extracts the message from an error that implements the GetPayload method.
// If the error does not implement the GetPayload method, it returns the error's string representation.
func GetPayloadMessage(err error) string {
	var payloadResponse interface {
		GetPayload() *managerclient.ErrorResponse
	}
	if errors.As(err, &payloadResponse) {
		payload := payloadResponse.GetPayload()
		if payload != nil {
			return payload.Message
		}
	}

	return err.Error()
}
