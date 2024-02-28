package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	supervisordConfDir                  = "/etc/supervisord.conf.d"
)

var scyllaJMXPaths = []string{"/usr/lib/scylla/jmx/scylla-jmx", "/opt/scylladb/jmx/scylla-jmx"}

type ScyllaConfig struct {
	member                              *identity.Member
	kubeClient                          kubernetes.Interface
	scyllaClient                        scyllaversionedclient.Interface
	scyllaRackDCPropertiesPath          string
	scyllaRackDCPropertiesConfigMapPath string
	cpuCount                            int
	externalSeeds                       []string
}

func NewScyllaConfig(m *identity.Member, kubeClient kubernetes.Interface, scyllaClient scyllaversionedclient.Interface, cpuCount int, externalSeeds []string) *ScyllaConfig {
	return &ScyllaConfig{
		member:                              m,
		kubeClient:                          kubeClient,
		scyllaClient:                        scyllaClient,
		scyllaRackDCPropertiesPath:          scyllaRackDCPropertiesPath,
		scyllaRackDCPropertiesConfigMapPath: scyllaRackDCPropertiesConfigMapPath,
		cpuCount:                            cpuCount,
		externalSeeds:                       externalSeeds,
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
	if err := s.setupRackDCProperties(); err != nil {
		return nil, fmt.Errorf("can't setup rackdc properties file: %w", err)
	}

	klog.Info("Setting up entrypoint script")
	cmd, err := s.setupEntrypoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't setup entrypoint: %w", err)
	}

	klog.Info("Disabling undesirable supervisord services")
	// Make sure the directory is present, individual files may be missing between versions.
	_, err = os.Stat(supervisordConfDir)
	if err != nil {
		return nil, fmt.Errorf("can't stat supervisord config directory %q: %w", supervisordConfDir, err)
	}
	for _, serviceConfigName := range []string{
		"scylla-jmx.conf",
	} {
		p := filepath.Join(supervisordConfDir, serviceConfigName)
		err = os.Remove(p)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("can't remove service config %q: %w", p, err)
		}
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
			scyllaFinalArgs[argName] = pointer.Ptr(strings.TrimSpace(argValue))
		} else {
			klog.Infof("ScyllaArgs: argument '%s' is ignored, it is already in the list", argName)
		}
	}
}

func (s *ScyllaConfig) setupEntrypoint(ctx context.Context) (*exec.Cmd, error) {
	m := s.member
	seeds, err := m.GetSeeds(ctx, s.kubeClient.CoreV1(), s.externalSeeds)
	if err != nil {
		return nil, fmt.Errorf("can't get seeds: %w", err)
	}

	// Check if we need to run in developer mode
	devMode := "0"
	cluster, err := s.scyllaClient.ScyllaV1().ScyllaClusters(s.member.Namespace).Get(ctx, s.member.Cluster, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
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
		"seeds":                 pointer.Ptr(strings.Join(seeds, ",")),
		"developer-mode":        &devMode,
		"overprovisioned":       &overprovisioned,
		"smp":                   pointer.Ptr(strconv.Itoa(s.cpuCount)),
		"prometheus-address":    &prometheusAddress,
		"broadcast-address":     &m.BroadcastAddress,
		"broadcast-rpc-address": &m.BroadcastRPCAddress,
	}

	// If node is being replaced
	if addr, ok := m.ServiceLabels[naming.ReplaceLabel]; ok {
		if len(addr) == 0 {
			klog.Warningf("Service %q have unexpectedly empty label %q, skipping replace", m.Name, naming.ReplaceLabel)
		} else {
			args["replace-address-first-boot"] = pointer.Ptr(addr)
		}
	}
	if hostID, ok := m.ServiceLabels[naming.ReplacingNodeHostIDLabel]; ok {
		if len(hostID) == 0 {
			klog.Warningf("Service %q have unexpectedly empty label %q, skipping replace", m.Name, naming.ReplacingNodeHostIDLabel)
		} else {
			args["replace-node-first-boot"] = pointer.Ptr(hostID)
		}
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
		if err := s.validateCpuSet(ctx, cpusAllowed, s.cpuCount); err != nil {
			return nil, errors.WithStack(err)
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
