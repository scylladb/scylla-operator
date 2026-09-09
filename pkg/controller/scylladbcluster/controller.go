// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	"github.com/scylladb/scylla-operator/pkg/remotecluster"
	"github.com/scylladb/scylla-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	remoteControllerGVK = scyllav1alpha1.GroupVersion.WithKind("RemoteOwner")
)

const (
	ControllerName = "ScyllaDBClusterController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scylladbcluster"
)

// Controller reconciles ScyllaDBClusters: it mirrors each datacenter into its remote Kubernetes cluster and the remote
// endpoints back. Local objects are read and written through the manager's client; remote ones through the client of
// the remote cluster, both with read-your-writes consistency.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption, and
	// the last check before a finalizer is removed.
	apiReader client.Reader

	remoteClusters *remotecluster.Set

	eventRecorder record.EventRecorder
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
	remoteClusters *remotecluster.Set,
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		remoteClusters: remoteClusters,

		eventRecorder: eventRecorder,
	}
}

// RemoteCacheOptions restricts what the cache of every remote cluster watches to what this controller mirrors: all
// of the kinds below, with the Pods limited to ScyllaDB's and the ConfigMaps and Secrets to the ones the operator
// manages remotely.
func RemoteCacheOptions() cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Label: labels.SelectorFromSet(naming.ScyllaLabels()),
			},
			&corev1.ConfigMap{}: {
				Label: labels.SelectorFromSet(map[string]string{
					naming.KubernetesManagedByLabel: naming.RemoteOperatorAppNameWithDomain,
				}),
			},
			&corev1.Secret{}: {
				Label: labels.SelectorFromSet(map[string]string{
					naming.KubernetesManagedByLabel: naming.RemoteOperatorAppNameWithDomain,
				}),
			},
		},
	}
}

// SetupWithManager registers the controller with the manager. The local objects it owns re-run the sync through
// their owner. The remote objects re-run the sync of the ScyllaDBCluster named in their labels; their watches are
// attached to every remote cluster the set has and gets, and stop with the cluster.
func (scc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cacheReader := mgr.GetCache()

	ctrl, err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBCluster{}).
		Owns(&corev1.Service{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Owns(&corev1.Endpoints{}).
		Owns(&corev1.Secret{}).
		WithOptions(options).
		Build(scc)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	parent := handler.EnqueueRequestsFromMapFunc(mapToParentScyllaDBCluster(cacheReader))
	remoteOwner := handler.EnqueueRequestsFromMapFunc(mapToRemoteOwnerScyllaDBCluster(cacheReader))

	return scc.remoteClusters.OnCluster(func(ctx context.Context, name string, c cluster.Cluster) error {
		remoteCache := c.GetCache()
		var errs []error
		for _, src := range []source.Source{
			source.Kind[client.Object](remoteCache, &scyllav1alpha1.RemoteOwner{}, remoteOwner),
			source.Kind[client.Object](remoteCache, &corev1.Namespace{}, parent),
			source.Kind[client.Object](remoteCache, &scyllav1alpha1.ScyllaDBDatacenter{}, parent),
			source.Kind[client.Object](remoteCache, &corev1.Service{}, parent),
			source.Kind[client.Object](remoteCache, &discoveryv1.EndpointSlice{}, parent),
			source.Kind[client.Object](remoteCache, &corev1.Endpoints{}, parent),
			source.Kind[client.Object](remoteCache, &corev1.Pod{}, parent),
			source.Kind[client.Object](remoteCache, &corev1.ConfigMap{}, parent),
			source.Kind[client.Object](remoteCache, &corev1.Secret{}, parent),
			source.Kind[client.Object](remoteCache, &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}, parent),
		} {
			// The source runs under the controller's context and can't be removed; it goes quiet when the remote
			// cluster's cache is stopped with the cluster.
			err := ctrl.Watch(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't watch remote cluster %q: %w", name, err))
			}
		}

		return apimachineryutilerrors.NewAggregate(errs)
	})
}

func (scc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rq := &controllertools.Requeue{}
	err := scc.sync(ctx, req.NamespacedName, rq)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		return rq.Result(), nil

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", req.NamespacedName, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", req.NamespacedName, "Error", err)
	}

	return reconcile.Result{}, fmt.Errorf("syncing key '%v' failed: %w", req.NamespacedName, err)
}

// remoteCluster returns the cluster of the datacenter's RemoteKubernetesCluster.
func (scc *Controller) remoteCluster(name string) (cluster.Cluster, error) {
	c, err := scc.remoteClusters.Cluster(name)
	if err != nil {
		return nil, fmt.Errorf("can't get cluster %q: %w", name, err)
	}

	return c, nil
}

// The remote listers below read the objects of a kind in any remote cluster through that cluster's client, under the
// context of the reconciliation, in the shape the mirroring functions take.

func (scc *Controller) remoteRemoteOwnerLister(ctx context.Context) remotelister.GenericClusterLister[scyllav1alpha1listers.RemoteOwnerLister] {
	return remotecluster.NewClusterLister(ctx, scc.remoteClusters, ctrlclient.NewRemoteOwnerLister)
}

func (scc *Controller) remoteNamespaceLister(ctx context.Context) remotelister.GenericClusterLister[corev1listers.NamespaceLister] {
	return remotecluster.NewClusterLister(ctx, scc.remoteClusters, ctrlclient.NewNamespaceLister)
}

func (scc *Controller) remoteServiceLister(ctx context.Context) remotelister.GenericClusterLister[corev1listers.ServiceLister] {
	return remotecluster.NewClusterLister(ctx, scc.remoteClusters, ctrlclient.NewServiceLister)
}

func (scc *Controller) remotePodLister(ctx context.Context) remotelister.GenericClusterLister[corev1listers.PodLister] {
	return remotecluster.NewClusterLister(ctx, scc.remoteClusters, ctrlclient.NewPodLister)
}

func (scc *Controller) remoteScyllaDBDatacenterNodesStatusReportLister(ctx context.Context) remotelister.GenericClusterLister[scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportLister] {
	return remotecluster.NewClusterLister(ctx, scc.remoteClusters, ctrlclient.NewScyllaDBDatacenterNodesStatusReportLister)
}

func requestFor(namespace, name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// mapToParentScyllaDBCluster enqueues the ScyllaDBCluster a remote object names in its parent labels, if it exists.
func mapToParentScyllaDBCluster(cacheReader client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		objLabels := obj.GetLabels()
		parentName, parentNamespace := objLabels[naming.ParentClusterNameLabel], objLabels[naming.ParentClusterNamespaceLabel]
		if len(parentName) == 0 || len(parentNamespace) == 0 {
			klog.V(5).InfoS("got event about object not having parent labels", "Object", klog.KObj(obj))
			return nil
		}

		sc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBCluster](ctx, cacheReader, parentNamespace, parentName)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaDBCluster for object %#v", obj))
			return nil
		}

		klog.V(4).InfoS("Enqueuing parent", resource.GetObjectGVKOrUnknown(obj).Kind, klog.KObj(obj), "ScyllaDBCluster", klog.KObj(sc))
		return []reconcile.Request{requestFor(sc.Namespace, sc.Name)}
	}
}

// mapToRemoteOwnerScyllaDBCluster enqueues the ScyllaDBCluster a remote RemoteOwner names in its remote owner labels,
// if it exists.
func mapToRemoteOwnerScyllaDBCluster(cacheReader client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		objLabels := obj.GetLabels()
		name, namespace, gvr := objLabels[naming.RemoteOwnerNameLabel], objLabels[naming.RemoteOwnerNamespaceLabel], objLabels[naming.RemoteOwnerGVR]
		if len(name) == 0 || len(namespace) == 0 {
			klog.V(5).InfoS("got event about object not having remoteOwner labels", "Object", klog.KObj(obj))
			return nil
		}

		if gvr != naming.GroupVersionResourceToLabelValue(scyllav1alpha1.GroupVersion.WithResource("scylladbclusters")) {
			return nil
		}

		sc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBCluster](ctx, cacheReader, namespace, name)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaDBCluster for object %#v", obj))
			return nil
		}

		klog.V(4).InfoS("Enqueuing parent", resource.GetObjectGVKOrUnknown(obj).Kind, klog.KObj(obj), "ScyllaDBCluster", klog.KObj(sc))
		return []reconcile.Request{requestFor(sc.Namespace, sc.Name)}
	}
}
