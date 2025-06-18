package genericclioptions

import (
	"fmt"
	"io"
	"os"
	goruntime "runtime"
	"time"

	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultKubeconfig = ""
)

// IOStreams is a structure containing all standard streams.
type IOStreams struct {
	// In think, os.Stdin
	In io.Reader
	// Out think, os.Stdout
	Out io.Writer
	// ErrOut think, os.Stderr
	ErrOut io.Writer
}

type MultiClusterClientConfig struct {
	ClientConfig

	workerKubeconfigs map[string]string

	WorkerClientConfigs map[string]ClientConfig
}

func NewMultiClusterClientConfig(userAgentName string) MultiClusterClientConfig {
	return MultiClusterClientConfig{
		ClientConfig: NewClientConfig(userAgentName),

		workerKubeconfigs: map[string]string{},

		WorkerClientConfigs: map[string]ClientConfig{},
	}
}

func (mccc *MultiClusterClientConfig) AddFlags(cmd *cobra.Command) {
	mccc.ClientConfig.AddFlags(cmd)

	// TODO: comment about multi-cluster setup
	// TODO: description
	cmd.PersistentFlags().StringToStringVarP(&mccc.workerKubeconfigs, "worker-kubeconfigs", "", mccc.workerKubeconfigs, "")
}

func (mccc *MultiClusterClientConfig) Validate() error {
	var errs []error
	var err error

	err = mccc.ClientConfig.Validate()
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid client config: %w", err))
	}

	for name, kubeconfig := range mccc.workerKubeconfigs {
		cc := NewClientConfig(mccc.UserAgentName)
		cc.Kubeconfig = kubeconfig
		cc.QPS = mccc.QPS
		cc.Burst = mccc.Burst

		err = cc.Validate()
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid client config for %q kubeconfig at %q: %w", name, kubeconfig, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (mccc *MultiClusterClientConfig) Complete() error {
	var err error

	err = mccc.ClientConfig.Complete()
	if err != nil {
		return fmt.Errorf("can't complete client config: %w", err)
	}

	dataplaneClientConfigMap := make(map[string]ClientConfig, len(mccc.workerKubeconfigs))
	for dataplaneClusterName, kubeconfig := range mccc.workerKubeconfigs {
		cc := NewClientConfig(mccc.UserAgentName)
		cc.Kubeconfig = kubeconfig
		cc.QPS = mccc.QPS
		cc.Burst = mccc.Burst

		err = cc.Complete()
		if err != nil {
			// TODO: use key in the error message
			return fmt.Errorf("can't complete client config for kubeconfig %q: %w", kubeconfig, err)
		}

		dataplaneClientConfigMap[dataplaneClusterName] = cc
	}

	mccc.WorkerClientConfigs = dataplaneClientConfigMap

	return nil
}

type ClientConfig struct {
	QPS           float32
	Burst         int
	UserAgentName string
	Kubeconfig    string
	RestConfig    *restclient.Config
	ProtoConfig   *restclient.Config
}

func MakeVersionedUserAgent(baseName string) string {
	return fmt.Sprintf(
		"%s/%s (%s/%s) scylla-operator/%s",
		baseName,
		version.Get().GitVersion,
		goruntime.GOOS,
		goruntime.GOARCH,
		version.Get().GitCommit,
	)
}

func NewClientConfig(userAgentName string) ClientConfig {
	return ClientConfig{
		QPS:           50,
		Burst:         75,
		UserAgentName: userAgentName,
		Kubeconfig:    defaultKubeconfig,
		RestConfig:    nil,
		ProtoConfig:   nil,
	}
}

func (cc *ClientConfig) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Float32VarP(&cc.QPS, "qps", "", cc.QPS, "Maximum allowed number of queries per second.")
	cmd.PersistentFlags().IntVarP(&cc.Burst, "burst", "", cc.Burst, "Allows extra queries to accumulate when a client is exceeding its rate.")
	cmd.PersistentFlags().StringVarP(&cc.Kubeconfig, "kubeconfig", "", cc.Kubeconfig, "Path to the kubeconfig file.")
}

func (cc *ClientConfig) Validate() error {
	return nil
}

func (cc *ClientConfig) Complete() error {
	var err error

	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	// Use explicit kubeconfig if set.
	loader.ExplicitPath = cc.Kubeconfig
	cc.RestConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("can't create client config: %w", err)
	}

	cc.RestConfig.QPS = cc.QPS
	cc.RestConfig.Burst = cc.Burst
	cc.RestConfig.UserAgent = MakeVersionedUserAgent(cc.UserAgentName)

	cc.ProtoConfig = restclient.CopyConfig(cc.RestConfig)
	cc.ProtoConfig.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	cc.ProtoConfig.ContentType = "application/vnd.kubernetes.protobuf"

	return nil
}

type InClusterReflection struct {
	Namespace string
}

func (o *InClusterReflection) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&o.Namespace, "namespace", "", o.Namespace, "Namespace where the controller is running. Auto-detected if run inside a cluster.")
}

func (o *InClusterReflection) Validate() error {
	return nil
}

func (o *InClusterReflection) Complete() error {
	if len(o.Namespace) == 0 {
		// Autodetect if running inside a cluster
		bytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			return fmt.Errorf("can't autodetect controller namespace: %w", err)
		}

		o.Namespace = string(bytes)
	}

	return nil
}

type LeaderElection struct {
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration
}

func NewLeaderElection() LeaderElection {
	return LeaderElection{
		LeaderElectionLeaseDuration: 60 * time.Second,
		LeaderElectionRenewDeadline: 35 * time.Second,
		LeaderElectionRetryPeriod:   10 * time.Second,
	}
}

func (le *LeaderElection) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().DurationVar(&le.LeaderElectionLeaseDuration, "leader-election-lease-duration", le.LeaderElectionLeaseDuration, "LeaseDuration is the duration that non-leader candidates will wait to force acquire leadership.")
	cmd.PersistentFlags().DurationVar(&le.LeaderElectionRenewDeadline, "leader-election-renew-deadline", le.LeaderElectionRenewDeadline, "RenewDeadline is the duration that the acting master will retry refreshing leadership before giving up.")
	cmd.PersistentFlags().DurationVar(&le.LeaderElectionRetryPeriod, "leader-election-retry-period", le.LeaderElectionRetryPeriod, "RetryPeriod is the duration the LeaderElector clients should wait between tries of actions.")
}

func (le *LeaderElection) Validate() error {
	return nil
}

func (le *LeaderElection) Complete() error {
	return nil
}
