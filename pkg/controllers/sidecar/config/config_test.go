package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/slices"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	require.Equal(t, err, nil)
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
		if err := sc.setupScyllaYAML(scyllaYamlPath, configMapYamlPath); err != nil {
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

func TestScyllaArgumentsAllTypesInOne(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1 --arg2 val --arg3=val --arg4 --arg5 \"val\" --arg6=\"val\" --arg7")
	require.Equal(t, "", argumentsMap["arg1"])
	require.Equal(t, "val", argumentsMap["arg2"])
	require.Equal(t, "val", argumentsMap["arg3"])
	require.Equal(t, "", argumentsMap["arg4"])
	require.Equal(t, "\"val\"", argumentsMap["arg5"])
	require.Equal(t, "\"val\"", argumentsMap["arg6"])
	require.Equal(t, "", argumentsMap["arg7"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 7, len(argumentsMap))
}

func TestScyllaArgumentsAllTypesInOneWithGarbage(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1 --arg2 val --arg3=val asdasdasdas --arg4 --arg5 \"val\" --arg6=\"val\" --arg7")
	require.Equal(t, "", argumentsMap["arg1"])
	require.Equal(t, "val", argumentsMap["arg2"])
	require.Equal(t, "val", argumentsMap["arg3"])
	require.Equal(t, "", argumentsMap["arg4"])
	require.Equal(t, "\"val\"", argumentsMap["arg5"])
	require.Equal(t, "\"val\"", argumentsMap["arg6"])
	require.Equal(t, "", argumentsMap["arg7"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 7, len(argumentsMap))
}

func TestScyllaArgumentsSingleFlag(t *testing.T) {
	t.Log("Single flag - test started")
	argumentsMap := convertScyllaArguments("--arg1")
	require.Equal(t, "", argumentsMap["arg1"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 1, len(argumentsMap))
}

func TestScyllaArgumentsSingleArgument(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1 val")
	require.Equal(t, "val", argumentsMap["arg1"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 1, len(argumentsMap))
}

func TestScyllaArgumentsSingleArgumemt2(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1=val")
	require.Equal(t, "val", argumentsMap["arg1"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 1, len(argumentsMap))
}

func TestScyllaArgumentsSingleArgumemtQuoted(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1 \"val\"")
	require.Equal(t, "\"val\"", argumentsMap["arg1"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 1, len(argumentsMap))
}

func TestScyllaArgumentsSingleArgumemt2Quoted(t *testing.T) {
	argumentsMap := convertScyllaArguments("--arg1=\"val\"")
	require.Equal(t, "\"val\"", argumentsMap["arg1"])
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 1, len(argumentsMap))
}

func TestScyllaArgumentsEmpty(t *testing.T) {
	argumentsMap := convertScyllaArguments("")
	require.Equal(t, "", argumentsMap["not_existing_key"])
	require.Equal(t, 0, len(argumentsMap))
}

func TestReplaceNodeLabelInMemberService(t *testing.T) {
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})
	if err := scyllav1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	replaceAddr := "1.2.3.4"
	options.GetSidecarOptions().CPU = "1"

	m := &identity.Member{
		Namespace: "namespace",
		Cluster:   "cluster",
		ServiceLabels: map[string]string{
			naming.ReplaceLabel: replaceAddr,
		},
	}

	fakeSeedService := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: m.Namespace,
			Labels: map[string]string{
				naming.SeedLabel:        "",
				naming.ClusterNameLabel: m.Cluster,
			},
		},
	}
	fakeCluster := &scyllav1.ScyllaCluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      m.Cluster,
			Namespace: m.Namespace,
		},
	}
	clientFake := fake.NewFakeClientWithScheme(scheme.Scheme, fakeCluster)
	kubeClientFake := kubefake.NewSimpleClientset(fakeSeedService)

	cfg := NewForMember(m, kubeClientFake, clientFake, logger)

	cmd, err := cfg.setupEntrypoint(context.Background())
	if err != nil {
		t.Errorf("entrypoint setup, err: %s", err)
	}

	expectedArg := fmt.Sprintf("--replace-address-first-boot=%s", replaceAddr)

	if !slices.ContainsString(expectedArg, cmd.Args) {
		t.Errorf("missing Scylla parameter %s", expectedArg)
	}

}
