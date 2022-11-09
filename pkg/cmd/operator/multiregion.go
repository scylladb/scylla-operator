package operator

import (
	"context"
	"fmt"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/remotekubeclusterconfig"
	remotekubeclusterconfigprotection "github.com/scylladb/scylla-operator/pkg/controller/remotekubeclusterconfig/protection"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	scyllaclusterprotection "github.com/scylladb/scylla-operator/pkg/controller/scyllacluster/protection"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	multiregiondynamicclient "github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	multiregiondynamicinformers "github.com/scylladb/scylla-operator/pkg/remotedynamicclient/informers"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type MultiRegionOperatorOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	genericclioptions.LeaderElection

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface

	localDynamicClient  dynamic.Interface
	remoteDynamicClient *multiregiondynamicclient.RemoteDynamicClient

	ConcurrentSyncs int
}

func NewMultiRegionOperatorOptions(streams genericclioptions.IOStreams) *MultiRegionOperatorOptions {
	return &MultiRegionOperatorOptions{
		ClientConfig:        genericclioptions.NewClientConfig("scylla-operator-multiregion"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		LeaderElection:      genericclioptions.NewLeaderElection(),

		ConcurrentSyncs: 5,
	}
}

func NewMultiRegionOperatorCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMultiRegionOperatorOptions(streams)

	cmd := &cobra.Command{
		Use:   "operator-multiregion",
		Short: "Run the scylla multi region operator.",
		Long:  `Run the scylla multi region operator.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
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

	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)
	o.LeaderElection.AddFlags(cmd)

	cmd.Flags().IntVarP(&o.ConcurrentSyncs, "concurrent-syncs", "", o.ConcurrentSyncs, "The number of ScyllaCluster objects that are allowed to sync concurrently.")

	return cmd
}

func (o *MultiRegionOperatorOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.LeaderElection.Validate())

	return apierrors.NewAggregate(errs)
}

func (o *MultiRegionOperatorOptions) Complete() error {
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

	o.localDynamicClient, err = dynamic.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build dynamic clientset: %w", err)
	}

	o.remoteDynamicClient = multiregiondynamicclient.NewRemoteDynamicClient()

	return nil
}

func (o *MultiRegionOperatorOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	// Lock names cannot be changed, because it may lead to two leaders during rolling upgrades.
	const lockName = "scylla-multi-region-operator-lock"

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

func (o *MultiRegionOperatorOptions) run(ctx context.Context, streams genericclioptions.IOStreams) error {
	kubeInformers := informers.NewSharedInformerFactory(o.kubeClient, resyncPeriod)
	scyllaInformers := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)

	remoteDynamicInformer := multiregiondynamicinformers.NewSharedInformerFactory(*scheme, o.remoteDynamicClient, resyncPeriod)

	namespaceGVR := corev1.SchemeGroupVersion.WithResource("namespaces")
	namespaceGVK := corev1.SchemeGroupVersion.WithKind("Namespace")

	scyllaDatacenterGVR := v1alpha1.GroupVersion.WithResource("scylladatacenters")
	scyllaDatacenterGVK := v1alpha1.GroupVersion.WithKind("ScyllaDatacenter")

	cachedClient := cacheddiscovery.NewMemCacheClient(o.kubeClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	// TODO: reset restMapper periodically?

	rkccc, err := remotekubeclusterconfig.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaInformers.Scylla().V1alpha1().RemoteKubeClusterConfigs(),
		kubeInformers.Core().V1().Secrets(),
		[]multiregiondynamicinformers.DynamicRegion{o.remoteDynamicClient, remoteDynamicInformer},
	)
	if err != nil {
		return err
	}

	rkccpc, err := remotekubeclusterconfigprotection.NewController(
		o.kubeClient,
		o.scyllaClient,
		scyllaInformers.Scylla().V2alpha1().ScyllaClusters(),
		scyllaInformers.Scylla().V1alpha1().RemoteKubeClusterConfigs(),
	)
	if err != nil {
		return err
	}

	scc, err := scyllacluster.NewController(
		o.kubeClient,
		o.scyllaClient,
		scyllaInformers.Scylla().V2alpha1().ScyllaClusters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDatacenters(),
		o.remoteDynamicClient,
		remoteDynamicInformer.ForResource(scyllaDatacenterGVR, scyllaDatacenterGVK),
		remoteDynamicInformer.ForResource(namespaceGVR, namespaceGVK),
	)
	if err != nil {
		return err
	}

	scpc, err := scyllaclusterprotection.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV2alpha1(),
		scyllaInformers.Scylla().V2alpha1().ScyllaClusters(),
		o.remoteDynamicClient,
		map[schema.GroupVersionResource]multiregiondynamicinformers.GenericRemoteInformer{
			scyllaDatacenterGVR: remoteDynamicInformer.ForResource(scyllaDatacenterGVR, scyllaDatacenterGVK),
			namespaceGVR:        remoteDynamicInformer.ForResource(namespaceGVR, namespaceGVK),
		},
		restMapper,
	)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	defer wg.Wait()

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
		remoteDynamicInformer.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rkccc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rkccpc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scpc.Run(ctx, o.ConcurrentSyncs)
	}()

	<-ctx.Done()

	return nil
}
