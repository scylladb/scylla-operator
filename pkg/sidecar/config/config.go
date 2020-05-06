package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
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
	if err = s.setupScyllaYAML(); err != nil {
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
func (s *ScyllaConfig) setupScyllaYAML() error {
	// Read default scylla.yaml
	scyllaYAMLBytes, err := ioutil.ReadFile(scyllaYAMLPath)
	if err != nil {
		return errors.Wrap(err, "failed to open scylla.yaml")
	}

	// Read config map scylla.yaml
	scyllaYAMLConfigMapBytes, err := ioutil.ReadFile(scyllaYAMLConfigMapPath)
	if err != nil {
		s.logger.Info(context.Background(), "no scylla.yaml config map available")
	}
	// Custom options
	var cfg = make(map[string]interface{})
	m := s.member
	cfg["cluster_name"] = m.Cluster
	cfg["rpc_address"] = "0.0.0.0"
	cfg["endpoint_snitch"] = "GossipingPropertyFileSnitch"

	overrideYAMLBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to parse override options for scylla.yaml")
	}
	scyllaYAMLConfigMapFilteredBytes, err := mergeYAMLs(scyllaYAMLConfigMapBytes, overrideYAMLBytes)
	if err != nil {
		return errors.Wrap(err, "failed to merged config map YAML with default yaml values")
	}

	customScyllaYAMLBytes, err := mergeYAMLs(scyllaYAMLBytes, scyllaYAMLConfigMapFilteredBytes)
	if err != nil {
		return errors.Wrap(err, "failed to merged YAMLs")
	}

	// Write result to file
	if err = ioutil.WriteFile(scyllaYAMLPath, customScyllaYAMLBytes, os.ModePerm); err != nil {
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

func (s *ScyllaConfig) setupEntrypoint(ctx context.Context) (*exec.Cmd, error) {
	m := s.member
	// Get seeds
	seeds, err := m.GetSeeds(s.kubeClient)
	if err != nil {
		return nil, errors.Wrap(err, "error getting seeds")
	}

	// Check if we need to run in developer mode
	devMode := "0"
	cluster := &v1alpha1.Cluster{}
	err = s.Get(ctx, naming.NamespacedName(s.member.Cluster, s.member.Namespace), cluster)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}
	if cluster.Spec.DeveloperMode {
		devMode = "1"
	}

	args := []string{
		fmt.Sprintf("--listen-address=%s", m.IP),
		fmt.Sprintf("--broadcast-address=%s", m.StaticIP),
		fmt.Sprintf("--broadcast-rpc-address=%s", m.StaticIP),
		fmt.Sprintf("--seeds=%s", strings.Join(seeds, ",")),
		fmt.Sprintf("--developer-mode=%s", devMode),
		fmt.Sprintf("--smp=%s", options.GetSidecarOptions().CPU),
	}
	if cluster.Spec.Alternator.Enabled() {
		args = append(args, fmt.Sprintf("--alternator-port=%d", cluster.Spec.Alternator.Port))
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

		args = append(args, fmt.Sprintf("--cpuset=%s", cpusAllowed))
	}

	scyllaCmd := exec.Command(entrypointPath, args...)
	scyllaCmd.Stderr = os.Stderr
	scyllaCmd.Stdout = os.Stdout
	s.logger.Info(ctx, "Scylla entrypoint", "command", scyllaCmd)

	return scyllaCmd, nil
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
