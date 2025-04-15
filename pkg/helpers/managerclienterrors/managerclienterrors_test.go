// Copyright (C) 2025 ScyllaDB

package managerclienterrors

import (
	"errors"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
)

type mockCodeResponse struct {
	code int
}

func (c mockCodeResponse) Error() string {
	return "mock code response error"
}

func (c mockCodeResponse) Code() int {
	return c.code
}

func TestCodeForError(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: 0,
		},
		{
			name:     "code response error",
			err:      mockCodeResponse{code: 500},
			expected: 500,
		},
		{
			name:     "runtime api error",
			err:      runtime.NewAPIError("op", "payload", 500),
			expected: 500,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := codeForError(tc.err)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestIsBadRequest(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "code response bad request",
			err:      mockCodeResponse{code: 400},
			expected: true,
		},
		{
			name:     "code response other",
			err:      mockCodeResponse{code: 500},
			expected: false,
		},
		{
			name:     "runtime api error bad request",
			err:      runtime.NewAPIError("op", "payload", 400),
			expected: true,
		},
		{
			name:     "runtime api error other",
			err:      runtime.NewAPIError("op", "payload", 500),
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IsBadRequest(tc.err)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "code response not found",
			err:      mockCodeResponse{code: 404},
			expected: true,
		},
		{
			name:     "code response other",
			err:      mockCodeResponse{code: 500},
			expected: false,
		},
		{
			name:     "runtime api error not found",
			err:      runtime.NewAPIError("op", "payload", 404),
			expected: true,
		},
		{
			name:     "runtime api error other",
			err:      runtime.NewAPIError("op", "payload", 500),
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IsNotFound(tc.err)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

type mockPayloadErr struct {
	message string
	payload *managerclient.ErrorResponse
}

func (m mockPayloadErr) Error() string {
	return m.message
}

func (m mockPayloadErr) GetPayload() *managerclient.ErrorResponse {
	return m.payload
}

func TestGetPayloadMessage(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: "regular error",
		},
		{
			name: "err with payload",
			err: mockPayloadErr{
				message: "mock message",
				payload: &managerclient.ErrorResponse{
					Message: "payload message",
				},
			},
			expected: "payload message",
		},

		{
			name: "err with nil payload",
			err: mockPayloadErr{
				message: "mock message",
				payload: nil,
			},
			expected: "mock message",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GetPayloadMessage(tc.err)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
