package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/shlex"
	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/arg"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	configDirScylla                     = "/etc/scylla"
	configDirScyllaD                    = "/etc/scylla.d"
	scyllaYAMLPath                      = configDirScylla + "/" + naming.ScyllaConfigName
	scyllaYAMLConfigMapPath             = naming.ScyllaConfigDirName + "/" + naming.ScyllaConfigName
	ScyllaIOPropertiesPath              = configDirScyllaD + "/" + naming.ScyllaIOPropertiesName
	scyllaRackDCPropertiesPath          = configDirScylla + "/" + naming.ScyllaRackDCPropertiesName
	scyllaRackDCPropertiesConfigMapPath = naming.ScyllaConfigDirName + "/" + naming.ScyllaRackDCPropertiesName
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

// SetupScyllaYAML edits the default scylla.yaml file with our custom options.
// We only edit the options that are not available to configure via the
// entrypoint script flags. Those options are:
// - cluster_name
// - rpc_address
// - endpoint_snitch
func (s *ScyllaConfig) SetupScyllaYAML() error {
	// Read default scylla.yaml
	configFileBytes, err := ioutil.ReadFile(scyllaYAMLPath)
	if err != nil {
		return errors.Wrap(err, "failed to open scylla.yaml")
	}

	// Read config map scylla.yaml
	configMapBytes, err := ioutil.ReadFile(scyllaYAMLConfigMapPath)
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
	if err = ioutil.WriteFile(scyllaYAMLPath, customConfigBytesBytes, os.ModePerm); err != nil {
		return errors.Wrap(err, "error trying to write scylla.yaml")
	}

	return nil
}

func (s *ScyllaConfig) SetupRackDCProperties() error {
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

func (s *ScyllaConfig) GenerateScyllaEnv() []string {
	osEnv := os.Environ()
	env := make([]string, 0, len(osEnv)+1)
	env = append(env, osEnv...)
	env = append(env, "SCYLLA_HOME=/var/lib/scylla")
	env = append(env, "SCYLLA_CONF=/etc/scylla")
	return env
}

func (s *ScyllaConfig) GetScyllaUserArgs(ctx context.Context) ([]string, error) {
	// TODO: cache it
	cluster, err := s.scyllaClient.ScyllaV1().ScyllaClusters(s.member.Namespace).Get(ctx, s.member.Cluster, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}

	version := semver.NewScyllaVersion(cluster.Spec.Version)
	if !version.SupportFeatureUnsafe(semver.ScyllaVersionThatSupportsArgs) && len(cluster.Spec.ScyllaArgs) > 0 {
		klog.InfoS("This scylla version does not support ScyllaArgs. ScyllaArgs is ignored", "version", cluster.Spec.Version)
		return nil, nil
	}

	// This is an unfortunate API design (eventually we want to migrate it to `unsupportedScyllaArgs []string`).
	// We need to parse the qouted arguments on our own :(
	args, err := shlex.Split(cluster.Spec.ScyllaArgs)
	if err != nil {
		return nil, fmt.Errorf("can't split scylla args: %w", err)
	}

	return args, nil
}

func (s *ScyllaConfig) GenerateScyllaArgs(ctx context.Context) ([]string, error) {
	m := s.member
	// Get seeds
	seedAddress, err := m.GetSeed(ctx, s.kubeClient.CoreV1())
	if err != nil {
		return nil, fmt.Errorf("can't get seed address: %w", err)
	}

	// Check if we need to run in developer mode
	cluster, err := s.scyllaClient.ScyllaV1().ScyllaClusters(s.member.Namespace).Get(ctx, s.member.Cluster, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}

	// Avoid a map to always write the arguments in a stable order.
	// TODO: move some of these to the config.
	args := arg.NewArgs()
	// Listen on all interfaces so users or a service mesh can use localhost.
	args.AddArgWithStringValue("--listen-address", "0.0.0.0")
	args.AddArgWithStringValue("--broadcast-address", m.StaticIP)
	args.AddArgWithStringValue("--broadcast-rpc-address", m.StaticIP)
	args.AddArgWithStringValue("--seed-provider-parameters", fmt.Sprintf("seeds=%s", seedAddress))
	args.AddArgWithIntValue("--developer-mode", arg.IntFromBool(cluster.Spec.DeveloperMode))
	args.AddArgWithIntValue("--smp", s.cpuCount)
	args.AddArgWithIntValue("--log-to-syslog", 0)
	args.AddArgWithIntValue("--log-to-stdout", 1)
	args.AddArgWithStringValue("--prometheus-address", "0.0.0.0")
	args.AddArgWithStringValue("--default-log-level", "info")
	args.AddArgWithStringValue("--network-stack", "posix")
	args.AddArgWithIntValue("--blocked-reactor-notify-ms", 999999999)

	if m.Overprovisioned {
		args.AddArg("--overprovisioned")
	}
	if cluster.Spec.Alternator.Enabled() {
		args.AddArgWithStringValue("--alternator-address", "0.0.0.0")
		args.AddArgWithIntValue("--alternator-port", int(cluster.Spec.Alternator.Port))
		if cluster.Spec.Alternator.WriteIsolation != "" {
			args.AddArgWithStringValue("--alternator-write-isolation", cluster.Spec.Alternator.WriteIsolation)
		}
	}
	// If node is being replaced
	if addr, ok := m.ServiceLabels[naming.ReplaceLabel]; ok {
		args.AddArgWithStringValue("--replace-address-first-boot", addr)
	}

	// if _, err := os.Stat(scyllaIOPropertiesPath); err == nil && version.SupportFeatureSafe(semver.ScyllaVersionThatSupportsDisablingIOTuning) {
	// 	klog.InfoS("Scylla IO properties are already set, skipping io tuning")
	// 	ioSetup := "0"
	// 	ab.AddArgWithIntValue("--cpuset", cpusAllowed)
	// 	argMap["io-setup"] = &ioSetup
	// 	argMap["io-properties-file"] = pointer.StringPtr(scyllaIOPropertiesPath)
	// }

	// 	command := []string{
	// 		"/usr/bin/bash",
	// 		"-euExo",
	// 		"pipefail",
	// 		"-O",
	// 		"inherit_errexit",
	// 		"-c",
	// 		`
	// # We need to setup /etc/scylla.d/dev-mode.conf for scylla_io_setup
	// /opt/scylladb/scripts/scylla_dev_mode_setup --developer-mode="${DEVELOPER_MODE}"
	//
	// #todo cpuset conf file
	// /opt/scylladb/scripts/scylla_io_setup
	//
	// /opt/scylladb/scripts/scylla_prepare
	// exec /usr/bin/scylla ` + ab.String() + userArgs,
	// 	}

	// klog.V(4).InfoS("Scylla command", "Command", command)

	return args.ArgStrings(), nil
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

func (s *ScyllaConfig) GetCpusetArg(ctx context.Context) (*arg.Arg, error) {
	cluster, err := s.scyllaClient.ScyllaV1().ScyllaClusters(s.member.Namespace).Get(ctx, s.member.Cluster, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster")
	}

	// See if we need to use cpu-pinning
	// TODO: Add more checks to make sure this is valid.
	// eg. parse the cpuset and check the number of cpus is the same as cpu limits
	// Now we rely completely on the user to have the cpu policy correctly
	// configured in the kubelet, otherwise scylla will crash.
	if !cluster.Spec.CpuSet {
		return nil, nil
	}

	cpusAllowed, err := getCPUsAllowedList("/proc/1/status")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = s.validateCpuSet(ctx, cpusAllowed, s.cpuCount)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &arg.Arg{
		Flag:  "--cpuset",
		Value: &cpusAllowed,
	}, nil
}

func SetupIOTuneCache() error {
	if _, err := os.Stat(ScyllaIOPropertiesPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("can't stat file %q: %w", ScyllaIOPropertiesPath, err)
		}

		cachePath := path.Join(naming.DataDir, naming.ScyllaIOPropertiesName)
		if err := os.Symlink(cachePath, ScyllaIOPropertiesPath); err != nil {
			return fmt.Errorf("can't create symlink from %q to %q: %w", ScyllaIOPropertiesPath, cachePath, err)
		}

		klog.V(2).Info("Initialized IOTune benchmark cache", "path", ScyllaIOPropertiesPath, "cachePath", cachePath)
	} else {
		klog.V(2).Info("Found cached IOTune benchmark results", "path", ScyllaIOPropertiesPath)
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
