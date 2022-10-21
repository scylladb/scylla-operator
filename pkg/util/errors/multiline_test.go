package errors

import (
	"errors"
	"testing"
)

func TestNewAggregate(t *testing.T) {
	tt := []struct {
		name           string
		errList        []error
		sep            string
		expectedString string
	}{
		{
			name:           "nil list results in an empty string",
			errList:        nil,
			sep:            "\n",
			expectedString: "",
		},
		{
			name: "multiline error is correctly formatted",
			errList: []error{
				errors.New("foo"),
				nil,
				errors.New("bar"),
			},
			sep:            "\n",
			expectedString: "foo\nbar",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := NewAggregate(tc.errList, tc.sep)

			var gotString string
			if got != nil {
				gotString = got.Error()
			}
			if gotString != tc.expectedString {
				t.Errorf("expected error %q, got %q", tc.expectedString, gotString)
			}
		})
	}
}
