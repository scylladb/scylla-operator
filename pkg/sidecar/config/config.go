package config

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

const (
	configDirScylla            = "/etc/scylla"
	scyllaYAMLPath             = configDirScylla + "/" + "scylla.yaml"
	scyllaRackDCPropertiesPath = configDirScylla + "/" + "cassandra-rackdc.properties"
	scyllaJMXPath              = "/usr/lib/scylla/jmx/scylla-jmx"
	jolokiaPath                = naming.SharedDirName + "/" + naming.JolokiaJarName
	entrypointPath             = "/docker-entrypoint.py"
	rackDCPropertiesFormat     = "dc=%s" + "\n" + "rack=%s" + "\n" + "prefer_local=false" + "\n"
)

type ScyllaConfig struct {
	client.Client
	member     *identity.Member
	kubeClient kubernetes.Interface
}

func NewForMember(
	m *identity.Member,
	kubeClient kubernetes.Interface,
	client client.Client,
) *ScyllaConfig {
	return &ScyllaConfig{
		member:     m,
		kubeClient: kubeClient,
		Client:     client,
	}
}

func (s *ScyllaConfig) Setup() (*exec.Cmd, error) {

	var err error
	var cmd *exec.Cmd

	log.Info("Setting up scylla.yaml")
	if err = s.setupScyllaYAML(); err != nil {
		return nil, errors.Wrap(err, "failed to setup scylla.yaml")
	}
	log.Info("Setting up cassandra-rackdc.properties")
	if err = s.setupRackDCProperties(); err != nil {
		return nil, errors.Wrap(err, "failed to setup rackdc properties file")
	}
	log.Info("Setting up jolokia config")
	if err = s.setupJolokia(); err != nil {
		return nil, errors.Wrap(err, "failed to setup jolokia")
	}
	log.Info("Setting up entrypoint script")
	if cmd, err = s.setupEntrypoint(); err != nil {
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

	customScyllaYAMLBytes, err := mergeYAMLs(scyllaYAMLBytes, overrideYAMLBytes)
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
	rackdcProperties := []byte(fmt.Sprintf(rackDCPropertiesFormat, s.member.Datacenter, s.member.Rack))
	if err := ioutil.WriteFile(scyllaRackDCPropertiesPath, rackdcProperties, os.ModePerm); err != nil {
		return errors.Wrap(err, "error trying to write cassandra-rackdc.properties")
	}
	return nil
}

// setupJolokia injects jolokia as a javaagent by
// modifying the scylla-jmx file.
func (s *ScyllaConfig) setupJolokia() error {

	// Create Jolokia Config
	opts := []struct {
		flag, value string
	}{
		{
			flag:  "host",
			value: "localhost",
		},
		{
			flag:  "port",
			value: fmt.Sprintf("%d", naming.JolokiaPort),
		},
		{
			flag:  "executor",
			value: "fixed",
		},
		{
			flag:  "threadNr",
			value: "2",
		},
	}

	cmd := []string{}
	for _, opt := range opts {
		cmd = append(cmd, fmt.Sprintf("%s=%s", opt.flag, opt.value))
	}
	jolokiaCfg := fmt.Sprintf("-javaagent:%s=%s", jolokiaPath, strings.Join(cmd, ","))

	// Open scylla-jmx file
	scyllaJMXBytes, err := ioutil.ReadFile(scyllaJMXPath)
	if err != nil {
		return errors.Wrap(err, "error reading scylla-jmx")
	}
	// Inject jolokia as a javaagent
	scyllaJMX := string(scyllaJMXBytes)
	splitIndex := strings.Index(scyllaJMX, `\`) + len(`\`)
	injectedLine := fmt.Sprintf("\n    %s \\", jolokiaCfg)
	scyllaJMXCustom := scyllaJMX[:splitIndex] + injectedLine + scyllaJMX[splitIndex:]
	// Write the custom scylla-jmx contents back
	if err := ioutil.WriteFile(scyllaJMXPath, []byte(scyllaJMXCustom), os.ModePerm); err != nil {
		return errors.Wrap(err, "error writing scylla-jmx: %s")
	}
	return nil
}

func (s *ScyllaConfig) setupEntrypoint() (*exec.Cmd, error) {
	m := s.member
	// Get seeds
	seeds, err := m.GetSeeds(s.kubeClient)
	if err != nil {
		return nil, errors.Wrap(err, "error getting seeds")
	}

	// Check if we need to run in developer mode
	devMode := "0"
	cluster := &v1alpha1.Cluster{}
	err = s.Get(context.TODO(), naming.NamespacedName(s.member.Cluster, s.member.Namespace), cluster)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}
	if cluster.Spec.DeveloperMode {
		devMode = "1"
	}

	// Get cpu cores
	cpu := options.GetSidecarOptions().CPU

	// Get memory
	mem := options.GetSidecarOptions().Memory
	// Leave some memory for other stuff
	memNumber, _ := strconv.ParseInt(mem, 10, 64)
	maxFn := func(x, y int64) (z int64) {
		if z = x; x < y {
			z = y
		}
		return
	}
	mem = fmt.Sprintf("%dM", maxFn(memNumber-700, 0))

	args := []string{
		fmt.Sprintf("--listen-address=%s", m.IP),
		fmt.Sprintf("--broadcast-address=%s", m.StaticIP),
		fmt.Sprintf("--broadcast-rpc-address=%s", m.StaticIP),
		fmt.Sprintf("--seeds=%s", strings.Join(seeds, ",")),
		fmt.Sprintf("--developer-mode=%s", devMode),
		fmt.Sprintf("--smp=%s", cpu),
		fmt.Sprintf("--memory=%s", mem),
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
	log.Infof("Scylla entrypoint command:\n %v", scyllaCmd)

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
