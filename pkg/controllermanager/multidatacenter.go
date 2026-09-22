// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/remotekubernetescluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	remoteinformers "github.com/scylladb/scylla-operator/pkg/remoteclient/informers"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

// registerMultiDatacenterControllers wires the RemoteKubernetesCluster and ScyllaDBCluster controllers, the two that
// reach into remote Kubernetes clusters. They watch those clusters through the remote informer factories, which are
// not part of the manager's cache and are started next to it. Everything the multi-datacenter support takes from the
// local cache is requested here, so that dropping the support means deleting this file and its one call.
func (m *Manager) registerMultiDatacenterControllers(ctx context.Context) error {
	o := m.options
	c := m.mgr.GetCache()
	errs := &errorCollector{}

	secrets := informerFor(ctx, c, errs, &corev1.Secret{}, corev1listers.NewSecretLister)
	configMaps := informerFor(ctx, c, errs, &corev1.ConfigMap{}, corev1listers.NewConfigMapLister)
	services := informerFor(ctx, c, errs, &corev1.Service{}, corev1listers.NewServiceLister)
	endpoints := informerFor(ctx, c, errs, &corev1.Endpoints{}, corev1listers.NewEndpointsLister)
	endpointSlices := informerFor(ctx, c, errs, &discoveryv1.EndpointSlice{}, discoveryv1listers.NewEndpointSliceLister)
	scyllaDBClusters := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBCluster{}, scyllav1alpha1listers.NewScyllaDBClusterLister)
	remoteKubernetesClusters := informerFor(ctx, c, errs, &scyllav1alpha1.RemoteKubernetesCluster{}, scyllav1alpha1listers.NewRemoteKubernetesClusterLister)
	scyllaOperatorConfigs := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaOperatorConfig{}, scyllav1alpha1listers.NewScyllaOperatorConfigLister)

	err := errs.Err()
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

	return nil
}
