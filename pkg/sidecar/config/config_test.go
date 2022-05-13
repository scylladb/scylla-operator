package config

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
)

func convertScyllaArguments(scyllaArguments string) map[string]string {
	output := make(map[string]string)
	for _, value := range scyllaArgumentsRegexp.FindAllStringSubmatch(scyllaArguments, -1) {
		if value[2] == "" {
			output[value[1]] = ""
		} else if value[2][0] == '=' {
			output[value[1]] = strings.TrimSpace(value[2][1:])
		} else {
			output[value[1]] = strings.TrimSpace(value[2])
		}
	}
	return output
}

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
	tests := []struct {
		initial     []byte
		override    []byte
		result      []byte
		expectedErr bool
	}{
		{
			[]byte("key: value"),
			[]byte("key: override_value"),
			[]byte("key: override_value\n"),
			false,
		},
		{
			[]byte("#comment"),
			[]byte("key: value"),
			[]byte("key: value\n"),
			false,
		},
		{
			[]byte("key: value"),
			[]byte("#comment"),
			[]byte("key: value\n"),
			false,
		},
		{
			[]byte("key1:\n  nestedkey1: nestedvalue1"),
			[]byte("key1:\n  nestedkey1: nestedvalue2"),
			[]byte("key1:\n  nestedkey1: nestedvalue2\n"),
			false,
		},
	}

	for _, test := range tests {
		result, err := mergeYAMLs(test.initial, test.override)
		if !bytes.Equal(result, test.result) {
			t.Errorf("Merge of '%s' and '%s' was incorrect,\n got: %s,\n want: %s.",
				test.initial, test.override, result, test.result)
		}
		if err == nil && test.expectedErr {
			t.Errorf("Expected error.")
		}
		if err != nil && !test.expectedErr {
			t.Logf("Got an error as expected: %s", err.Error())
		}
	}
}

func TestAllowedCPUs(t *testing.T) {
	cpusAllowed, err := getCPUsAllowedList("./procstatus")
	if err != nil {
		t.Error(err)
	}
	t.Log(cpusAllowed)
}

func TestScyllaYamlMerging(t *testing.T) {
	tests := []struct {
		DefaultContent    string
		UserConfigContent string
		ExpectedResult    string
	}{
		{
			DefaultContent:    "",
			UserConfigContent: "",
			ExpectedResult:    "cluster_name: cluster-name\nendpoint_snitch: GossipingPropertyFileSnitch\nrpc_address: 0.0.0.0\n",
		},
		{
			DefaultContent:    "some_key: 1",
			UserConfigContent: "",
			ExpectedResult:    "cluster_name: cluster-name\nendpoint_snitch: GossipingPropertyFileSnitch\nrpc_address: 0.0.0.0\nsome_key: 1\n",
		},
		{
			DefaultContent:    "cluster_name: default-name",
			UserConfigContent: "",
			ExpectedResult:    "cluster_name: cluster-name\nendpoint_snitch: GossipingPropertyFileSnitch\nrpc_address: 0.0.0.0\n",
		},
		{
			DefaultContent:    "some_key: 1",
			UserConfigContent: "cluster_name: different_name",
			ExpectedResult:    "cluster_name: different_name\nendpoint_snitch: GossipingPropertyFileSnitch\nrpc_address: 0.0.0.0\nsome_key: 1\n",
		},
		{
			DefaultContent:    "some_key: 1",
			UserConfigContent: "some_key: 2",
			ExpectedResult:    "cluster_name: cluster-name\nendpoint_snitch: GossipingPropertyFileSnitch\nrpc_address: 0.0.0.0\nsome_key: 2\n",
		},
	}

	for _, test := range tests {
		scyllaYamlPath := writeTempFile(t, "scylla-yaml", test.DefaultContent)
		defer os.Remove(scyllaYamlPath)
		configMapYamlPath := writeTempFile(t, "config-map", test.UserConfigContent)
		defer os.Remove(configMapYamlPath)

		sc := &ScyllaConfig{member: &identity.Member{Cluster: "cluster-name"}}
		if err := sc.SetupScyllaYAML(); err != nil {
			t.Error(err)
		}

		resultContent, err := ioutil.ReadFile(scyllaYamlPath)
		if err != nil {
			t.Error(err)
		}

		if string(resultContent) != test.ExpectedResult {
			t.Error(cmp.Diff(test.ExpectedResult, string(resultContent)))
		}
	}
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

	for i := range ts {
		test := ts[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			argumentsMap := convertScyllaArguments(test.Args)
			if !reflect.DeepEqual(test.ExpectedArgs, argumentsMap) {
				t.Errorf("expected %+v, got %+v", test.ExpectedArgs, argumentsMap)
			}
		})
	}
}
