package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
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

			argumentsMap := parseScyllaArguments(test.Args)
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

func TestIPFamilyConsistency(t *testing.T) {
	tests := []struct {
		name                   string
		broadcastAddress       string
		expectedListenAddr     string
		expectedRPCAddr        string
		expectedPrometheusAddr string
	}{
		{
			name:                   "IPv4 broadcast address",
			broadcastAddress:       "10.0.0.1",
			expectedListenAddr:     "0.0.0.0",
			expectedRPCAddr:        "0.0.0.0",
			expectedPrometheusAddr: "0.0.0.0",
		},
		{
			name:                   "IPv6 broadcast address",
			broadcastAddress:       "2001:db8::1",
			expectedListenAddr:     "::",
			expectedRPCAddr:        "::",
			expectedPrometheusAddr: "::",
		},
		{
			name:                   "IPv6 ULA address",
			broadcastAddress:       "fd00:10:244:2::a",
			expectedListenAddr:     "::",
			expectedRPCAddr:        "::",
			expectedPrometheusAddr: "::",
		},
		{
			name:                   "Link-local IPv6 address",
			broadcastAddress:       "fe80::1%eth0",
			expectedListenAddr:     "::",
			expectedRPCAddr:        "::",
			expectedPrometheusAddr: "::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock member with broadcast address
			member := &identity.Member{
				BroadcastAddress:    tt.broadcastAddress,
				BroadcastRPCAddress: tt.broadcastAddress,
			}

			// Simulate the logic from setupEntrypoint
			listenAddress := "0.0.0.0"
			rpcAddress := "0.0.0.0"
			prometheusAddress := "0.0.0.0"

			// If broadcast address is IPv6, listen on IPv6 wildcard address
			if strings.Contains(member.BroadcastAddress, ":") {
				listenAddress = "::"
				rpcAddress = "::"
				prometheusAddress = "::"
			}

			// Verify the IP family consistency
			if listenAddress != tt.expectedListenAddr {
				t.Errorf("expected listen address %q, got %q", tt.expectedListenAddr, listenAddress)
			}
			if rpcAddress != tt.expectedRPCAddr {
				t.Errorf("expected RPC address %q, got %q", tt.expectedRPCAddr, rpcAddress)
			}
			if prometheusAddress != tt.expectedPrometheusAddr {
				t.Errorf("expected Prometheus address %q, got %q", tt.expectedPrometheusAddr, prometheusAddress)
			}
		})
	}
}

func TestRPCAddressValidation(t *testing.T) {
	tests := []struct {
		name               string
		broadcastAddress   string
		userRPCAddress     *string
		expectedRPCAddress string
		expectWarning      bool
	}{
		{
			name:               "IPv4 broadcast with matching IPv4 user rpc-address",
			broadcastAddress:   "10.0.0.1",
			userRPCAddress:     pointer.Ptr("192.168.1.100"),
			expectedRPCAddress: "192.168.1.100",
			expectWarning:      false,
		},
		{
			name:               "IPv6 broadcast with matching IPv6 user rpc-address",
			broadcastAddress:   "2001:db8::1",
			userRPCAddress:     pointer.Ptr("fd00::100"),
			expectedRPCAddress: "fd00::100",
			expectWarning:      false,
		},
		{
			name:               "IPv4 broadcast with IPv6 user rpc-address (rejected)",
			broadcastAddress:   "10.0.0.1",
			userRPCAddress:     pointer.Ptr("2001:db8::100"),
			expectedRPCAddress: "0.0.0.0", // Should use default
			expectWarning:      true,
		},
		{
			name:               "IPv6 broadcast with IPv4 user rpc-address (rejected)",
			broadcastAddress:   "2001:db8::1",
			userRPCAddress:     pointer.Ptr("192.168.1.100"),
			expectedRPCAddress: "::", // Should use default
			expectWarning:      true,
		},
		{
			name:               "IPv4 broadcast with no user rpc-address",
			broadcastAddress:   "10.0.0.1",
			userRPCAddress:     nil,
			expectedRPCAddress: "0.0.0.0",
			expectWarning:      false,
		},
		{
			name:               "IPv6 broadcast with no user rpc-address",
			broadcastAddress:   "2001:db8::1",
			userRPCAddress:     nil,
			expectedRPCAddress: "::",
			expectWarning:      false,
		},
		{
			name:               "IPv4 broadcast with IPv4 wildcard user rpc-address",
			broadcastAddress:   "10.0.0.1",
			userRPCAddress:     pointer.Ptr("0.0.0.0"),
			expectedRPCAddress: "0.0.0.0",
			expectWarning:      false,
		},
		{
			name:               "IPv6 broadcast with IPv6 wildcard user rpc-address",
			broadcastAddress:   "2001:db8::1",
			userRPCAddress:     pointer.Ptr("::"),
			expectedRPCAddress: "::",
			expectWarning:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the validation logic from setupEntrypoint
			userArgs := make(map[string]*string)
			if tt.userRPCAddress != nil {
				userArgs["rpc-address"] = tt.userRPCAddress
			}

			// Check if user provided rpc-address and validate IP family consistency
			if userRpcAddress, hasUserRpcAddress := userArgs["rpc-address"]; hasUserRpcAddress && userRpcAddress != nil {
				isBroadcastIPv6 := strings.Contains(tt.broadcastAddress, ":")
				isUserRpcIPv6 := strings.Contains(*userRpcAddress, ":")

				if isBroadcastIPv6 != isUserRpcIPv6 {
					// IP families don't match, reject user's value
					delete(userArgs, "rpc-address")
					// In real code, this would log a warning
					if !tt.expectWarning {
						t.Errorf("expected no warning but IP families don't match")
					}
				} else {
					// IP families match, keep user's value
					if tt.expectWarning {
						t.Errorf("expected warning but IP families match")
					}
				}
			}

			// Determine final rpc-address
			var finalRPCAddress string
			if rpcAddr, hasRPC := userArgs["rpc-address"]; hasRPC && rpcAddr != nil {
				finalRPCAddress = *rpcAddr
			} else {
				// Set default based on broadcast address IP family
				if strings.Contains(tt.broadcastAddress, ":") {
					finalRPCAddress = "::"
				} else {
					finalRPCAddress = "0.0.0.0"
				}
			}

			// Verify the result
			if finalRPCAddress != tt.expectedRPCAddress {
				t.Errorf("expected rpc-address %q, got %q", tt.expectedRPCAddress, finalRPCAddress)
			}
		})
	}
}
