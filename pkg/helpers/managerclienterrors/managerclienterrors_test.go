// Copyright (C) 2025 ScyllaDB

package managerclienterrors

import (
	"errors"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	managerclientoperations "github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/client/operations"
)

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "client operations response not found",
			err:      managerclientoperations.NewGetClusterClusterIDDefault(404),
			expected: true,
		},
		{
			name:     "client operations response other",
			err:      managerclientoperations.NewGetClusterClusterIDDefault(500),
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
