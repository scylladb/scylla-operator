package controllertools

import (
	"errors"
	"fmt"
	"testing"
)

func TestNonRetriable(t *testing.T) {
	t.Parallel()

	regularErr := errors.New("regular error")
	nonRetriableErr := NonRetriable(regularErr)

	tt := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "regular error", err: regularErr, expected: false},
		{name: "non-retriable error", err: nonRetriableErr, expected: true},
		{name: "wrapped non-retriable error", err: fmt.Errorf("wrapped: %w", nonRetriableErr), expected: true},
		{name: "non-retriable message", err: NewNonRetriable("message"), expected: true},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsNonRetriable(tc.err); got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}

	if !errors.Is(nonRetriableErr, regularErr) {
		t.Errorf("expected the non-retriable error to unwrap to the wrapped one")
	}
}
