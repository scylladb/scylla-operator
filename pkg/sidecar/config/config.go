package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
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
	member                              *identity.Member
	kubeClient                          kubernetes.Interface
	scyllaClient                        scyllaversionedclient.Interface
	scyllaRackDCPropertiesPath          string
	scyllaRackDCPropertiesConfigMapPath string
	cpuCount                            int
}

func NewScyllaConfig(m *identity.Member, kubeClient kubernetes.Interface, scyllaClient scyllaversionedclient.Interface, cpuCount int) *ScyllaConfig {
	return &ScyllaConfig{
		member:                              m,
		kubeClient:                          kubeClient,
		scyllaClient:                        scyllaClient,
		scyllaRackDCPropertiesPath:          scyllaRackDCPropertiesPath,
		scyllaRackDCPropertiesConfigMapPath: scyllaRackDCPropertiesConfigMapPath,
		cpuCount:                            cpuCount,
	}
}

func (s *ScyllaConfig) Setup(ctx context.Context) ([]string, []string, error) {
	klog.Info("Setting up iotune cache")
	if err := setupIOTuneCache(); err != nil {
		return nil, nil, fmt.Errorf("can't setup iotune cache: %w", err)
	}

	klog.Info("Setting up scylla.yaml")
	if err := s.setupScyllaYAML(scyllaYAMLPath, scyllaYAMLConfigMapPath); err != nil {
		return nil, nil, fmt.Errorf("can't setup scylla.yaml: %w", err)
	}

	klog.Info("Setting up cassandra-rackdc.properties")
	if err := s.setupRackDCProperties(); err != nil {
		return nil, nil, fmt.Errorf("can't setup rackdc properties file: %w", err)
	}

	klog.Info("Generating Scylla arguments")
	return s.generateCommand(ctx)
}

// setupScyllaYAML edits the default scylla.yaml file with our custom options.
// We only edit the options that are not available to configure via the
// entrypoint script flags. Those options are:
// - cluster_name
// - rpc_address
// - endpoint_snitch
func (s *ScyllaConfig) setupScyllaYAML(configFilePath, configMapPath string) error {
	// Read default scylla.yaml
	configFileBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to open scylla.yaml")
	}

	// Read config map scylla.yaml
	configMapBytes, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		klog.InfoS("no scylla.yaml config map available")
	}
	// Custom options
	var cfg = make(map[string]interface{})
	m := s.member
	cfg["cluster_name"] = m.Cluster
	cfg["rpc_address"] = "0.0.0.0"
	cfg["endpoint_snitch"] = "GossipingPropertyFileSnitch"

	overrideBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to parse override options for scylla.yaml")
	}
	overwrittenBytes, err := mergeYAMLs(configFileBytes, overrideBytes)
	if err != nil {
		return errors.Wrap(err, "failed to merge scylla yaml with operator pre-sets")
	}

	customConfigBytesBytes, err := mergeYAMLs(overwrittenBytes, configMapBytes)
	if err != nil {
		return errors.Wrap(err, "failed to merge overwritten scylla yaml with user config map")
	}

	// Write result to file
	if err = ioutil.WriteFile(configFilePath, customConfigBytesBytes, os.ModePerm); err != nil {
		return errors.Wrap(err, "error trying to write scylla.yaml")
	}

	return nil
}

func (s *ScyllaConfig) setupRackDCProperties() error {
	suppliedProperties := loadProperties(s.scyllaRackDCPropertiesConfigMapPath)
	rackDCProperties := createRackDCProperties(suppliedProperties, s.member.Datacenter, s.member.Rack)
	f, err := os.Create(s.scyllaRackDCPropertiesPath)
	if err != nil {
		return errors.Wrap(err, "error trying to create cassandra-rackdc.properties")
	}
	if _, err := rackDCProperties.Write(f, properties.UTF8); err != nil {
		return errors.Wrap(err, "error trying to write cassandra-rackdc.properties")
	}
	return nil
}

func createRackDCProperties(suppliedProperties *properties.Properties, dc, rack string) *properties.Properties {
	suppliedProperties.DisableExpansion = true
	rackDCProperties := properties.NewProperties()
	rackDCProperties.DisableExpansion = true
	rackDCProperties.Set("dc", dc)
	rackDCProperties.Set("rack", rack)
	rackDCProperties.Set("prefer_local", suppliedProperties.GetString("prefer_local", "false"))
	if dcSuffix, ok := suppliedProperties.Get("dc_suffix"); ok {
		rackDCProperties.Set("dc_suffix", dcSuffix)
	}
	return rackDCProperties
}

func loadProperties(fileName string) *properties.Properties {
	l := &properties.Loader{Encoding: properties.UTF8}
	p, err := l.LoadFile(scyllaRackDCPropertiesConfigMapPath)
	if err != nil {
		klog.InfoS("unable to read properties", "file", fileName)
		return properties.NewProperties()
	}
	return p
}

var scyllaArgumentsRegexp = regexp.MustCompile(`--([^= ]+)(="[^"]+"|=\S+|\s+"[^"]+"|\s+[^\s-]+|\s+-?\d*\.?\d+[^\s-]+|)`)

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

func appendScyllaArguments(ctx context.Context, s *ScyllaConfig, scyllaArgs string, scyllaFinalArgs map[string]*string) {
	for argName, argValue := range convertScyllaArguments(scyllaArgs) {
		if existing := scyllaFinalArgs[argName]; existing == nil {
			scyllaFinalArgs[argName] = pointer.StringPtr(strings.TrimSpace(argValue))
		} else {
			klog.Infof("ScyllaArgs: argument '%s' is ignored, it is already in the list", argName)
		}
	}
}

func (s *ScyllaConfig) generateCommand(ctx context.Context) ([]string, []string, error) {
	m := s.member
	// Get seeds
	seed, err := m.GetSeed(ctx, s.kubeClient.CoreV1())
	if err != nil {
		return nil, nil, errors.Wrap(err, "error getting seeds")
	}

	// Check if we need to run in developer mode
	devMode := "0"
	cluster, err := s.scyllaClient.ScyllaV1().ScyllaClusters(s.member.Namespace).Get(ctx, s.member.Cluster, metav1.GetOptions{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "error getting cluster")
	}
	if cluster.Spec.DeveloperMode {
		devMode = "1"
	}

	overprovisioned := "0"
	if m.Overprovisioned {
		overprovisioned = "1"
	}

	// Listen on all interfaces so users or a service mesh can use localhost.
	listenAddress := "0.0.0.0"
	prometheusAddress := "0.0.0.0"
	args := map[string]*string{
		"listen-address":        &listenAddress,
		"broadcast-address":     &m.StaticIP,
		"broadcast-rpc-address": &m.StaticIP,
		"seeds":                 pointer.StringPtr(seed),
		"developer-mode":        &devMode,
		"overprovisioned":       &overprovisioned,
		"smp":                   pointer.StringPtr(strconv.Itoa(s.cpuCount)),
		"prometheus-address":    &prometheusAddress,
	}
	if cluster.Spec.Alternator.Enabled() {
		args["alternator-port"] = pointer.StringPtr(strconv.Itoa(int(cluster.Spec.Alternator.Port)))
		if cluster.Spec.Alternator.WriteIsolation != "" {
			args["alternator-write-isolation"] = pointer.StringPtr(cluster.Spec.Alternator.WriteIsolation)
		}
	}
	// If node is being replaced
	if addr, ok := m.ServiceLabels[naming.ReplaceLabel]; ok {
		args["replace-address-first-boot"] = pointer.StringPtr(addr)
	}
	// See if we need to use cpu-pinning
	// TODO: Add more checks to make sure this is valid.
	// eg. parse the cpuset and check the number of cpus is the same as cpu limits
	// Now we rely completely on the user to have the cpu policy correctly
	// configured in the kubelet, otherwise scylla will crash.
	if cluster.Spec.CpuSet {
		cpusAllowed, err := getCPUsAllowedList("/proc/1/status")
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if err := s.validateCpuSet(ctx, cpusAllowed, s.cpuCount); err != nil {
			return nil, nil, errors.WithStack(err)
		}
		args["cpuset"] = &cpusAllowed
	}

	version := semver.NewScyllaVersion(cluster.Spec.Version)

	klog.InfoS("Scylla version detected", "version", version)

	if len(cluster.Spec.ScyllaArgs) > 0 {
		if !version.SupportFeatureUnsafe(semver.ScyllaVersionThatSupportsArgs) {
			klog.InfoS("This scylla version does not support ScyllaArgs. ScyllaArgs is ignored", "version", cluster.Spec.Version)
		} else {
			appendScyllaArguments(ctx, s, cluster.Spec.ScyllaArgs, args)
		}
	}

	if _, err := os.Stat(scyllaIOPropertiesPath); err == nil && version.SupportFeatureSafe(semver.ScyllaVersionThatSupportsDisablingIOTuning) {
		klog.InfoS("Scylla IO properties are already set, skipping io tuning")
		ioSetup := "0"
		args["io-setup"] = &ioSetup
		args["io-properties-file"] = pointer.StringPtr(scyllaIOPropertiesPath)
	}

	command := []string{
		"/usr/bin/bash",
		"-euExo",
		"pipefail",
		"-c",
		`
python3 - $@ << EOF
import os
import sys
import scyllasetup
import logging
import commandlineparser

logging.basicConfig(stream=sys.stdout, level=logging.DEBUG, format="%(message)s")

try:
	arguments, extra_arguments = commandlineparser.parse()
	setup = scyllasetup.ScyllaSetup(arguments, extra_arguments=extra_arguments)
	setup.developerMode()
	setup.cpuSet()
	setup.io()
	setup.cqlshrc()
	setup.arguments()
except Exception:
	logging.exception('failed!')
EOF

/opt/scylladb/supervisor/scylla-server.sh
`,
	}
	for key, value := range args {
		if value == nil {
			command = append(command, fmt.Sprintf("--%s", key))
		} else {
			command = append(command, fmt.Sprintf("--%s=%s", key, *value))
		}
	}

	return command, os.Environ(), nil
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

// mergeYAMLs merges two arbitrary YAML structures at the top level.
func mergeYAMLs(initialYAML, overrideYAML []byte) ([]byte, error) {

	var initial, override map[string]interface{}
	if err := yaml.Unmarshal(initialYAML, &initial); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := yaml.Unmarshal(overrideYAML, &override); err != nil {
		return nil, errors.WithStack(err)
	}

	if initial == nil {
		initial = make(map[string]interface{})
	}
	// Overwrite the values onto initial
	for k, v := range override {
		initial[k] = v
	}
	return yaml.Marshal(initial)
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
