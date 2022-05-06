package scyllaclient

import (
	"fmt"
	"testing"

	openapiruntime "github.com/go-openapi/runtime"
)

func TestStatusCodeOf(t *testing.T) {
	tt := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name: "OpenAPI error",
			err: &openapiruntime.APIError{
				Code: 401,
			},
			expectedCode: 401,
		},
		{
			name: "W wrapped OpenAPI error",
			err: fmt.Errorf("foo: %w", &openapiruntime.APIError{
				Code: 401,
			}),
			expectedCode: 401,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := StatusCodeOf(tc.err)

			if got != tc.expectedCode {
				t.Errorf("expected %d, got %d", tc.expectedCode, got)
			}
		})
	}
}
