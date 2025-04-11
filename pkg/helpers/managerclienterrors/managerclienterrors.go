// Copyright (C) 2025 ScyllaDB

package managerclienterrors

import (
	"errors"
	"net/http"

	"github.com/go-openapi/runtime"
)

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
