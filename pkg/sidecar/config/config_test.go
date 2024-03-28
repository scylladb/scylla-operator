package config

import (
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties"
)

func TestCreateRackDCProperties(t *testing.T) {
	tests := map[string]struct {
		input *properties.Properties
		dc    string
		rack  string
		want  *properties.Properties
	}{
		"empty input": {
			input: properties.LoadMap(map[string]string{}),
			dc:    "dc",
			rack:  "rack",
			want:  properties.LoadMap(map[string]string{"dc": "dc", "rack": "rack", "prefer_local": "false"}),
		},
		"override dc": {
			input: properties.LoadMap(map[string]string{"dc": "dc2"}),
			dc:    "dc",
			rack:  "rack",
			want:  properties.LoadMap(map[string]string{"dc": "dc", "rack": "rack", "prefer_local": "false"}),
		},
		"override rack": {
			input: properties.LoadMap(map[string]string{"rack": "rack2"}),
			dc:    "dc",
			rack:  "rack",
			want:  properties.LoadMap(map[string]string{"dc": "dc", "rack": "rack", "prefer_local": "false"}),
		},
		"override prefer_local": {
			input: properties.LoadMap(map[string]string{"prefer_local": "true"}),
			dc:    "dc",
			rack:  "rack",
			want:  properties.LoadMap(map[string]string{"dc": "dc", "rack": "rack", "prefer_local": "true"}),
		},
		"override dc_suffix": {
			input: properties.LoadMap(map[string]string{"dc_suffix": "suffix"}),
			dc:    "dc",
			rack:  "rack",
			want:  properties.LoadMap(map[string]string{"dc": "dc", "rack": "rack", "prefer_local": "false", "dc_suffix": "suffix"}),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := createRackDCProperties(tc.input, tc.dc, tc.rack)
			if diff := cmp.Diff(tc.want.Map(), got.Map()); diff != "" {
				t.Fatalf("expected: %v, got: %v, diff: %s", tc.want, got, diff)
			}
		})
	}
}

func TestMergeYAMLs(t *testing.T) {
	tt := []struct {
		name        string
		initial     []byte
		overrides   [][]byte
		expected    []byte
		expectedErr error
	}{
		{
			name: "no overrides produce identical content",
			initial: []byte(`
key: value
`),
			overrides: nil,
			expected: []byte(`
key: value
`),
			expectedErr: nil,
		},
		{
			name: "one level keys",
			initial: []byte(`
foo: bar
key: value
`),
			overrides: [][]byte{
				[]byte(`
key: override_value
`),
			},
			expected: []byte(`
foo: bar
key: override_value
`),
			expectedErr: nil,
		},
		{
			name: "one level keys, last override wins",
			initial: []byte(`
foo: bar
key: value
`),
			overrides: [][]byte{
				[]byte(`
key: override_value_1
`),
				[]byte(`
key: "override_value_2"
`),
			},
			expected: []byte(`
foo: bar
key: override_value_2
`),
			expectedErr: nil,
		},
		{
			name: "initial comment",
			initial: []byte(`
#comment
`),
			overrides: [][]byte{
				[]byte(`
key: override_value
`),
			},
			expected: []byte(`
key: override_value
`),
			expectedErr: nil,
		},
		{
			name: "override comment",
			initial: []byte(`
key: value
`),
			overrides: [][]byte{
				[]byte(`
#comment
`),
			},
			expected: []byte(`
key: value
`),
			expectedErr: nil,
		},
		{
			name: "nested keys override on the top level",
			initial: []byte(`
foo: bar
key:
  nestedKey_1: value_1
  nestedKey_2: value_2
`),
			overrides: [][]byte{
				[]byte(`
key:
  nestedKey_1: override_value_1
`),
			},
			expected: []byte(`
foo: bar
key:
  nestedKey_1: override_value_1
`),
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := mergeYAMLs(tc.initial, tc.overrides...)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if tc.expected != nil {
				tc.expected = []byte(strings.TrimPrefix(string(tc.expected), "\n"))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual data differ: %s", cmp.Diff(string(tc.expected), string(got)))
			}
		})
	}
}

func TestAllowedCPUs(t *testing.T) {
	cpusAllowed, err := getCPUsAllowedList("./procstatus")
	if err != nil {
		t.Error(err)
	}
	t.Log(cpusAllowed)
}

func writeTempFile(t *testing.T, namePattern, content string) string {
	tmp, err := ioutil.TempFile(os.TempDir(), namePattern)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.WriteString(tmp, content); err != nil {
		t.Error(err)
	}
	if err := tmp.Close(); err != nil {
		t.Error(err)
	}

	return tmp.Name()
}

func TestScyllaArguments(t *testing.T) {
	ts := []struct {
		Name         string
		Args         string
		ExpectedArgs map[string]string
	}{
		{
			Name: "all combinations in one",
			Args: `--arg1 --arg2=val2 --arg3 "val3" --arg4="val4" --arg5=-1.23 --arg6 -123 --arg7 123`,
			ExpectedArgs: map[string]string{
				"arg1": "",
				"arg2": "val2",
				"arg3": `"val3"`,
				"arg4": `"val4"`,
				"arg5": "-1.23",
				"arg6": "-123",
				"arg7": "123",
			},
		},
		{
			Name:         "single empty flag",
			Args:         "--arg",
			ExpectedArgs: map[string]string{"arg": ""},
		},
		{
			Name:         "single flag",
			Args:         "--arg val",
			ExpectedArgs: map[string]string{"arg": "val"},
		},
		{
			Name:         "integer value",
			Args:         "--arg 123",
			ExpectedArgs: map[string]string{"arg": "123"},
		},
		{
			Name:         "negative integer value",
			Args:         "--arg -123",
			ExpectedArgs: map[string]string{"arg": "-123"},
		},
		{
			Name:         "float value",
			Args:         "--arg 1.23",
			ExpectedArgs: map[string]string{"arg": "1.23"},
		},
		{
			Name:         "negative float value",
			Args:         "--arg -1.23",
			ExpectedArgs: map[string]string{"arg": "-1.23"},
		},
		{
			Name:         "negative float value and string parameter",
			Args:         "--arg1 -1.23 --arg2 val",
			ExpectedArgs: map[string]string{"arg1": "-1.23", "arg2": "val"},
		},
		{
			Name:         "bool value",
			Args:         "--arg true",
			ExpectedArgs: map[string]string{"arg": "true"},
		},
		{
			Name:         "flag quoted",
			Args:         `--arg "val"`,
			ExpectedArgs: map[string]string{"arg": `"val"`},
		},
		{
			Name:         "flag quoted with equal sign",
			Args:         `--arg="val"`,
			ExpectedArgs: map[string]string{"arg": `"val"`},
		},
		{
			Name:         "empty",
			Args:         "",
			ExpectedArgs: map[string]string{},
		},
		{
			Name: "garbage inside",
			Args: "--arg1 val asdasdasdas --arg2=val",
			ExpectedArgs: map[string]string{
				"arg1": "val",
				"arg2": "val",
			},
		},
		{
			Name: "mixed types",
			Args: "--skip-wait-for-gossip-to-settle 0 --ring-delay-ms 5000 --compaction-enforce-min-threshold true --shadow-round-ms 1",
			ExpectedArgs: map[string]string{
				"skip-wait-for-gossip-to-settle":   "0",
				"ring-delay-ms":                    "5000",
				"compaction-enforce-min-threshold": "true",
				"shadow-round-ms":                  "1",
			},
		},
	}

	for _, test := range ts {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			argumentsMap := convertScyllaArguments(test.Args)
			if !reflect.DeepEqual(test.ExpectedArgs, argumentsMap) {
				t.Errorf("expected %+v, got %+v", test.ExpectedArgs, argumentsMap)
			}
		})
	}
}
