// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1listers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
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
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	remoteinformers "github.com/scylladb/scylla-operator/pkg/remoteclient/informers"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// registerControllers wires every controller of the operator binary. The controllers not migrated to
// controller-runtime yet get their informers from the manager's cache through the Informer bridge, so there is
// one informer per kind in the process regardless of which world a controller lives in.
func (m *Manager) registerControllers(ctx context.Context) error {
	o := m.options
	f := newInformers(ctx, m.mgr.GetCache())

	services := informerFor(f, &corev1.Service{}, corev1listers.NewServiceLister)
	secrets := informerFor(f, &corev1.Secret{}, corev1listers.NewSecretLister)
	configMaps := informerFor(f, &corev1.ConfigMap{}, corev1listers.NewConfigMapLister)
	serviceAccounts := informerFor(f, &corev1.ServiceAccount{}, corev1listers.NewServiceAccountLister)
	endpoints := informerFor(f, &corev1.Endpoints{}, corev1listers.NewEndpointsLister)
	endpointSlices := informerFor(f, &discoveryv1.EndpointSlice{}, discoveryv1listers.NewEndpointSliceLister)
	roleBindings := informerFor(f, &rbacv1.RoleBinding{}, rbacv1listers.NewRoleBindingLister)
	statefulSets := informerFor(f, &appsv1.StatefulSet{}, appsv1listers.NewStatefulSetLister)
	deployments := informerFor(f, &appsv1.Deployment{}, appsv1listers.NewDeploymentLister)
	podDisruptionBudgets := informerFor(f, &policyv1.PodDisruptionBudget{}, policyv1listers.NewPodDisruptionBudgetLister)
	ingresses := informerFor(f, &networkingv1.Ingress{}, networkingv1listers.NewIngressLister)
	jobs := informerFor(f, &batchv1.Job{}, batchv1listers.NewJobLister)

	scyllaClusters := informerFor(f, &scyllav1.ScyllaCluster{}, scyllav1listers.NewScyllaClusterLister)
	scyllaDBDatacenters := informerFor(f, &scyllav1alpha1.ScyllaDBDatacenter{}, scyllav1alpha1listers.NewScyllaDBDatacenterLister)
	scyllaDBClusters := informerFor(f, &scyllav1alpha1.ScyllaDBCluster{}, scyllav1alpha1listers.NewScyllaDBClusterLister)
	scyllaDBMonitorings := informerFor(f, &scyllav1alpha1.ScyllaDBMonitoring{}, scyllav1alpha1listers.NewScyllaDBMonitoringLister)
	scyllaDBManagerClusterRegistrations := informerFor(f, &scyllav1alpha1.ScyllaDBManagerClusterRegistration{}, scyllav1alpha1listers.NewScyllaDBManagerClusterRegistrationLister)
	scyllaDBManagerTasks := informerFor(f, &scyllav1alpha1.ScyllaDBManagerTask{}, scyllav1alpha1listers.NewScyllaDBManagerTaskLister)
	remoteKubernetesClusters := informerFor(f, &scyllav1alpha1.RemoteKubernetesCluster{}, scyllav1alpha1listers.NewRemoteKubernetesClusterLister)
	// ScyllaOperatorConfig is a singleton, so the name-filtered informer the operator used to keep next to the
	// unfiltered one is not needed with a single cache.
	scyllaOperatorConfigs := informerFor(f, &scyllav1alpha1.ScyllaOperatorConfig{}, scyllav1alpha1listers.NewScyllaOperatorConfigLister)

	err := f.Err()
	if err != nil {
		return fmt.Errorf("can't get informers: %w", err)
	}

	remoteKubernetesInformer := remoteinformers.NewSharedInformerFactory[kubernetes.Interface](o.ClusterKubeClient, o.ResyncPeriod)
	remoteScyllaInformer := remoteinformers.NewSharedInformerFactory[scyllaversionedclient.Interface](o.ClusterScyllaClient, o.ResyncPeriod)

	remoteScyllaPodInformer := remoteinformers.NewSharedInformerFactoryWithOptions[kubernetes.Interface](
		o.ClusterKubeClient,
		o.ResyncPeriod,
		remoteinformers.WithTweakListOptions[kubernetes.Interface](
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.SelectorFromSet(naming.ScyllaLabels()).String()
			},
		),
	)

	remoteOperatorManagedResourcesOnlyInformer := remoteinformers.NewSharedInformerFactoryWithOptions[kubernetes.Interface](
		o.ClusterKubeClient,
		o.ResyncPeriod,
		remoteinformers.WithTweakListOptions[kubernetes.Interface](
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.SelectorFromSet(map[string]string{
					naming.KubernetesManagedByLabel: naming.RemoteOperatorAppNameWithDomain,
				}).String()
			},
		),
	)

	m.starters = append(m.starters,
		remoteKubernetesInformer.Start,
		remoteScyllaInformer.Start,
		remoteScyllaPodInformer.Start,
		remoteOperatorManagedResourcesOnlyInformer.Start,
	)

	// The ScyllaDBDatacenter controller is a controller-runtime reconciler: it reads and writes through the manager's
	// client and is run by the manager.
	sdcc := scylladbdatacenter.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbdatacenter-controller"),
		o.OperatorImage,
		o.CQLSIngressPort,
		o.KeyGenerator,
	)
	err = sdcc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: o.ConcurrentSyncs,
	})
	if err != nil {
		return fmt.Errorf("can't set up scylladbdatacenter controller: %w", err)
	}

	scc, err := scyllacluster.NewController(
		o.KubeClient,
		o.ScyllaClient,
		services,
		secrets,
		configMaps,
		serviceAccounts,
		roleBindings,
		statefulSets,
		podDisruptionBudgets,
		ingresses,
		jobs,
		scyllaClusters,
		scyllaDBDatacenters,
		scyllaDBManagerClusterRegistrations,
		scyllaDBManagerTasks,
	)
	if err != nil {
		return fmt.Errorf("can't create scyllacluster controller: %w", err)
	}
	m.addRunnable(scc.Run, o.ConcurrentSyncs)

	opc := orphanedpv.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("orphanedpv-controller"),
	)
	err = opc.SetupWithManager(m.mgr, orphanedpv.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up orphanedpv controller: %w", err)
	}

	ncc := nodeconfig.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("NodeConfig-controller"),
		o.OperatorImage,
	)
	err = ncc.SetupWithManager(m.mgr, nodeconfig.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up nodeconfig controller: %w", err)
	}

	ncpc := nodeconfigpod.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("NodeConfigCM-controller"),
	)
	err = ncpc.SetupWithManager(m.mgr, nodeconfigpod.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up nodeconfigpod controller: %w", err)
	}

	socc := scyllaoperatorconfig.NewController(
		m.mgr.GetClient(),
		m.mgr.GetEventRecorderFor("scyllaoperatorconfig-controller"),
		o.ClusterDomainGetter,
	)
	err = socc.SetupWithManager(m.mgr, scyllaoperatorconfig.ControllerOptions())
	if err != nil {
		return fmt.Errorf("can't set up scyllaoperatorconfig controller: %w", err)
	}

	if o.MonitoringCRDsInstalled {
		prometheuses := informerFor(f, &monitoringv1.Prometheus{}, monitoringv1listers.NewPrometheusLister)
		prometheusRules := informerFor(f, &monitoringv1.PrometheusRule{}, monitoringv1listers.NewPrometheusRuleLister)
		serviceMonitors := informerFor(f, &monitoringv1.ServiceMonitor{}, monitoringv1listers.NewServiceMonitorLister)
		err = f.Err()
		if err != nil {
			return fmt.Errorf("can't get monitoring informers: %w", err)
		}

		mc, err := scylladbmonitoring.NewController(
			o.KubeClient,
			o.ScyllaClient.ScyllaV1alpha1(),
			o.MonitoringClient.MonitoringV1(),
			scyllaOperatorConfigs,
			configMaps,
			secrets,
			services,
			serviceAccounts,
			roleBindings,
			podDisruptionBudgets,
			deployments,
			ingresses,
			scyllaDBMonitorings,
			prometheuses,
			prometheusRules,
			serviceMonitors,
			o.KeyGenerator,
		)
		if err != nil {
			return fmt.Errorf("can't create scylladbmonitoring controller: %w", err)
		}
		m.addRunnable(mc.Run, o.ConcurrentSyncs)
	}

	rkcc, err := remotekubernetescluster.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		remoteKubernetesClusters,
		scyllaDBClusters,
		secrets,
		[]remoteclient.DynamicClusterInterface{
			o.ClusterKubeClient,
			o.ClusterScyllaClient,
			remoteKubernetesInformer,
			remoteScyllaInformer,
			remoteScyllaPodInformer,
		},
		o.ClusterKubeClient,
		o.ClusterScyllaClient,
	)
	if err != nil {
		return fmt.Errorf("can't create RemoteKubernetesCluster controller: %w", err)
	}
	m.addRunnable(rkcc.Run, o.ConcurrentSyncs)

	sdbcc, err := scylladbcluster.NewController(
		o.KubeClient,
		o.ScyllaClient,
		o.ClusterKubeClient,
		o.ClusterScyllaClient,
		scyllaDBClusters,
		scyllaOperatorConfigs,
		configMaps,
		secrets,
		services,
		endpointSlices,
		endpoints,
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
	m.addRunnable(sdbcc.Run, o.ConcurrentSyncs)

	gsmc := globalscylladbmanager.NewController(
		m.mgr.GetClient(),
		m.mgr.GetEventRecorderFor("globalscylladbmanager-controller"),
	)
	err = gsmc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return fmt.Errorf("can't set up global ScyllaDB Manager controller: %w", err)
	}

	smcrc := scylladbmanagerclusterregistration.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbmanagerclusterregistration-controller"),
	)
	err = smcrc.SetupWithManager(m.mgr, scylladbmanagerclusterregistration.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up ScyllaDBManagerClusterRegistration controller: %w", err)
	}

	smtc := scylladbmanagertask.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbmanagertask-controller"),
	)
	err = smtc.SetupWithManager(m.mgr, scylladbmanagertask.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up ScyllaDBManagerTask controller: %w", err)
	}

	return nil
}

func (m *Manager) addRunnable(run func(ctx context.Context, workers int), workers int) {
	m.runnables = append(m.runnables, func(ctx context.Context) {
		run(ctx, workers)
	})
}
