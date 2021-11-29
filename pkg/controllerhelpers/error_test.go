package controllerhelpers

import (
	"testing"
)

func TestRequeueError_Error(t *testing.T) {
	tt := []struct {
		name                string
		reasons             []string
		expectedErrorString string
	}{
		{
			name:                "no reason",
			reasons:             []string{},
			expectedErrorString: "",
		},
		{
			name:                "single reason",
			reasons:             []string{"first error"},
			expectedErrorString: "first error",
		},
		{
			name:                "multiple reasons",
			reasons:             []string{"first error", "second error"},
			expectedErrorString: "first error, second error",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := NewRequeueError(tc.reasons...).Error()

			if got != tc.expectedErrorString {
				t.Errorf("expected %q, got %q", tc.expectedErrorString, got)
			}
		})
	}
}
