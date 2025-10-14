package scyllaclient_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	scyllav2models "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/models"
)

type mockCoderError struct {
	code int
}

func (e *mockCoderError) Code() int {
	return e.code
}

func (e *mockCoderError) Error() string {
	return fmt.Sprintf("mock coder error with code %d", e.code)
}

type mockPayloaderError struct {
	payload *scyllav2models.ErrorModel
}

func (e *mockPayloaderError) GetPayload() *scyllav2models.ErrorModel {
	return e.payload
}

func (e *mockPayloaderError) Error() string {
	if e.payload != nil {
		return fmt.Sprintf("mock payloader error with code %d", e.payload.Code)
	}
	return "mock payloader error with nil payload"
}

func TestStatusCodeOf(t *testing.T) {
	testCases := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "coder interface - direct error",
			err:          &mockCoderError{code: 404},
			expectedCode: 404,
		},
		{
			name:         "coder interface - wrapped error",
			err:          fmt.Errorf("wrapped: %w", &mockCoderError{code: 500}),
			expectedCode: 500,
		},
		{
			name:         "payloader interface with valid payload - direct error",
			err:          &mockPayloaderError{payload: &scyllav2models.ErrorModel{Code: 400}},
			expectedCode: 400,
		},
		{
			name:         "payloader interface with valid payload - wrapped error",
			err:          fmt.Errorf("wrapped: %w", &mockPayloaderError{payload: &scyllav2models.ErrorModel{Code: 403}}),
			expectedCode: 403,
		},
		{
			name:         "payloader interface with nil payload - direct error",
			err:          &mockPayloaderError{payload: nil},
			expectedCode: 0,
		},
		{
			name:         "payloader interface with nil payload - wrapped error",
			err:          fmt.Errorf("wrapped: %w", &mockPayloaderError{payload: nil}),
			expectedCode: 0,
		},
		{
			name:         "runtime.APIError - direct error",
			err:          &runtime.APIError{Code: 502},
			expectedCode: 502,
		},
		{
			name:         "runtime.APIError - wrapped error",
			err:          fmt.Errorf("wrapped: %w", &runtime.APIError{Code: 503}),
			expectedCode: 503,
		},
		{
			name:         "unknown error type - direct error",
			err:          errors.New("some unknown error"),
			expectedCode: 0,
		},
		{
			name:         "unknown error type - wrapped error",
			err:          fmt.Errorf("wrapped: %w", errors.New("some unknown error")),
			expectedCode: 0,
		},
		{
			name:         "nil error",
			err:          nil,
			expectedCode: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scyllaclient.StatusCodeOf(tc.err)
			if result != tc.expectedCode {
				t.Errorf("StatusCodeOf(%v) = %d, expected %d", tc.err, result, tc.expectedCode)
			}
		})
	}
}
