// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "RemoteKubernetesClusterController"
)

var (
	keyFunc                              = cache.DeletionHandlingMetaNamespaceKeyFunc
	remoteKubernetesClusterControllerGVK = scyllav1alpha1.GroupVersion.WithKind("RemoteKubernetesCluster")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	remoteKubernetesClusterLister scyllav1alpha1listers.RemoteKubernetesClusterLister
	secretLister                  corev1listers.SecretLister

	clusterKubeClient      remoteclient.ClusterClientInterface[kubernetes.Interface]
	clusterScyllaClient    remoteclient.ClusterClientInterface[scyllaclient.Interface]
	dynamicClusterHandlers []remoteclient.DynamicClusterInterface

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.RateLimitingInterface
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.RemoteKubernetesCluster]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	remoteKubernetesClusterInformer scyllav1alpha1informers.RemoteKubernetesClusterInformer,
	secretInformer corev1informers.SecretInformer,
	dynamicClusterHandlers []remoteclient.DynamicClusterInterface,
	clusterKubeClient remoteclient.ClusterClientInterface[kubernetes.Interface],
	clusterScyllaClient remoteclient.ClusterClientInterface[scyllaclient.Interface],
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	rkcc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		remoteKubernetesClusterLister: remoteKubernetesClusterInformer.Lister(),
		secretLister:                  secretInformer.Lister(),

		dynamicClusterHandlers: dynamicClusterHandlers,

		clusterKubeClient:   clusterKubeClient,
		clusterScyllaClient: clusterScyllaClient,

		cachesToSync: []cache.InformerSynced{
			remoteKubernetesClusterInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "remotekubernetescluster-controller"}),

		queue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: "remotekubernetescluster",
		}),
	}

	var err error
	rkcc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.RemoteKubernetesCluster](
		rkcc.queue,
		keyFunc,
		scheme.Scheme,
		remoteKubernetesClusterControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.RemoteKubernetesCluster]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.RemoteKubernetesCluster, error) {
				return rkcc.remoteKubernetesClusterLister.Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.RemoteKubernetesCluster, err error) {
				return rkcc.remoteKubernetesClusterLister.List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	remoteKubernetesClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rkcc.addRemoteKubernetesCluster,
		UpdateFunc: rkcc.updateRemoteKubernetesCluster,
		DeleteFunc: rkcc.deleteRemoteKubernetesCluster,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rkcc.addSecret,
		UpdateFunc: rkcc.updateSecret,
		DeleteFunc: rkcc.deleteSecret,
	})

	return rkcc, nil
}

func (rkcc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := rkcc.queue.Get()
	if quit {
		return false
	}
	defer rkcc.queue.Done(key)

	err := rkcc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		rkcc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	rkcc.queue.AddRateLimited(key)

	return true
}

func (rkcc *Controller) runWorker(ctx context.Context) {
	for rkcc.processNextItem(ctx) {
	}
}

func (rkcc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		rkcc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), rkcc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, rkcc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (rkcc *Controller) enqueue(rkc *scyllav1alpha1.RemoteKubernetesCluster) {
	key, err := keyFunc(rkc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", rkc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "RemoteKubernetesCluster", klog.KObj(rkc))
	rkcc.queue.Add(key)
}

func (rkcc *Controller) addRemoteKubernetesCluster(obj interface{}) {
	rkcc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.RemoteKubernetesCluster),
		rkcc.handlers.Enqueue,
	)
}

func (rkcc *Controller) updateRemoteKubernetesCluster(old, cur interface{}) {
	rkcc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.RemoteKubernetesCluster),
		cur.(*scyllav1alpha1.RemoteKubernetesCluster),
		rkcc.handlers.Enqueue,
		rkcc.deleteRemoteKubernetesCluster,
	)
}

func (rkcc *Controller) deleteRemoteKubernetesCluster(obj interface{}) {
	rkcc.handlers.HandleDelete(
		obj,
		rkcc.handlers.Enqueue,
	)
}

func (rkcc *Controller) addSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)

	rkcc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		rkcc.enqueueRemoteKubernetesClusterUsingSecret(secret),
	)
}

func (rkcc *Controller) updateSecret(old, cur interface{}) {
	secret := cur.(*corev1.Secret)

	rkcc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		rkcc.enqueueRemoteKubernetesClusterUsingSecret(secret),
		rkcc.deleteSecret,
	)
}

func (rkcc *Controller) deleteSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)

	rkcc.handlers.HandleDelete(
		obj,
		rkcc.enqueueRemoteKubernetesClusterUsingSecret(secret),
	)
}

func (rkcc *Controller) enqueueRemoteKubernetesClusterUsingSecret(secret *corev1.Secret) controllerhelpers.EnqueueFuncType {
	return rkcc.handlers.EnqueueAllFunc(rkcc.handlers.EnqueueWithFilterFunc(func(rkc *scyllav1alpha1.RemoteKubernetesCluster) bool {
		return rkc.Spec.KubeconfigSecretRef.Namespace == secret.Namespace && rkc.Spec.KubeconfigSecretRef.Name == secret.Name
	}))
}
