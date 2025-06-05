// Copyright (C) 2025 ScyllaDB

package controllertools

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNonRetriable(t *testing.T) {
	t.Parallel()

	regularErr := errors.New("regular error")
	nonRetriableErr := NonRetriable(regularErr)

	tt := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      regularErr,
			expected: false,
		},
		{
			name:     "non-retriable error",
			err:      nonRetriableErr,
			expected: true,
		},
		{
			name:     "wrapped non-retriable error",
			err:      fmt.Errorf("wrapped: %w", nonRetriableErr),
			expected: true,
		},
		{
			name:     "aggregated non-retriable error",
			err:      errors.Join(nonRetriableErr, regularErr),
			expected: true,
		},
		{
			name:     "aggregated, wrapped non-retriable error",
			err:      errors.Join(fmt.Errorf("wrapped: %w", nonRetriableErr), regularErr),
			expected: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IsNonRetriable(tc.err)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	t.Parallel()

	regularErr := errors.New("regular error")
	nonRetriableErr := NonRetriable(regularErr)

	type customErr struct {
		error
	}

	tt := []struct {
		name     string
		err      error
		target   any
		expected bool
	}{
		{
			name:     "nil err as non-retriable",
			err:      nil,
			target:   &NonRetriableError{},
			expected: false,
		},
		{
			name:     "regular err as non-retriable",
			err:      regularErr,
			target:   &NonRetriableError{},
			expected: false,
		},
		{
			name:     "non-retriable regular err as non-retriable",
			err:      nonRetriableErr,
			target:   &NonRetriableError{},
			expected: true,
		},
		{
			name:     "wrapped non-retriable regular err as non-retriable",
			err:      fmt.Errorf("wrapped: %w", nonRetriableErr),
			target:   &NonRetriableError{},
			expected: true,
		},
		{
			name:     "aggregated non-retriable regular err as non-retriable",
			err:      errors.Join(nonRetriableErr, regularErr),
			target:   &NonRetriableError{},
			expected: true,
		},
		{
			name:     "non-retriable regular err as regular",
			err:      nonRetriableErr,
			target:   &regularErr,
			expected: true,
		},
		{
			name:     "non-retriable custom err as custom",
			err:      NonRetriable(customErr{errors.New("custom error")}),
			target:   &customErr{},
			expected: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := errors.As(tc.err, tc.target)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
