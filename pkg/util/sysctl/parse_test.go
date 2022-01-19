package sysctl_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/sysctl"
)

func TestParseConfig(t *testing.T) {
	t.Parallel()

	ts := []struct {
		name          string
		config        string
		expectedError error
		expectedKV    map[string]string
	}{
		{
			name:          "empty config",
			config:        "",
			expectedKV:    map[string]string{},
			expectedError: nil,
		},
		{
			name: "config with comments",
			config: `# blablabla
token=value
; dsadsadsa`,
			expectedKV:    map[string]string{"token": "value"},
			expectedError: nil,
		},
		{
			name: "config with empty lines",
			config: `

token=value
`,
			expectedKV:    map[string]string{"token": "value"},
			expectedError: nil,
		},
		{
			name:          "config with extra spaces between separator",
			config:        `token   =    value`,
			expectedKV:    map[string]string{"token": "value"},
			expectedError: nil,
		},
		{
			name:          "leading and trailing spaces around token and value",
			config:        `     token     =    value      `,
			expectedKV:    map[string]string{"token": "value"},
			expectedError: nil,
		},
		{
			name:          "multi word value containing space",
			config:        `     token     = i have spaces     `,
			expectedKV:    map[string]string{"token": "i have spaces"},
			expectedError: nil,
		},
		{
			name: "multiple tokens and values",
			config: `
token1=value1
token2=value2
`,
			expectedKV:    map[string]string{"token1": "value1", "token2": "value2"},
			expectedError: nil,
		},
		{
			name:          "token with separators",
			config:        `kernel.fs.aio-max-nr = 3000000`,
			expectedKV:    map[string]string{"kernel.fs.aio-max-nr": "3000000"},
			expectedError: nil,
		},
		{
			name:          "wrong separator",
			config:        `kernel.fs.aio-max-nr: 3000000`,
			expectedKV:    nil,
			expectedError: fmt.Errorf("invalid syntax at line 1"),
		},
		{
			name:          "extra separator",
			config:        `kernel.fs.aio-max-nr == 3000000`,
			expectedKV:    nil,
			expectedError: fmt.Errorf("invalid syntax at line 1"),
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			kv, err := sysctl.ParseConfig(strings.NewReader(test.config))
			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf("expected %v error, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(kv, test.expectedKV) {
				t.Errorf("expected %v map, got %v", test.expectedKV, kv)
			}
		})
	}
}
