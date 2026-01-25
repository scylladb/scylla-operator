package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/equality"
)

func TestMergeSnitchConfigs(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		userConfig     *properties.Properties
		operatorConfig *properties.Properties
		expectedConfig *properties.Properties
	}{
		{
			name: "values provided by Operator are preferred over user provided ones for dc, rack and prefer_local keys",
			userConfig: properties.LoadMap(map[string]string{
				"dc":           "user-dc",
				"rack":         "user-rack",
				"prefer_local": "true",
			}),
			operatorConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
			}),
			expectedConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
			}),
		},
		{
			name: "dc_suffix is taken from user config",
			userConfig: properties.LoadMap(map[string]string{
				"dc":           "user-dc",
				"rack":         "user-rack",
				"prefer_local": "true",
				"dc_suffix":    "user-dc-suffix",
			}),
			operatorConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
				"dc_suffix":    "operator-dc-suffix",
			}),
			expectedConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
				"dc_suffix":    "user-dc-suffix",
			}),
		},
		{
			name:       "operator config is rewritten when user config is not created",
			userConfig: nil,
			operatorConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
			}),
			expectedConfig: properties.LoadMap(map[string]string{
				"dc":           "operator-dc",
				"rack":         "operator-rack",
				"prefer_local": "false",
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			userConfigPath := filepath.Join(tmpDir, "user.properties")
			operatorConfigPath := filepath.Join(tmpDir, "operator.properties")
			resultConfigPath := filepath.Join(tmpDir, "result.properties")

			if tc.userConfig != nil {
				userConfigFile, err := os.Create(userConfigPath)
				if err != nil {
					t.Fatalf("can't create user config file: %v", err)
				}

				_, err = tc.userConfig.Write(userConfigFile, properties.UTF8)
				if err != nil {
					t.Fatalf("can't write user config file: %v", err)
				}

				err = userConfigFile.Close()
				if err != nil {
					t.Fatalf("can't close user config file: %v", err)
				}
			}

			operatorConfigFile, err := os.Create(operatorConfigPath)
			if err != nil {
				t.Fatalf("can't create user config file: %v", err)
			}

			_, err = tc.operatorConfig.Write(operatorConfigFile, properties.UTF8)
			if err != nil {
				t.Fatalf("can't write user config file: %v", err)
			}

			err = operatorConfigFile.Close()
			if err != nil {
				t.Fatalf("can't close operator config file: %v", err)
			}

			err = mergeSnitchConfigs(userConfigPath, operatorConfigPath, resultConfigPath)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			gotConfig, err := properties.LoadFile(resultConfigPath, properties.UTF8)
			if err != nil {
				t.Fatalf("can't load result config: %v", err)
			}

			if !equality.Semantic.DeepEqual(tc.expectedConfig.Map(), gotConfig.Map()) {
				t.Fatalf("expected and got configs differ, diff: %q", cmp.Diff(tc.expectedConfig.Map(), gotConfig.Map()))
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

func TestParseScyllaArguments(t *testing.T) {
	ts := []struct {
		Name         string
		Args         string
		ExpectedArgs map[string]*string
	}{
		{
			Name: "all combinations in one",
			Args: `--arg1 --arg2=val2 --arg3 "val3" --arg4="val4" --arg5=-1.23 --arg6 -123 --arg7 123 --arg8="" --arg9 ""`,
			ExpectedArgs: map[string]*string{
				"arg1": nil,
				"arg2": pointer.Ptr("val2"),
				"arg3": pointer.Ptr(`"val3"`),
				"arg4": pointer.Ptr(`"val4"`),
				"arg5": pointer.Ptr("-1.23"),
				"arg6": pointer.Ptr("-123"),
				"arg7": pointer.Ptr("123"),
				"arg8": pointer.Ptr(`""`),
				"arg9": pointer.Ptr(`""`),
			},
		},
		{
			Name:         "single empty flag",
			Args:         "--arg",
			ExpectedArgs: map[string]*string{"arg": nil},
		},
		{
			Name:         "single flag",
			Args:         "--arg val",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("val")},
		},
		{
			Name:         "integer value",
			Args:         "--arg 123",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("123")},
		},
		{
			Name:         "negative integer value",
			Args:         "--arg -123",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("-123")},
		},
		{
			Name:         "float value",
			Args:         "--arg 1.23",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("1.23")},
		},
		{
			Name:         "negative float value",
			Args:         "--arg -1.23",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("-1.23")},
		},
		{
			Name: "negative float value and string parameter",
			Args: "--arg1 -1.23 --arg2 val",
			ExpectedArgs: map[string]*string{
				"arg1": pointer.Ptr("-1.23"),
				"arg2": pointer.Ptr("val"),
			},
		},
		{
			Name:         "bool value",
			Args:         "--arg true",
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr("true")},
		},
		{
			Name:         "flag quoted",
			Args:         `--arg "val"`,
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr(`"val"`)},
		},
		{
			Name:         "flag quoted with equal sign",
			Args:         `--arg="val"`,
			ExpectedArgs: map[string]*string{"arg": pointer.Ptr(`"val"`)},
		},
		{
			Name:         "empty",
			Args:         "",
			ExpectedArgs: map[string]*string{},
		},
		{
			Name: "garbage inside",
			Args: "--arg1 val asdasdasdas --arg2=val",
			ExpectedArgs: map[string]*string{
				"arg1": pointer.Ptr("val"),
				"arg2": pointer.Ptr("val"),
			},
		},
		{
			Name: "mixed types",
			Args: "--skip-wait-for-gossip-to-settle 0 --ring-delay-ms 5000 --compaction-enforce-min-threshold true --shadow-round-ms 1",
			ExpectedArgs: map[string]*string{
				"skip-wait-for-gossip-to-settle":   pointer.Ptr("0"),
				"ring-delay-ms":                    pointer.Ptr("5000"),
				"compaction-enforce-min-threshold": pointer.Ptr("true"),
				"shadow-round-ms":                  pointer.Ptr("1"),
			},
		},
	}

	for _, test := range ts {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			argumentsMap := helpers.ParseScyllaArguments(test.Args)
			if !reflect.DeepEqual(test.ExpectedArgs, argumentsMap) {
				t.Errorf("expected %+v, got %+v", test.ExpectedArgs, argumentsMap)
			}
		})
	}
}

func TestMergeArguments(t *testing.T) {
	t.Parallel()

	ts := []struct {
		name                     string
		operatorManagedArguments map[string]*string
		userProvidedArguments    map[string]*string
		expectedArguments        map[string]*string
	}{
		{
			name: "no conflicts",
			operatorManagedArguments: map[string]*string{
				"foo": pointer.Ptr("bar"),
			},
			userProvidedArguments: map[string]*string{
				"bar": pointer.Ptr("foo"),
			},
			expectedArguments: map[string]*string{
				"foo": pointer.Ptr("bar"),
				"bar": pointer.Ptr("foo"),
			},
		},
		{
			name: "operator wins on conflicts",
			operatorManagedArguments: map[string]*string{
				"foo": pointer.Ptr("bar"),
			},
			userProvidedArguments: map[string]*string{
				"foo": pointer.Ptr("buzz"),
			},
			expectedArguments: map[string]*string{
				"foo": pointer.Ptr("bar"),
			},
		},
	}
	for _, test := range ts {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gotArguments := mergeArguments(test.operatorManagedArguments, test.userProvidedArguments)
			if !reflect.DeepEqual(test.expectedArguments, gotArguments) {
				t.Errorf("expected %+v, got %+v", test.expectedArguments, gotArguments)
			}
		})
	}
}
