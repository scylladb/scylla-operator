package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	configDirScylla                     = "/etc/scylla"
	configDirScyllaD                    = "/etc/scylla.d"
	scyllaYAMLPath                      = configDirScylla + "/" + naming.ScyllaConfigName
	scyllaYAMLConfigMapPath             = naming.ScyllaConfigDirName + "/" + naming.ScyllaConfigName
	scyllaIOPropertiesPath              = configDirScyllaD + "/" + naming.ScyllaIOPropertiesName
	scyllaRackDCPropertiesPath          = configDirScylla + "/" + naming.ScyllaRackDCPropertiesName
	scyllaRackDCPropertiesConfigMapPath = naming.ScyllaConfigDirName + "/" + naming.ScyllaRackDCPropertiesName
	entrypointPath                      = "/docker-entrypoint.py"
)

type ScyllaConfig struct {
	member        *identity.Member
	kubeClient    kubernetes.Interface
	cpuCount      int
	externalSeeds []string
}

func NewScyllaConfig(m *identity.Member, kubeClient kubernetes.Interface, cpuCount int, externalSeeds []string) *ScyllaConfig {
	return &ScyllaConfig{
		member:        m,
		kubeClient:    kubeClient,
		cpuCount:      cpuCount,
		externalSeeds: externalSeeds,
	}
}

func (s *ScyllaConfig) Setup(ctx context.Context) (*exec.Cmd, error) {
	klog.Info("Setting up iotune cache")
	if err := setupIOTuneCache(); err != nil {
		return nil, fmt.Errorf("can't setup iotune cache: %w", err)
	}

	klog.Info("Setting up scylla.yaml")
	if err := s.setupScyllaYAML(scyllaYAMLPath, naming.ScyllaManagedConfigPath, scyllaYAMLConfigMapPath); err != nil {
		return nil, fmt.Errorf("can't setup scylla.yaml: %w", err)
	}

	klog.Info("Setting up cassandra-rackdc.properties")
	if err := mergeSnitchConfigs(scyllaRackDCPropertiesConfigMapPath, filepath.Join(naming.ScyllaDBSnitchConfigDir, naming.ScyllaRackDCPropertiesName), scyllaRackDCPropertiesPath); err != nil {
		return nil, fmt.Errorf("can't setup rackdc properties file: %w", err)
	}

	klog.Info("Setting up entrypoint script")
	cmd, err := s.setupEntrypoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't setup entrypoint: %w", err)
	}

	return cmd, nil
}

// setupScyllaYAML edits the default scylla.yaml file with our custom options.
// We only edit the options that are not available to configure via the
// entrypoint script flags. Those options are:
// - cluster_name
// - rpc_address
// - endpoint_snitch
func (s *ScyllaConfig) setupScyllaYAML(configFilePath, managedConfigMapPath, configMapPath string) error {
	// Read default scylla.yaml
	configFileBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("can't read file %q: %w", configFilePath, err)
	}

	operatorConfigOverrides, err := os.ReadFile(managedConfigMapPath)
	if err != nil {
		return fmt.Errorf("can't make scylladb config overrides: %w", err)
	}

	// Read config map scylla.yaml
	configMapBytes, err := os.ReadFile(configMapPath)
	if err != nil {
		klog.InfoS("no scylla.yaml config map available")
	}

	desiredConfigBytes, err := mergeYAMLs(configFileBytes, operatorConfigOverrides, configMapBytes)
	if err != nil {
		return fmt.Errorf("can't merge scylladb configs: %w", err)
	}

	// Write result to file
	err = os.WriteFile(configFilePath, desiredConfigBytes, os.ModePerm)
	if err != nil {
		return fmt.Errorf("can't write file %q: %w", configFilePath, err)
	}

	return nil
}

// Operator reconciles only three out of four possible settings in snitch config taking values from an API object.
// Users can change the snitch being used and provide their own configuration.
// The missing setting is taken from user provided config.
func mergeSnitchConfigs(userSnitchConfigPath string, operatorSnitchConfigPath string, scylladbSnitchConfigPath string) error {
	var userProperties, operatorProperties *properties.Properties
	_, err := os.Stat(userSnitchConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("can't stat %q: %w", userSnitchConfigPath, err)
	}
	if err == nil {
		userProperties, err = properties.LoadFile(userSnitchConfigPath, properties.UTF8)
		if err != nil {
			return fmt.Errorf("can't read user snitch config from %q: %w", userSnitchConfigPath, err)
		}
	}

	operatorProperties, err = properties.LoadFile(operatorSnitchConfigPath, properties.UTF8)
	if err != nil {
		return fmt.Errorf("can't read operator snitch config from %q: %w", operatorSnitchConfigPath, err)
	}

	mergedProperties, err := mergeSnitchConfigProperties(userProperties, operatorProperties)
	if err != nil {
		return fmt.Errorf("can't merge snitch configs: %w", err)
	}

	snitchConfigFile, err := os.Create(scylladbSnitchConfigPath)
	if err != nil {
		return fmt.Errorf("can't create snitch config file %q: %w", scylladbSnitchConfigPath, err)
	}

	_, err = mergedProperties.Write(snitchConfigFile, properties.UTF8)
	if err != nil {
		return fmt.Errorf("can't write snitch config file %q: %w", snitchConfigFile.Name(), err)
	}

	return nil
}

func mergeSnitchConfigProperties(userConfig, operatorConfig *properties.Properties) (*properties.Properties, error) {
	if operatorConfig == nil {
		return nil, fmt.Errorf("unexpected nil Operator snitch config")
	}

	const dcSuffixKey = "dc_suffix"

	if userConfig != nil {
		userDCSuffix := userConfig.GetString(dcSuffixKey, "")
		if len(userDCSuffix) != 0 {
			_, _, err := operatorConfig.Set(dcSuffixKey, userDCSuffix)
			if err != nil {
				return nil, fmt.Errorf("can't set %q key in Operator snitch config: %w", dcSuffixKey, err)
			}
		}
	}

	return operatorConfig, nil
}

var scyllaArgumentsRegexp = regexp.MustCompile(`--([^= ]+)(="[^"]+"|=\S+|\s+"[^"]+"|\s+[^\s-]+|\s+-?\d*\.?\d+[^\s-]+|)`)

func parseScyllaArguments(scyllaArguments string) map[string]*string {
	output := make(map[string]*string)
	for _, value := range scyllaArgumentsRegexp.FindAllStringSubmatch(scyllaArguments, -1) {
		if value[2] == "" {
			output[value[1]] = nil
		} else {
			output[value[1]] = pointer.Ptr(strings.TrimSpace(strings.TrimPrefix(value[2], "=")))
		}
	}
	return output
}

// mergeArguments merges two sets of arguments.
// Arguments managed by the operator take precedence and cannot be overridden by user-provided arguments.
func mergeArguments(operatorManagedArguments, userProvidedArguments map[string]*string) map[string]*string {
	finalArguments := maps.Clone(operatorManagedArguments)
	for argName, argValue := range userProvidedArguments {
		if _, ok := operatorManagedArguments[argName]; !ok {
			finalArguments[argName] = argValue
		} else {
			klog.Infof("ScyllaArgs: argument '%s' is ignored, it's managed by Operator", argName)
		}
	}
	return finalArguments
}

func (s *ScyllaConfig) setupEntrypoint(ctx context.Context) (*exec.Cmd, error) {
	m := s.member
	seeds, err := m.GetSeeds(ctx, s.kubeClient.CoreV1(), s.externalSeeds)
	if err != nil {
		return nil, fmt.Errorf("can't get seeds: %w", err)
	}

	overprovisioned := "0"
	if m.Overprovisioned {
		overprovisioned = "1"
	}

	cpusAllowed, err := getCPUsAllowedList("/proc/1/status")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.validateCpuSet(ctx, cpusAllowed, s.cpuCount); err != nil {
		return nil, errors.WithStack(err)
	}

	isBroadcastIPv6 := m.IPFamily == corev1.IPv6Protocol

	prometheusAddress := "0.0.0.0"
	if isBroadcastIPv6 {
		prometheusAddress = "::"
	}

	args := map[string]*string{
		"seeds":                 pointer.Ptr(strings.Join(seeds, ",")),
		"overprovisioned":       &overprovisioned,
		"smp":                   pointer.Ptr(strconv.Itoa(s.cpuCount)),
		"prometheus-address":    &prometheusAddress,
		"broadcast-address":     &m.BroadcastAddress,
		"broadcast-rpc-address": &m.BroadcastRPCAddress,
		"cpuset":                &cpusAllowed,
	}

	if !isBroadcastIPv6 {
		listenAddress := "0.0.0.0"
		args["listen-address"] = &listenAddress
	}

	if hostID, ok := m.ServiceLabels[naming.ReplacingNodeHostIDLabel]; ok {
		if len(hostID) == 0 {
			klog.Warningf("Service %q have unexpectedly empty label %q, skipping replace", m.Name, naming.ReplacingNodeHostIDLabel)
		} else {
			args["replace-node-first-boot"] = pointer.Ptr(hostID)
		}
	}

	if len(m.AdditionalScyllaDBArguments) > 0 {
		userArgs := parseScyllaArguments(strings.Join(m.AdditionalScyllaDBArguments, " "))

		if userRpcAddress, hasUserRpcAddress := userArgs["rpc-address"]; hasUserRpcAddress && userRpcAddress != nil {
			isUserRpcIPv6 := strings.Contains(*userRpcAddress, ":")

			if isBroadcastIPv6 != isUserRpcIPv6 {
				klog.Warningf("User-provided rpc-address '%s' IP family doesn't match broadcast-address '%s' IP family. Removing user's rpc-address for consistency.",
					*userRpcAddress, m.BroadcastAddress)
				delete(userArgs, "rpc-address")
			} else {
				klog.Infof("Using user-provided rpc-address: %s", *userRpcAddress)
			}
		}

		if userListenAddress, hasUserListenAddress := userArgs["listen-address"]; hasUserListenAddress && userListenAddress != nil {
			isUserListenIPv6 := strings.Contains(*userListenAddress, ":")

			if isBroadcastIPv6 != isUserListenIPv6 {
				klog.Warningf("User-provided listen-address '%s' IP family doesn't match broadcast-address '%s' IP family. Removing user's listen-address for consistency.",
					*userListenAddress, m.BroadcastAddress)
				delete(userArgs, "listen-address")
			} else {
				klog.Infof("Using user-provided listen-address: %s", *userListenAddress)
			}
		}

		args = mergeArguments(args, userArgs)
	}

	if isBroadcastIPv6 {
		if _, hasRpcAddress := args["rpc-address"]; !hasRpcAddress {
			args["rpc-address"] = pointer.Ptr("::")
			klog.Infof("Using default IPv6 rpc-address: ::")
		}

		if _, hasListenAddress := args["listen-address"]; !hasListenAddress {
			args["listen-address"] = pointer.Ptr("::")
			klog.Infof("Using default IPv6 listen-address: ::")
		}

		args["enable-ipv6-dns-lookup"] = pointer.Ptr("1")
		klog.Info("Enabling IPv6 DNS lookup due to cluster IPv6 IPFamily")
	}

	if _, err := os.Stat(scyllaIOPropertiesPath); err == nil {
		klog.InfoS("Scylla IO properties are already set, skipping io tuning")
		args["io-setup"] = pointer.Ptr("0")
		args["io-properties-file"] = pointer.Ptr(scyllaIOPropertiesPath)
	}

	var argsList []string
	for key, value := range args {
		if value == nil {
			argsList = append(argsList, fmt.Sprintf("--%s", key))
		} else {
			argsList = append(argsList, fmt.Sprintf("--%s=%s", key, *value))
		}
	}

	scyllaCmd := exec.Command(entrypointPath, argsList...)
	scyllaCmd.Stderr = os.Stderr
	scyllaCmd.Stdout = os.Stdout
	klog.InfoS("Scylla entrypoint", "Command", scyllaCmd)

	return scyllaCmd, nil
}

func (s *ScyllaConfig) validateCpuSet(ctx context.Context, cpusAllowed string, shards int) error {
	cpuSet, err := cpuset.Parse(cpusAllowed)
	if err != nil {
		return err
	}
	if cpuSet.Size() != shards {
		klog.InfoS("suboptimal shard and cpuset config, shard count (config: 'CPU') and cpuset size should match for optimal performance",
			"shards", shards, "cpuset", cpuSet.String())
	}
	return nil
}

func setupIOTuneCache() error {
	if _, err := os.Stat(scyllaIOPropertiesPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("can't stat file %q: %w", scyllaIOPropertiesPath, err)
		}

		cachePath := path.Join(naming.DataDir, naming.ScyllaIOPropertiesName)
		if err := os.Symlink(cachePath, scyllaIOPropertiesPath); err != nil {
			return fmt.Errorf("can't create symlink from %q to %q: %w", scyllaIOPropertiesPath, cachePath, err)
		}

		klog.V(2).Info("Initialized IOTune benchmark cache", "path", scyllaIOPropertiesPath, "cachePath", cachePath)
	} else {
		klog.V(2).Info("Found cached IOTune benchmark results", "path", scyllaIOPropertiesPath)
	}

	return nil
}

// mergeYAMLs merges arbitrary YAML structures on the top level keys.
func mergeYAMLs(initialYAML []byte, overrideYAMLs ...[]byte) ([]byte, error) {
	var result map[string]interface{}
	err := yaml.Unmarshal(initialYAML, &result)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal inititial yaml for overriding: %w", err)
	}

	if result == nil {
		result = map[string]interface{}{}
	}

	for i, overrideYAML := range overrideYAMLs {
		var override map[string]interface{}
		err := yaml.Unmarshal(overrideYAML, &override)
		if err != nil {
			return nil, fmt.Errorf("can't unmarshal override yaml (%d) for overriding: %w", i, err)
		}

		// Overwrite the values onto initial
		for k, v := range override {
			result[k] = v
		}
	}

	return yaml.Marshal(result)
}

func getCPUsAllowedList(procFile string) (string, error) {
	statusFile, err := ioutil.ReadFile(procFile)
	if err != nil {
		return "", errors.Wrapf(err, "error reading proc status file '%s'", procFile)
	}
	procStatus := string(statusFile[:])
	startIndex := strings.Index(procStatus, "Cpus_allowed_list:")

	if err != nil {
		return "", fmt.Errorf("failed to get process status: %s", err.Error())
	}
	endIndex := startIndex + strings.Index(procStatus[startIndex:], "\n")
	cpusAllowed := strings.TrimSpace(procStatus[startIndex+len("Cpus_allowed_list:") : endIndex])
	return cpusAllowed, nil
}
