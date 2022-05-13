package arg

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewArgsFromSlice(t *testing.T) {
	tt := []struct {
		name        string
		args        []string
		expected    Args
		expectedErr error
	}{
		{
			name:        "nil args succeed",
			args:        nil,
			expected:    Args{},
			expectedErr: nil,
		},
		{
			name:        "empty args succeed",
			args:        []string{},
			expected:    Args{},
			expectedErr: nil,
		},
		{
			name: "repeated args succeed",
			args: []string{
				"--no-arg",
				"--with-arg=wa",
				"--complex=cv",
			},
			expected: Args{
				{
					Flag:  "--no-arg",
					Value: nil,
				},
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewArgsFromSlice(tc.args)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and real error differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected and real result differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
