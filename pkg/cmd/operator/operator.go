package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/clusterdomain"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
)

type OperatorOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	genericclioptions.LeaderElection

	kubeClient                 kubernetes.Interface
	scyllaClient               scyllaversionedclient.Interface
	monitoringClient           monitoringversionedclient.Interface
	dynamicClusterDomainGetter *clusterdomain.DynamicClusterDomain

	ConcurrentSyncs  int
	OperatorImage    string
	CQLSIngressPort  int
	CryptoKeyOptions CryptoKeyOptions

	// ReconcileTraceDir and ReconcileTraceAddress drive an experiment: recording the reconcile trace of the
	// controllers on controller-runtime for review in tests. Empty ReconcileTraceDir disables it.
	ReconcileTraceDir     string
	ReconcileTraceAddress string

	reconcileTraceStore *reconciletrace.NamespacedFileStore
}

// reconcileTraceMaxBytesPerNamespace caps the trace of one namespace.
const reconcileTraceMaxBytesPerNamespace = 128 << 20

func NewOperatorOptions(streams genericclioptions.IOStreams) *OperatorOptions {
	return &OperatorOptions{
		ClientConfig:        genericclioptions.NewClientConfig("scylla-operator"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		LeaderElection:      genericclioptions.NewLeaderElection(),

		ConcurrentSyncs:  50,
		OperatorImage:    "",
		CQLSIngressPort:  0,
		CryptoKeyOptions: DefaultCryptoKeyOptions(),

		ReconcileTraceDir:     "",
		ReconcileTraceAddress: ":8095",
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
	o.CryptoKeyOptions.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ReconcileTraceDir, "reconcile-trace-dir", "", o.ReconcileTraceDir, "Experimental, for tests: directory to record the reconcile trace of the controllers on controller-runtime to, one JSON lines file per namespace. Empty disables recording.")
	cmd.Flags().StringVarP(&o.ReconcileTraceAddress, "reconcile-trace-address", "", o.ReconcileTraceAddress, "Experimental, for tests: address serving the recorded reconcile traces under /reconcile-traces/, when --reconcile-trace-dir is set.")
	// The flags drive an experiment and aren't part of the supported interface.
	for _, name := range []string{"reconcile-trace-dir", "reconcile-trace-address"} {
		err := cmd.Flags().MarkHidden(name)
		if err != nil {
			panic(err)
		}
	}
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

	if len(o.ReconcileTraceDir) != 0 {
		stop, err := o.startReconcileTraceServer(ctx)
		if err != nil {
			return err
		}
		defer stop()
	}

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

	// A nil store must stay a nil interface, not a typed nil.
	var traceRecorder reconciletrace.Recorder
	if o.reconcileTraceStore != nil {
		traceRecorder = o.reconcileTraceStore
	}

	cm, err := controllermanager.New(controllermanager.Options{
		RestConfig:          o.RestConfig,
		Logger:              klog.NewKlogr(),
		KubeClient:          o.kubeClient,
		ScyllaClient:        o.scyllaClient,
		MonitoringClient:    o.monitoringClient,
		ClusterDomainGetter: o.dynamicClusterDomainGetter.GetClusterDomain,
		KeyGenerator:        keyGenerator,
		OperatorImage:       o.OperatorImage,
		CQLSIngressPort:     o.CQLSIngressPort,
		ConcurrentSyncs:     o.ConcurrentSyncs,
		ResyncPeriod:        resyncPeriod,
		TraceRecorder:       traceRecorder,
	})
	if err != nil {
		return fmt.Errorf("can't create controller manager: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// The key generator has to stop with the controller manager, also when the manager fails to start; the deferred
	// cancel runs before the deferred wait.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Go(func() {
		keyGenerator.Run(ctx)
	})

	return cm.Run(ctx)
}

// startReconcileTraceServer creates the reconcile trace store and serves it. Every replica serves, so that a
// follower answers with no traces instead of refusing the connection; only the leader records.
func (o *OperatorOptions) startReconcileTraceServer(ctx context.Context) (func(), error) {
	store, err := reconciletrace.NewNamespacedFileStore(o.ReconcileTraceDir, reconcileTraceMaxBytesPerNamespace)
	if err != nil {
		return nil, fmt.Errorf("can't create reconcile trace store: %w", err)
	}
	o.reconcileTraceStore = store

	server := &http.Server{
		Addr:              o.ReconcileTraceAddress,
		Handler:           store.Handler("/reconcile-traces"),
		ReadHeaderTimeout: 10 * time.Second,
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		klog.InfoS("Serving reconcile traces", "Address", o.ReconcileTraceAddress, "Directory", o.ReconcileTraceDir)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.ErrorS(err, "Reconcile trace server failed")
		}
	})

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			klog.ErrorS(err, "Can't shut down the reconcile trace server")
		}
		wg.Wait()

		err = store.Close()
		if err != nil {
			klog.ErrorS(err, "Can't close the reconcile trace store")
		}
	}, nil
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
