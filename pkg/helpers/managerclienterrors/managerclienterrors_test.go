// Copyright (C) 2025 ScyllaDB

package managerclienterrors

import (
	"testing"

	"github.com/go-openapi/runtime"
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
