package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"

	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/clusterdomain"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type OperatorOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	genericclioptions.LeaderElection

	kubeClient                 kubernetes.Interface
	scyllaClient               scyllaversionedclient.Interface
	monitoringClient           monitoringversionedclient.Interface
	dynamicClusterDomainGetter *clusterdomain.DynamicClusterDomain

	clusterKubeClient   remoteclient.ClusterClient[kubernetes.Interface]
	clusterScyllaClient remoteclient.ClusterClient[scyllaversionedclient.Interface]

	ConcurrentSyncs    int
	OperatorImage      string
	CQLSIngressPort    int
	CryptoKeyOptions   CryptoKeyOptions
	MetricsBindAddress string
}

func NewOperatorOptions(streams genericclioptions.IOStreams) *OperatorOptions {
	return &OperatorOptions{
		ClientConfig:        genericclioptions.NewClientConfig("scylla-operator"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		LeaderElection:      genericclioptions.NewLeaderElection(),

		ConcurrentSyncs:    50,
		OperatorImage:      "",
		CQLSIngressPort:    0,
		CryptoKeyOptions:   DefaultCryptoKeyOptions(),
		MetricsBindAddress: controllermanager.MetricsDisabledBindAddress,
	}
}

func NewOperatorCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOperatorOptions(streams)

	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the scylla operator.",
		Long:  `Run the scylla operator.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Complete(cmd)
			if err != nil {
				return err
			}

			err = o.Validate()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *OperatorOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)
	o.LeaderElection.AddFlags(cmd)

	cmd.Flags().IntVarP(&o.ConcurrentSyncs, "concurrent-syncs", "", o.ConcurrentSyncs, "The number of ScyllaCluster objects that are allowed to sync concurrently.")
	cmd.Flags().StringVarP(&o.OperatorImage, "image", "", o.OperatorImage, "Image of the operator used.")
	cmd.Flags().IntVarP(&o.CQLSIngressPort, "cqls-ingress-port", "", o.CQLSIngressPort, "Port on which is the ingress controller listening for secure CQL connections.")
	cmd.Flags().StringVarP(&o.MetricsBindAddress, "metrics-bind-address", "", o.MetricsBindAddress, fmt.Sprintf("The address the controller metrics endpoint binds to, e.g. \":8080\". Set to %q to disable serving metrics.", controllermanager.MetricsDisabledBindAddress))
	o.CryptoKeyOptions.AddFlags(cmd)
}

func (o *OperatorOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.LeaderElection.Validate())
	errs = append(errs, o.CryptoKeyOptions.Validate())

	if len(o.OperatorImage) == 0 {
		errs = append(errs, errors.New("operator image can't be empty"))
	}

	if len(o.OperatorImage) == 0 {
		errs = append(errs, errors.New("operator image can't be empty"))
	}

	msg := apimachineryutilvalidation.IsInRange(o.CQLSIngressPort, 0, 65535)
	if len(msg) != 0 {
		errs = append(errs, fmt.Errorf("invalid secure cql ingress port %d: %s", o.CQLSIngressPort, msg))
	}

	if o.MetricsBindAddress != controllermanager.MetricsDisabledBindAddress {
		_, _, err := net.SplitHostPort(o.MetricsBindAddress)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid metrics bind address %q: %w", o.MetricsBindAddress, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *OperatorOptions) Complete(cmd *cobra.Command) error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	err = o.LeaderElection.Complete()
	if err != nil {
		return err
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.scyllaClient, err = scyllaversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build scylla clientset: %w", err)
	}

	o.monitoringClient, err = monitoringversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build monitoring clientset: %w", err)
	}

	o.dynamicClusterDomainGetter = clusterdomain.NewDynamicClusterDomain(net.DefaultResolver)

	o.clusterKubeClient = *remoteclient.NewClusterClient(func(config []byte) (kubernetes.Interface, error) {
		restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
		if err != nil {
			return nil, fmt.Errorf("can't create REST config from kubeconfig: %w", err)
		}

		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("can't build kubernetes clientset: %w", err)
		}

		return client, nil
	})

	o.clusterScyllaClient = *remoteclient.NewClusterClient(func(config []byte) (scyllaversionedclient.Interface, error) {
		restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
		if err != nil {
			return nil, fmt.Errorf("can't create REST config from kubeconfig: %w", err)
		}

		client, err := scyllaversionedclient.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("can't build scylla clientset: %w", err)
		}

		return client, nil
	})

	err = o.CryptoKeyOptions.Complete(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (o *OperatorOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.Execute(ctx, streams, cmd)
}

func (o *OperatorOptions) Execute(ctx context.Context, streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	// Lock names cannot be changed, because it may lead to two leaders during rolling upgrades.
	const lockName = "scylla-operator-lock"

	return leaderelection.Run(
		ctx,
		cmd.Name(),
		lockName,
		o.Namespace,
		o.kubeClient,
		o.LeaderElectionLeaseDuration,
		o.LeaderElectionRenewDeadline,
		o.LeaderElectionRetryPeriod,
		func(ctx context.Context) error {
			return o.run(ctx, streams)
		},
	)
}

func (o *OperatorOptions) run(ctx context.Context, streams genericclioptions.IOStreams) error {
	invalidTaskNameRefs, err := listScyllaClustersWithNonRFC1123SubdomainTaskNames(ctx, o.scyllaClient.ScyllaV1())
	if err != nil {
		return fmt.Errorf("can't check for ScyllaClusters with invalid task names: %w", err)
	}
	if len(invalidTaskNameRefs) > 0 {
		return fmt.Errorf(
			"ScyllaCluster(s) %v have repair or backup task names that do not conform to RFC 1123 subdomain requirements. Please update the task names before starting ScyllaDB Operator.",
			invalidTaskNameRefs,
		)
	}

	keyGenerator, err := crypto.NewKeyGenerator(o.CryptoKeyOptions.ToKeyGeneratorConfig())
	if err != nil {
		return fmt.Errorf("can't create key generator: %w", err)
	}
	defer keyGenerator.Close()

	monitoringCRDsInstalled, err := helpers.IsAPIGroupVersionAvailable(o.kubeClient.Discovery(), "monitoring.coreos.com/v1")
	if err != nil {
		return fmt.Errorf("can't check if monitoring API group version is available: %w", err)
	}
	if !monitoringCRDsInstalled {
		klog.InfoS("Prometheus Operator CRDs (monitoring.coreos.com) are not installed in the cluster. " +
			"ScyllaDBMonitoring controller will not be started. " +
			"To enable monitoring, install Prometheus Operator and restart the ScyllaDB Operator.")
	}

	// controller-runtime logs through logr; route it to klog so the log flags keep applying to everything.
	ctrllog.SetLogger(klog.NewKlogr())

	cm, err := controllermanager.New(controllermanager.Options{
		RestConfig:              o.RestConfig,
		Logger:                  klog.NewKlogr(),
		KubeClient:              o.kubeClient,
		ScyllaClient:            o.scyllaClient,
		MonitoringClient:        o.monitoringClient,
		MonitoringCRDsInstalled: monitoringCRDsInstalled,
		ClusterKubeClient:       &o.clusterKubeClient,
		ClusterScyllaClient:     &o.clusterScyllaClient,
		ClusterDomainGetter:     o.dynamicClusterDomainGetter.GetClusterDomain,
		KeyGenerator:            keyGenerator,
		OperatorImage:           o.OperatorImage,
		CQLSIngressPort:         o.CQLSIngressPort,
		ConcurrentSyncs:         o.ConcurrentSyncs,
		ResyncPeriod:            resyncPeriod,
		MetricsBindAddress:      o.MetricsBindAddress,
	})
	if err != nil {
		return fmt.Errorf("can't create controller manager: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		keyGenerator.Run(ctx)
	}()

	return cm.Run(ctx)
}

func listScyllaClustersWithNonRFC1123SubdomainTaskNames(ctx context.Context, scyllaV1Client scyllav1client.ScyllaV1Interface) ([]string, error) {
	scyllaClusters, err := scyllaV1Client.ScyllaClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't list ScyllaClusters: %w", err)
	}

	var invalidRefs []string
	for _, sc := range scyllaClusters.Items {
		taskNames := slices.Concat(
			oslices.ConvertSlice(sc.Spec.Repairs, func(r scyllav1.RepairTaskSpec) string {
				return r.Name
			}),
			oslices.ConvertSlice(sc.Spec.Backups, func(b scyllav1.BackupTaskSpec) string {
				return b.Name
			}),
		)

		hasInvalidTaskName := oslices.Contains(taskNames, func(name string) bool {
			return len(apimachineryutilvalidation.IsDNS1123Subdomain(name)) > 0
		})
		if hasInvalidTaskName {
			invalidRefs = append(invalidRefs, naming.ManualRef(sc.Namespace, sc.Name))
		}
	}

	return invalidRefs, nil
}
