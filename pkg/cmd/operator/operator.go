package operator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	monitoringversionedclient "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/clientset/versioned"
	monitoringinformers "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	cryptoKeyBufferSizeMaxFlagKey = "crypto-key-buffer-size-max"
)

type OperatorOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	genericclioptions.LeaderElection

	kubeClient       kubernetes.Interface
	scyllaClient     scyllaversionedclient.Interface
	monitoringClient monitoringversionedclient.Interface

	ConcurrentSyncs int
	OperatorImage   string
	CQLSIngressPort int

	CryptoKeyBufferSizeMin int
	CryptoKeyBufferSizeMax int
	CryptoKeyBufferDelay   time.Duration
}

func NewOperatorOptions(streams genericclioptions.IOStreams) *OperatorOptions {
	return &OperatorOptions{
		ClientConfig:        genericclioptions.NewClientConfig("scylla-operator"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		LeaderElection:      genericclioptions.NewLeaderElection(),

		ConcurrentSyncs: 50,
		OperatorImage:   "",
		CQLSIngressPort: 0,

		CryptoKeyBufferSizeMin: 10,
		CryptoKeyBufferSizeMax: 30,
		CryptoKeyBufferDelay:   200 * time.Millisecond,
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
	cmd.Flags().IntVarP(&o.CryptoKeyBufferSizeMin, "crypto-key-buffer-size-min", "", o.CryptoKeyBufferSizeMin, "Minimal number of pre-generated crypto keys that are used for quick certificate issuance. The minimum size is 1.")
	cmd.Flags().IntVarP(&o.CryptoKeyBufferSizeMax, cryptoKeyBufferSizeMaxFlagKey, "", o.CryptoKeyBufferSizeMax, "Maximum number of pre-generated crypto keys that are used for quick certificate issuance. The minimum size is 1. If not set, it will adjust to be at least the size of crypto-key-buffer-size-min.")
	cmd.Flags().DurationVarP(&o.CryptoKeyBufferDelay, "crypto-key-buffer-delay", "", o.CryptoKeyBufferDelay, "Delay is the time to wait when generating next certificate in the (min, max) range. Certificate generation bellow the min threshold is not affected.")
}

func (o *OperatorOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.LeaderElection.Validate())

	if len(o.OperatorImage) == 0 {
		errs = append(errs, errors.New("operator image can't be empty"))
	}

	if len(o.OperatorImage) == 0 {
		errs = append(errs, errors.New("operator image can't be empty"))
	}

	if o.CryptoKeyBufferSizeMin < 1 {
		errs = append(errs, fmt.Errorf("crypto-key-buffer-size-min (%d) has to be at least 1", o.CryptoKeyBufferSizeMin))
	}

	if o.CryptoKeyBufferSizeMax < 1 {
		errs = append(errs, fmt.Errorf("crypto-key-buffer-size-max (%d) has to be at least 1", o.CryptoKeyBufferSizeMax))
	}

	if o.CryptoKeyBufferSizeMax < o.CryptoKeyBufferSizeMin {
		errs = append(errs, fmt.Errorf(
			"crypto-key-buffer-size-max (%d) can't be lower then crypto-key-buffer-size-min (%d)",
			o.CryptoKeyBufferSizeMax,
			o.CryptoKeyBufferSizeMin,
		))
	}

	msg := validation.IsInRange(o.CQLSIngressPort, 0, 65535)
	if len(msg) != 0 {
		errs = append(errs, fmt.Errorf("invalid secure cql ingress port %d: %s", o.CQLSIngressPort, msg))
	}

	return apierrors.NewAggregate(errs)
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

	maxChanged := cmd.Flags().Lookup(cryptoKeyBufferSizeMaxFlagKey).Changed
	if !maxChanged && o.CryptoKeyBufferSizeMin > o.CryptoKeyBufferSizeMax {
		o.CryptoKeyBufferSizeMax = o.CryptoKeyBufferSizeMin
	}

	return nil
}

func (o *OperatorOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
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
	rsaKeyGenerator, err := crypto.NewRSAKeyGenerator(
		o.CryptoKeyBufferSizeMin,
		o.CryptoKeyBufferSizeMax,
		o.CryptoKeyBufferDelay,
	)
	if err != nil {
		return fmt.Errorf("can't create rsa key generator: %w", err)
	}
	defer rsaKeyGenerator.Close()

	kubeInformers := informers.NewSharedInformerFactory(o.kubeClient, resyncPeriod)
	scyllaInformers := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)

	scyllaOperatorConfigInformers := scyllainformers.NewSharedInformerFactoryWithOptions(o.scyllaClient, resyncPeriod, scyllainformers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", naming.SingletonName).String()
		},
	))

	monitoringInformers := monitoringinformers.NewSharedInformerFactory(o.monitoringClient, resyncPeriod)

	scc, err := scyllacluster.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1(),
		kubeInformers.Core().V1().Pods(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Apps().V1().StatefulSets(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Networking().V1().Ingresses(),
		kubeInformers.Batch().V1().Jobs(),
		kubeInformers.Discovery().V1().EndpointSlices(),
		kubeInformers.Core().V1().Endpoints(),
		scyllaInformers.Scylla().V1().ScyllaClusters(),
		o.OperatorImage,
		o.CQLSIngressPort,
		rsaKeyGenerator,
	)
	if err != nil {
		return fmt.Errorf("can't create scyllacluster controller: %w", err)
	}

	opc, err := orphanedpv.NewController(
		o.kubeClient,
		kubeInformers.Core().V1().PersistentVolumes(),
		kubeInformers.Core().V1().PersistentVolumeClaims(),
		kubeInformers.Core().V1().Nodes(),
		scyllaInformers.Scylla().V1().ScyllaClusters(),
	)
	if err != nil {
		return fmt.Errorf("can't create orphanpv controller: %w", err)
	}

	ncc, err := nodeconfig.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaInformers.Scylla().V1alpha1().NodeConfigs(),
		scyllaInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		kubeInformers.Rbac().V1().ClusterRoles(),
		kubeInformers.Rbac().V1().ClusterRoleBindings(),
		kubeInformers.Apps().V1().DaemonSets(),
		kubeInformers.Core().V1().Namespaces(),
		kubeInformers.Core().V1().Nodes(),
		kubeInformers.Core().V1().ServiceAccounts(),
		o.OperatorImage,
	)
	if err != nil {
		return fmt.Errorf("can't create nodeconfig controller: %w", err)
	}

	ncpc, err := nodeconfigpod.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		kubeInformers.Core().V1().Pods(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().Nodes(),
		scyllaInformers.Scylla().V1alpha1().NodeConfigs(),
	)
	if err != nil {
		return fmt.Errorf("can't create nodeconfigpod controller: %w", err)
	}

	socc, err := scyllaoperatorconfig.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaOperatorConfigInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
	)
	if err != nil {
		return fmt.Errorf("can't create scyllaoperatorconfig controller: %w", err)
	}

	mc, err := scylladbmonitoring.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		o.monitoringClient.MonitoringV1(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Apps().V1().Deployments(),
		kubeInformers.Networking().V1().Ingresses(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBMonitorings(),
		monitoringInformers.Monitoring().V1().Prometheuses(),
		monitoringInformers.Monitoring().V1().PrometheusRules(),
		monitoringInformers.Monitoring().V1().ServiceMonitors(),
		rsaKeyGenerator,
	)
	if err != nil {
		return fmt.Errorf("can't create scylladbmonitoring controller: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rsaKeyGenerator.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		kubeInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scyllaInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scyllaOperatorConfigInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		monitoringInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		opc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ncc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ncpc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		socc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		mc.Run(ctx, o.ConcurrentSyncs)
	}()

	<-ctx.Done()

	return nil
}
