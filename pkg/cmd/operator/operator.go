package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/clusterdomain"
	"github.com/scylladb/scylla-operator/pkg/controller/globalscylladbmanager"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/pkg/controller/remotekubernetescluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagerclusterregistration"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagertask"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	remoteinformers "github.com/scylladb/scylla-operator/pkg/remoteclient/informers"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	// RFC 5702: The key size of RSA/SHA-512 keys MUST NOT be less than 1024 bits and MUST NOT be more than 4096 bits.
	// NIST Special Publication 800-57 Part 3 (DOI: 10.6028) recommends a minimum of 2048-bit keys for RSA.
	rsaKeySizeMin = 2048
	rsaKeySizeMax = 4096

	cryptoKeyBufferSizeMaxFlagKey = "crypto-key-buffer-size-max"
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

	ConcurrentSyncs int
	OperatorImage   string
	CQLSIngressPort int

	CryptoKeySize          int
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

		CryptoKeySize:          4096,
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
	cmd.Flags().IntVarP(&o.CryptoKeySize, "crypto-key-size", "", o.CryptoKeySize, "The size of the RSA key to use, in bits.")
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

	if o.CryptoKeySize < rsaKeySizeMin {
		errs = append(errs, fmt.Errorf("crypto-key-size must not be less than %d", rsaKeySizeMin))
	}

	if o.CryptoKeySize > rsaKeySizeMax {
		errs = append(errs, fmt.Errorf("crypto-key-size must not be more than %d", rsaKeySizeMax))
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
	if ok, err := isStandaloneScyllaDBManagerControllerDeployed(ctx, o.kubeClient.AppsV1()); err != nil {
		return fmt.Errorf("can't check for standalone ScyllaDB Manager controller presence: %w", err)
	} else if ok {
		return fmt.Errorf("Standalone ScyllaDB Manager controller Deployment %q should not be running alongside Scylla Operator. Delete it before starting Scylla Operator.", naming.ManualRef(naming.ScyllaManagerNamespace, naming.StandaloneScyllaDBManagerControllerName))
	}

	rsaKeyGenerator, err := crypto.NewRSAKeyGenerator(
		o.CryptoKeyBufferSizeMin,
		o.CryptoKeyBufferSizeMax,
		o.CryptoKeySize,
		o.CryptoKeyBufferDelay,
	)
	if err != nil {
		return fmt.Errorf("can't create rsa key generator: %w", err)
	}
	defer rsaKeyGenerator.Close()

	kubeInformers := informers.NewSharedInformerFactory(o.kubeClient, resyncPeriod)
	scyllaInformers := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)

	remoteKubernetesInformer := remoteinformers.NewSharedInformerFactory[kubernetes.Interface](&o.clusterKubeClient, resyncPeriod)
	remoteScyllaInformer := remoteinformers.NewSharedInformerFactory[scyllaversionedclient.Interface](&o.clusterScyllaClient, resyncPeriod)

	remoteScyllaPodInformer := remoteinformers.NewSharedInformerFactoryWithOptions[kubernetes.Interface](
		&o.clusterKubeClient,
		resyncPeriod,
		remoteinformers.WithTweakListOptions[kubernetes.Interface](
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.SelectorFromSet(naming.ScyllaLabels()).String()
			},
		),
	)

	remoteOperatorManagedResourcesOnlyInformer := remoteinformers.NewSharedInformerFactoryWithOptions[kubernetes.Interface](
		&o.clusterKubeClient,
		resyncPeriod,
		remoteinformers.WithTweakListOptions[kubernetes.Interface](
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.SelectorFromSet(map[string]string{
					naming.KubernetesManagedByLabel: naming.RemoteOperatorAppNameWithDomain,
				}).String()
			},
		),
	)

	scyllaOperatorConfigInformers := scyllainformers.NewSharedInformerFactoryWithOptions(o.scyllaClient, resyncPeriod, scyllainformers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", naming.SingletonName).String()
		},
	))

	monitoringInformers := monitoringinformers.NewSharedInformerFactory(o.monitoringClient, resyncPeriod)

	sdcc, err := scylladbdatacenter.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
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
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenterNodesStatusReports(),
		o.OperatorImage,
		o.CQLSIngressPort,
		rsaKeyGenerator,
	)
	if err != nil {
		return fmt.Errorf("can't create scylladbdatacenter controller: %w", err)
	}

	scc, err := scyllacluster.NewController(
		o.kubeClient,
		o.scyllaClient,
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Apps().V1().StatefulSets(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Networking().V1().Ingresses(),
		kubeInformers.Batch().V1().Jobs(),
		scyllaInformers.Scylla().V1().ScyllaClusters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerTasks(),
	)
	if err != nil {
		return fmt.Errorf("can't create scyllacluster controller: %w", err)
	}

	opc, err := orphanedpv.NewController(
		o.kubeClient,
		kubeInformers.Core().V1().PersistentVolumes(),
		kubeInformers.Core().V1().PersistentVolumeClaims(),
		kubeInformers.Core().V1().Nodes(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
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
		kubeInformers.Rbac().V1().Roles(),
		kubeInformers.Rbac().V1().RoleBindings(),
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
		o.dynamicClusterDomainGetter.GetClusterDomain,
	)
	if err != nil {
		return fmt.Errorf("can't create scyllaoperatorconfig controller: %w", err)
	}

	mc, err := scylladbmonitoring.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		o.monitoringClient.MonitoringV1(),
		scyllaInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
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

	rkcc, err := remotekubernetescluster.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaInformers.Scylla().V1alpha1().RemoteKubernetesClusters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBClusters(),
		kubeInformers.Core().V1().Secrets(),
		[]remoteclient.DynamicClusterInterface{
			&o.clusterKubeClient,
			&o.clusterScyllaClient,
			remoteKubernetesInformer,
			remoteScyllaInformer,
			remoteScyllaPodInformer,
		},
		&o.clusterKubeClient,
		&o.clusterScyllaClient,
	)
	if err != nil {
		return fmt.Errorf("can't create RemoteKubernetesCluster controller: %w", err)
	}

	sdbcc, err := scylladbcluster.NewController(
		o.kubeClient,
		o.scyllaClient,
		&o.clusterKubeClient,
		&o.clusterScyllaClient,
		scyllaInformers.Scylla().V1alpha1().ScyllaDBClusters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Discovery().V1().EndpointSlices(),
		kubeInformers.Core().V1().Endpoints(),
		remoteScyllaInformer.ForResource(&scyllav1alpha1.RemoteOwner{}, remoteinformers.ClusterListWatch[scyllaversionedclient.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().RemoteOwners(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().RemoteOwners(ns).Watch(ctx, options)
				}
			},
		}),
		remoteScyllaInformer.ForResource(&scyllav1alpha1.ScyllaDBDatacenter{}, remoteinformers.ClusterListWatch[scyllaversionedclient.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(ns).Watch(ctx, options)
				}
			},
		}),
		remoteKubernetesInformer.ForResource(&corev1.Namespace{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Namespaces().List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Namespaces().Watch(ctx, options)
				}
			},
		}),
		remoteKubernetesInformer.ForResource(&corev1.Service{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Services(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Services(ns).Watch(ctx, options)
				}
			},
		}),
		remoteKubernetesInformer.ForResource(&discoveryv1.EndpointSlice{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.DiscoveryV1().EndpointSlices(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.DiscoveryV1().EndpointSlices(ns).Watch(ctx, options)
				}
			},
		}),
		remoteKubernetesInformer.ForResource(&corev1.Endpoints{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Endpoints(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Endpoints(ns).Watch(ctx, options)
				}
			},
		}),
		remoteScyllaPodInformer.ForResource(&corev1.Pod{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Pods(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Pods(ns).Watch(ctx, options)
				}
			},
		}),
		remoteOperatorManagedResourcesOnlyInformer.ForResource(&corev1.ConfigMap{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().ConfigMaps(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().ConfigMaps(ns).Watch(ctx, options)
				}
			},
		}),
		remoteOperatorManagedResourcesOnlyInformer.ForResource(&corev1.Secret{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Secrets(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.CoreV1().Secrets(ns).Watch(ctx, options)
				}
			},
		}),
	)
	if err != nil {
		return fmt.Errorf("can't create ScyllaDBCluster controller: %w", err)
	}

	gsmc, err := globalscylladbmanager.NewController(
		o.kubeClient,
		o.scyllaClient,
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBClusters(),
		kubeInformers.Core().V1().Namespaces(),
	)
	if err != nil {
		return fmt.Errorf("can't create global ScyllaDB Manager controller: %w", err)
	}

	smcrc, err := scylladbmanagerclusterregistration.NewController(
		o.kubeClient,
		o.scyllaClient,
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBClusters(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().Namespaces(),
	)
	if err != nil {
		return fmt.Errorf("can't create ScyllaDBManagerClusterRegistration controller: %w", err)
	}

	smtc, err := scylladbmanagertask.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerTasks(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
	)
	if err != nil {
		return fmt.Errorf("can't create ScyllaDBManagerTask controller: %w", err)
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
		remoteKubernetesInformer.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		remoteScyllaInformer.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		remoteScyllaPodInformer.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		remoteOperatorManagedResourcesOnlyInformer.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sdcc.Run(ctx, o.ConcurrentSyncs)
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		rkcc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sdbcc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		gsmc.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		smcrc.Run(ctx, o.ConcurrentSyncs)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		smtc.Run(ctx, o.ConcurrentSyncs)
	}()

	<-ctx.Done()

	return nil
}

func isStandaloneScyllaDBManagerControllerDeployed(ctx context.Context, appsV1Client appsv1client.AppsV1Interface) (bool, error) {
	_, err := appsV1Client.Deployments(naming.ScyllaManagerNamespace).Get(ctx, naming.StandaloneScyllaDBManagerControllerName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("can't get Deployment %q: %w", naming.ManualRef(naming.ScyllaManagerNamespace, naming.StandaloneScyllaDBManagerControllerName), err)
		}

		return false, nil
	}

	return true, nil
}
