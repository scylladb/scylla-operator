// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_parseSSTableBootstrappedQueryResult(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		data                []byte
		expected            bool
		expectedErrorString string
	}{
		{
			name:                "no data",
			data:                []byte(``),
			expected:            false,
			expectedErrorString: "can't unmarshall scylla-sstable query result: unexpected end of JSON input",
		},
		{
			name:                "invalid json",
			data:                []byte(`]`),
			expected:            false,
			expectedErrorString: "can't unmarshall scylla-sstable query result: invalid character ']' looking for beginning of value",
		},
		{
			name:                "empty array",
			data:                []byte(`[]`),
			expected:            false,
			expectedErrorString: "",
		},
		{
			name:                "empty struct",
			data:                []byte(`[{}]`),
			expected:            false,
			expectedErrorString: "",
		},
		{
			name:                "bootstrapped",
			data:                []byte(`[{"bootstrapped":"COMPLETED"}]`),
			expected:            true,
			expectedErrorString: "",
		},
		{
			name:                "not bootstrapped",
			data:                []byte(`[{"bootstrapped":"NEEDS_BOOTSTRAP"}]`),
			expected:            false,
			expectedErrorString: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSSTableBootstrappedQueryResult(tc.data)

			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			if !cmp.Equal(errStr, tc.expectedErrorString) {
				t.Fatalf("expected and got error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}

			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
