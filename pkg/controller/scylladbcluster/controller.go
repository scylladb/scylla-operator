package scylladbcluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	remoteinformers "github.com/scylladb/scylla-operator/pkg/remoteclient/informers"
	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	discoveryv1informers "k8s.io/client-go/informers/discovery/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaDBClusterController"
)

var (
	keyFunc             = cache.DeletionHandlingMetaNamespaceKeyFunc
	remoteControllerGVK = scyllav1alpha1.GroupVersion.WithKind("RemoteOwner")
)

type Controller struct {
	kubeClient         kubernetes.Interface
	scyllaClient       scyllaclient.Interface
	kubeRemoteClient   remoteclient.ClusterClientInterface[kubernetes.Interface]
	scyllaRemoteClient remoteclient.ClusterClientInterface[scyllaclient.Interface]

	scyllaDBClusterLister      scyllav1alpha1listers.ScyllaDBClusterLister
	scyllaOperatorConfigLister scyllav1alpha1listers.ScyllaOperatorConfigLister
	configMapLister            corev1listers.ConfigMapLister
	secretLister               corev1listers.SecretLister
	serviceLister              corev1listers.ServiceLister
	endpointSliceLister        discoveryv1listers.EndpointSliceLister
	endpointsLister            corev1listers.EndpointsLister

	remoteRemoteOwnerLister        remotelister.GenericClusterLister[scyllav1alpha1listers.RemoteOwnerLister]
	remoteScyllaDBDatacenterLister remotelister.GenericClusterLister[scyllav1alpha1listers.ScyllaDBDatacenterLister]
	remoteNamespaceLister          remotelister.GenericClusterLister[corev1listers.NamespaceLister]
	remoteServiceLister            remotelister.GenericClusterLister[corev1listers.ServiceLister]
	remoteEndpointSliceLister      remotelister.GenericClusterLister[discoveryv1listers.EndpointSliceLister]
	remoteEndpointsLister          remotelister.GenericClusterLister[corev1listers.EndpointsLister]
	remotePodLister                remotelister.GenericClusterLister[corev1listers.PodLister]
	remoteConfigMapLister          remotelister.GenericClusterLister[corev1listers.ConfigMapLister]
	remoteSecretLister             remotelister.GenericClusterLister[corev1listers.SecretLister]

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.TypedRateLimitingInterface[string]
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBCluster]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	kubeRemoteClient remoteclient.ClusterClientInterface[kubernetes.Interface],
	scyllaRemoteClient remoteclient.ClusterClientInterface[scyllaclient.Interface],
	scyllaDBClusterInformer scyllav1alpha1informers.ScyllaDBClusterInformer,
	scyllaOperatorConfigInformer scyllav1alpha1informers.ScyllaOperatorConfigInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	secretInformer corev1informers.SecretInformer,
	serviceInformer corev1informers.ServiceInformer,
	endpointSliceInformer discoveryv1informers.EndpointSliceInformer,
	endpointsInformer corev1informers.EndpointsInformer,
	remoteRemoteOwnerInformer remoteinformers.GenericClusterInformer,
	remoteScyllaDBDatacenterInformer remoteinformers.GenericClusterInformer,
	remoteNamespaceInformer remoteinformers.GenericClusterInformer,
	remoteServiceInformer remoteinformers.GenericClusterInformer,
	remoteEndpointSliceInformer remoteinformers.GenericClusterInformer,
	remoteEndpointsInformer remoteinformers.GenericClusterInformer,
	remotePodInformer remoteinformers.GenericClusterInformer,
	remoteConfigMapInformer remoteinformers.GenericClusterInformer,
	remoteSecretInformer remoteinformers.GenericClusterInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	scc := &Controller{
		kubeClient:         kubeClient,
		scyllaClient:       scyllaClient,
		kubeRemoteClient:   kubeRemoteClient,
		scyllaRemoteClient: scyllaRemoteClient,

		scyllaDBClusterLister:      scyllaDBClusterInformer.Lister(),
		scyllaOperatorConfigLister: scyllaOperatorConfigInformer.Lister(),
		configMapLister:            configMapInformer.Lister(),
		secretLister:               secretInformer.Lister(),
		serviceLister:              serviceInformer.Lister(),
		endpointSliceLister:        endpointSliceInformer.Lister(),
		endpointsLister:            endpointsInformer.Lister(),

		remoteRemoteOwnerLister:        remotelister.NewClusterLister(scyllav1alpha1listers.NewRemoteOwnerLister, remoteRemoteOwnerInformer.Indexer().Cluster),
		remoteScyllaDBDatacenterLister: remotelister.NewClusterLister(scyllav1alpha1listers.NewScyllaDBDatacenterLister, remoteScyllaDBDatacenterInformer.Indexer().Cluster),
		remoteNamespaceLister:          remotelister.NewClusterLister(corev1listers.NewNamespaceLister, remoteNamespaceInformer.Indexer().Cluster),
		remoteServiceLister:            remotelister.NewClusterLister(corev1listers.NewServiceLister, remoteServiceInformer.Indexer().Cluster),
		remoteEndpointSliceLister:      remotelister.NewClusterLister(discoveryv1listers.NewEndpointSliceLister, remoteEndpointSliceInformer.Indexer().Cluster),
		remoteEndpointsLister:          remotelister.NewClusterLister(corev1listers.NewEndpointsLister, remoteEndpointsInformer.Indexer().Cluster),
		remotePodLister:                remotelister.NewClusterLister(corev1listers.NewPodLister, remotePodInformer.Indexer().Cluster),
		remoteConfigMapLister:          remotelister.NewClusterLister(corev1listers.NewConfigMapLister, remoteConfigMapInformer.Indexer().Cluster),
		remoteSecretLister:             remotelister.NewClusterLister(corev1listers.NewSecretLister, remoteSecretInformer.Indexer().Cluster),

		cachesToSync: []cache.InformerSynced{
			scyllaDBClusterInformer.Informer().HasSynced,
			scyllaOperatorConfigInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			serviceInformer.Informer().HasSynced,
			endpointSliceInformer.Informer().HasSynced,
			endpointsInformer.Informer().HasSynced,
			remoteRemoteOwnerInformer.Informer().HasSynced,
			remoteScyllaDBDatacenterInformer.Informer().HasSynced,
			remoteNamespaceInformer.Informer().HasSynced,
			remoteServiceInformer.Informer().HasSynced,
			remoteEndpointSliceInformer.Informer().HasSynced,
			remoteEndpointsInformer.Informer().HasSynced,
			remotePodInformer.Informer().HasSynced,
			remoteConfigMapInformer.Informer().HasSynced,
			remoteSecretInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbcluster-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "scylladbcluster",
			},
		),
	}

	var err error
	scc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBCluster](
		scc.queue,
		keyFunc,
		scheme.Scheme,
		scyllav1alpha1.ScyllaDBClusterGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.ScyllaDBCluster]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.ScyllaDBCluster, error) {
				return scc.scyllaDBClusterLister.ScyllaDBClusters(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.ScyllaDBCluster, err error) {
				return scc.scyllaDBClusterLister.ScyllaDBClusters(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	var errs []error
	_, err = scyllaDBClusterInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addScyllaDBCluster,
			UpdateFunc: scc.updateScyllaDBCluster,
			DeleteFunc: scc.deleteScyllaDBCluster,
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't register to ScyllaDBCluster events: %w", err))
	}

	serviceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addService,
			UpdateFunc: scc.updateService,
			DeleteFunc: scc.deleteService,
		},
	)

	endpointSliceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addEndpointSlice,
			UpdateFunc: scc.updateEndpointSlice,
			DeleteFunc: scc.deleteEndpointSlice,
		},
	)

	endpointsInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addEndpoints,
			UpdateFunc: scc.updateEndpoints,
			DeleteFunc: scc.deleteEndpoints,
		},
	)

	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addSecret,
			UpdateFunc: scc.updateSecret,
			DeleteFunc: scc.deleteSecret,
		},
	)

	// Handlers for local ConfigMaps and Secrets referenced by ScyllaDBClusters are skipped to optimize number of syncs which doesn't do anything.
	// Applying configuration change requires rolling restart of ScyllaDBCluster, so these resources will be synced upon
	// ScyllaDBCluster update.
	// These could be added once ConfigMaps and Secrets would require immediate sync.

	// TODO: add error handling once these start returning errors
	remoteRemoteOwnerInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteRemoteOwner,
			UpdateFunc: scc.updateRemoteRemoteOwner,
			DeleteFunc: scc.deleteRemoteRemoteOwner,
		},
	)

	remoteNamespaceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteNamespace,
			UpdateFunc: scc.updateRemoteNamespace,
			DeleteFunc: scc.deleteRemoteNamespace,
		},
	)

	remoteScyllaDBDatacenterInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteScyllaDBDatacenter,
			UpdateFunc: scc.updateRemoteScyllaDBDatacenter,
			DeleteFunc: scc.deleteRemoteScyllaDBDatacenter,
		},
	)

	remoteServiceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteService,
			UpdateFunc: scc.updateRemoteService,
			DeleteFunc: scc.deleteRemoteService,
		},
	)

	remoteEndpointSliceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteEndpointSlice,
			UpdateFunc: scc.updateRemoteEndpointSlice,
			DeleteFunc: scc.deleteRemoteEndpointSlice,
		},
	)

	remoteEndpointsInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteEndpoints,
			UpdateFunc: scc.updateRemoteEndpoints,
			DeleteFunc: scc.deleteRemoteEndpoints,
		},
	)

	remotePodInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemotePod,
			UpdateFunc: scc.updateRemotePod,
			DeleteFunc: scc.deleteRemotePod,
		},
	)

	remoteConfigMapInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteConfigMap,
			UpdateFunc: scc.updateRemoteConfigMap,
			DeleteFunc: scc.deleteRemoteConfigMap,
		},
	)

	remoteSecretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    scc.addRemoteSecret,
			UpdateFunc: scc.updateRemoteSecret,
			DeleteFunc: scc.deleteRemoteSecret,
		},
	)

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, fmt.Errorf("can't register event handlers: %w", err)
	}

	return scc, nil
}

func (scc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := scc.queue.Get()
	if quit {
		return false
	}
	defer scc.queue.Done(key)

	err := scc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		scc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	scc.queue.AddRateLimited(key)

	return true
}

func (scc *Controller) runWorker(ctx context.Context) {
	for scc.processNextItem(ctx) {
	}
}

func (scc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaDBCluster")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaDBCluster")
		scc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaDBCluster")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), scc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, scc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (scc *Controller) enqueueThroughParentLabel(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	objLabels := obj.GetLabels()
	parentName, parentNamespace := objLabels[naming.ParentClusterNameLabel], objLabels[naming.ParentClusterNamespaceLabel]
	if len(parentName) == 0 || len(parentNamespace) == 0 {
		klog.V(5).InfoSDepth(depth, "got event about object not having parent labels", "Object", obj)
		return
	}

	sc, err := scc.scyllaDBClusterLister.ScyllaDBClusters(parentNamespace).Get(parentName)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaDBCluster for object %#v", obj))
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing parent", gvk.Kind, klog.KObj(obj), "ScyllaDBCluster", klog.KObj(sc))
	scc.handlers.Enqueue(depth+1, sc, op)
}

func (scc *Controller) enqueueThroughRemoteOwnerLabel(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	objLabels := obj.GetLabels()
	name, namespace, gvr := objLabels[naming.RemoteOwnerNameLabel], objLabels[naming.RemoteOwnerNamespaceLabel], objLabels[naming.RemoteOwnerGVR]
	if len(name) == 0 || len(namespace) == 0 {
		klog.V(5).InfoSDepth(depth, "got event about object not having remoteOwner labels", "Object", obj)
		return
	}

	if gvr != naming.GroupVersionResourceToLabelValue(scyllav1alpha1.GroupVersion.WithResource("scylladbclusters")) {
		return
	}

	sc, err := scc.scyllaDBClusterLister.ScyllaDBClusters(namespace).Get(name)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaDBCluster for object %#v", obj))
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing parent", gvk.Kind, klog.KObj(obj), "ScyllaDBCluster", klog.KObj(sc))
	scc.handlers.Enqueue(depth+1, sc, op)
}

func (scc *Controller) addScyllaDBCluster(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBCluster),
		scc.handlers.Enqueue,
	)
}

func (scc *Controller) updateScyllaDBCluster(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBCluster),
		cur.(*scyllav1alpha1.ScyllaDBCluster),
		scc.handlers.Enqueue,
		scc.deleteScyllaDBCluster,
	)
}

func (scc *Controller) deleteScyllaDBCluster(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.Enqueue,
	)
}

func (scc *Controller) addRemoteRemoteOwner(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.RemoteOwner),
		scc.enqueueThroughRemoteOwnerLabel,
	)
}

func (scc *Controller) updateRemoteRemoteOwner(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.RemoteOwner),
		cur.(*scyllav1alpha1.RemoteOwner),
		scc.enqueueThroughRemoteOwnerLabel,
		scc.deleteRemoteRemoteOwner,
	)
}

func (scc *Controller) deleteRemoteRemoteOwner(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughRemoteOwnerLabel,
	)
}

func (scc *Controller) addRemoteNamespace(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Namespace),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteNamespace(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Namespace),
		cur.(*corev1.Namespace),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteNamespace,
	)
}

func (scc *Controller) deleteRemoteNamespace(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteScyllaDBDatacenter(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBDatacenter),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteScyllaDBDatacenter(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBDatacenter),
		cur.(*scyllav1alpha1.ScyllaDBDatacenter),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteScyllaDBDatacenter,
	)
}

func (scc *Controller) deleteRemoteScyllaDBDatacenter(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteService(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Service),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteService(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteService,
	)
}

func (scc *Controller) deleteRemoteService(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteEndpointSlice(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*discoveryv1.EndpointSlice),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteEndpointSlice(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*discoveryv1.EndpointSlice),
		cur.(*discoveryv1.EndpointSlice),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteEndpointSlice,
	)
}

func (scc *Controller) deleteRemoteEndpointSlice(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteEndpoints(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Endpoints),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteEndpoints(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Endpoints),
		cur.(*corev1.Endpoints),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteEndpoints,
	)
}

func (scc *Controller) deleteRemoteEndpoints(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemotePod(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Pod),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemotePod(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Pod),
		cur.(*corev1.Pod),
		scc.enqueueThroughParentLabel,
		scc.deleteRemotePod,
	)
}

func (scc *Controller) deleteRemotePod(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteConfigMap(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteConfigMap(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteConfigMap,
	)
}

func (scc *Controller) deleteRemoteConfigMap(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addRemoteSecret(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) updateRemoteSecret(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		scc.enqueueThroughParentLabel,
		scc.deleteRemoteSecret,
	)
}

func (scc *Controller) deleteRemoteSecret(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueThroughParentLabel,
	)
}

func (scc *Controller) addService(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Service),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateService(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		scc.handlers.EnqueueOwner,
		scc.deleteService,
	)
}

func (scc *Controller) deleteService(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addEndpointSlice(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*discoveryv1.EndpointSlice),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateEndpointSlice(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*discoveryv1.EndpointSlice),
		cur.(*discoveryv1.EndpointSlice),
		scc.handlers.EnqueueOwner,
		scc.deleteEndpointSlice,
	)
}

func (scc *Controller) deleteEndpointSlice(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addEndpoints(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Endpoints),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateEndpoints(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Endpoints),
		cur.(*corev1.Endpoints),
		scc.handlers.EnqueueOwner,
		scc.deleteEndpoints,
	)
}

func (scc *Controller) deleteEndpoints(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addSecret(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateSecret(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		scc.handlers.EnqueueOwner,
		scc.deleteSecret,
	)
}

func (scc *Controller) deleteSecret(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}
