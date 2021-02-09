package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"k8s.io/utils/pointer"

	"github.com/ghodss/yaml"
	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configDirScylla                     = "/etc/scylla"
	scyllaYAMLPath                      = configDirScylla + "/" + naming.ScyllaConfigName
	scyllaYAMLConfigMapPath             = naming.ScyllaConfigDirName + "/" + naming.ScyllaConfigName
	scyllaRackDCPropertiesPath          = configDirScylla + "/" + naming.ScyllaRackDCPropertiesName
	scyllaRackDCPropertiesConfigMapPath = naming.ScyllaConfigDirName + "/" + naming.ScyllaRackDCPropertiesName
	entrypointPath                      = "/docker-entrypoint.py"
)

var scyllaJMXPaths = []string{"/usr/lib/scylla/jmx/scylla-jmx", "/opt/scylladb/jmx/scylla-jmx"}

type ScyllaConfig struct {
	client.Client
	member                              *identity.Member
	kubeClient                          kubernetes.Interface
	scyllaRackDCPropertiesPath          string
	scyllaRackDCPropertiesConfigMapPath string
	logger                              log.Logger
}

func NewForMember(m *identity.Member, kubeClient kubernetes.Interface, client client.Client, l log.Logger) *ScyllaConfig {
	return &ScyllaConfig{
		member:                              m,
		kubeClient:                          kubeClient,
		Client:                              client,
		scyllaRackDCPropertiesPath:          scyllaRackDCPropertiesPath,
		scyllaRackDCPropertiesConfigMapPath: scyllaRackDCPropertiesConfigMapPath,
		logger:                              l,
	}
}

func (s *ScyllaConfig) Setup(ctx context.Context) (*exec.Cmd, error) {
	var err error
	var cmd *exec.Cmd

	s.logger.Info(ctx, "Setting up scylla.yaml")
	if err = s.setupScyllaYAML(scyllaYAMLPath, scyllaYAMLConfigMapPath); err != nil {
		return nil, errors.Wrap(err, "failed to setup scylla.yaml")
	}
	s.logger.Info(ctx, "Setting up cassandra-rackdc.properties")
	if err = s.setupRackDCProperties(); err != nil {
		return nil, errors.Wrap(err, "failed to setup rackdc properties file")
	}
	s.logger.Info(ctx, "Setting up entrypoint script")
	if cmd, err = s.setupEntrypoint(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup entrypoint")
	}

	return cmd, nil
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
		s.logger.Info(context.Background(), "no scylla.yaml config map available")
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
	suppliedProperties := loadProperties(s.scyllaRackDCPropertiesConfigMapPath, s.logger)
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

func loadProperties(fileName string, logger log.Logger) *properties.Properties {
	l := &properties.Loader{Encoding: properties.UTF8}
	p, err := l.LoadFile(scyllaRackDCPropertiesConfigMapPath)
	if err != nil {
		logger.Info(context.Background(), "unable to read properties", "file", fileName)
		return properties.NewProperties()
	}
	return p
}

var scyllaArgumentsRegexp = regexp.MustCompile(`--([^= ]+)(="[^"]+"|=[^ ]+|[ \t]+"[^"]+"|[ \t]+[^-][^-]?[^ ]*|)`)

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
			s.logger.Info(ctx, fmt.Sprintf("ScyllaArgs: argument '%s' is ignored, it is already in the list", argName))
		}
	}
}

func (s *ScyllaConfig) setupEntrypoint(ctx context.Context) (*exec.Cmd, error) {
	m := s.member
	// Get seeds
	seeds, err := m.GetSeeds(ctx, s.kubeClient)
	if err != nil {
		return nil, errors.Wrap(err, "error getting seeds")
	}

	// Check if we need to run in developer mode
	devMode := "0"
	cluster := &v1.ScyllaCluster{}
	err = s.Get(ctx, naming.NamespacedName(s.member.Cluster, s.member.Namespace), cluster)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}
	if cluster.Spec.DeveloperMode {
		devMode = "1"
	}
	shards, err := strconv.Atoi(options.GetSidecarOptions().CPU)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	args := map[string]*string{
		"listen-address":        &m.IP,
		"broadcast-address":     &m.StaticIP,
		"broadcast-rpc-address": &m.StaticIP,
		"seeds":                 pointer.StringPtr(strings.Join(seeds, ",")),
		"developer-mode":        &devMode,
		"overprovisioned":       pointer.StringPtr("0"),
		"smp":                   pointer.StringPtr(strconv.Itoa(shards)),
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
			return nil, errors.WithStack(err)
		}
		if err := s.validateCpuSet(ctx, cpusAllowed, shards); err != nil {
			return nil, errors.WithStack(err)
		}
		args["cpuset"] = &cpusAllowed
	}

	if len(cluster.Spec.ScyllaArgs) > 0 {
		version, err := semver.Parse(cluster.Spec.Version)
		if err != nil {
			s.logger.Info(ctx, "This scylla version might not support ScyllaArgs", "version", cluster.Spec.Version)
			appendScyllaArguments(ctx, s, cluster.Spec.ScyllaArgs, args)
		} else if version.LT(v1.ScyllaVersionThatSupportsArgs) {
			s.logger.Info(ctx, "This scylla version does not support ScyllaArgs. ScyllaArgs is ignored", "version", cluster.Spec.Version)
		} else {
			appendScyllaArguments(ctx, s, cluster.Spec.ScyllaArgs, args)
		}
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
	s.logger.Info(ctx, "Scylla entrypoint", "command", scyllaCmd)

	return scyllaCmd, nil
}

func (s *ScyllaConfig) validateCpuSet(ctx context.Context, cpusAllowed string, shards int) error {
	cpuSet, err := cpuset.Parse(cpusAllowed)
	if err != nil {
		return err
	}
	if cpuSet.Size() != shards {
		s.logger.Info(ctx, "suboptimal shard and cpuset config, shard count (config: 'CPU') and cpuset size should match for optimal performance",
			"shards", shards, "cpuset", cpuSet.String())
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
