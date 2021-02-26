// Copyright (C) 2017 ScyllaDB

package slices

import (
	"testing"
)

func TestContainsString(t *testing.T) {
	tt := []struct {
		name     string
		s        string
		array    []string
		expected bool
	}{
		{
			name:     "empty",
			s:        "a",
			array:    []string{},
			expected: false,
		},
		{
			name:     "contain element",
			s:        "a",
			array:    []string{"a", "b"},
			expected: true,
		},
		{
			name:     "at the end",
			s:        "b",
			array:    []string{"a", "b"},
			expected: true,
		},
		{
			name:     "does not contain element",
			s:        "c",
			array:    []string{"a", "b"},
			expected: false,
		},
	}
	for _, tc := range tt {
		t.Run(t.Name(), func(t *testing.T) {
			got := ContainsString(tc.s, tc.array)
			if got != tc.expected {
				t.Errorf("expected %t, got %t", tc.expected, got)
			}
		})
	}
}
