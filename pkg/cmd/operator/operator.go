package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"

	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/clusterdomain"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
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
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/leaderelection"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	remoteinformers "github.com/scylladb/scylla-operator/pkg/remoteclient/informers"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
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

	ConcurrentSyncs  int
	OperatorImage    string
	CQLSIngressPort  int
	CryptoKeyOptions CryptoKeyOptions
}

func NewOperatorOptions(streams genericclioptions.IOStreams) *OperatorOptions {
	return &OperatorOptions{
		ClientConfig:        genericclioptions.NewClientConfig("scylla-operator"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		LeaderElection:      genericclioptions.NewLeaderElection(),

		ConcurrentSyncs:  50,
		OperatorImage:    "",
		CQLSIngressPort:  0,
		CryptoKeyOptions: NewCryptoKeyOptions(),
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
		keyGenerator,
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
		kubeInformers.Core().V1().ConfigMaps(),
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

	var mc *scylladbmonitoring.Controller
	if monitoringCRDsInstalled {
		mc, err = scylladbmonitoring.NewController(
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
			keyGenerator,
		)
		if err != nil {
			return fmt.Errorf("can't create scylladbmonitoring controller: %w", err)
		}
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
		remoteScyllaInformer.ForResource(&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}, remoteinformers.ClusterListWatch[scyllaversionedclient.Interface]{
			ListFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.ListFunc {
				return func(options metav1.ListOptions) (runtime.Object, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().ScyllaDBDatacenterNodesStatusReports(ns).List(ctx, options)
				}
			},
			WatchFunc: func(client remoteclient.ClusterClientInterface[scyllaversionedclient.Interface], cluster, ns string) cache.WatchFunc {
				return func(options metav1.ListOptions) (watch.Interface, error) {
					clusterClient, err := client.Cluster(cluster)
					if err != nil {
						return nil, err
					}
					return clusterClient.ScyllaV1alpha1().ScyllaDBDatacenterNodesStatusReports(ns).Watch(ctx, options)
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
		keyGenerator.Run(ctx)
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

	if monitoringCRDsInstalled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			monitoringInformers.Start(ctx.Done())
		}()
	}

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

	if monitoringCRDsInstalled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mc.Run(ctx, o.ConcurrentSyncs)
		}()
	}

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
