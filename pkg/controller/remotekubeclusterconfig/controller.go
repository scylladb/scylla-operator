// Copyright (c) 2022 ScyllaDB.

package remotekubeclusterconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	scyllamultiregionclient "github.com/scylladb/scylla-operator/pkg/remotedynamicclient/informers"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "MultiRegionClientController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 1 * time.Minute
	resyncPeriod    = 12 * time.Hour
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	remoteKubeClusterConfigLister scyllav1alpha1listers.RemoteKubeClusterConfigLister

	secretLister corev1listers.SecretLister

	syncHandlers []scyllamultiregionclient.DynamicRegion

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	remoteKubeClusterConfigInformer scyllav1alpha1informers.RemoteKubeClusterConfigInformer,
	secretInformer corev1informers.SecretInformer,
	syncHandlers []scyllamultiregionclient.DynamicRegion,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"multiregionclient_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	scc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		secretLister: secretInformer.Lister(),

		remoteKubeClusterConfigLister: remoteKubeClusterConfigInformer.Lister(),
		syncHandlers:                  syncHandlers,

		cachesToSync: []cache.InformerSynced{
			remoteKubeClusterConfigInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "multiregionclient-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "multiregionclient"),
	}

	remoteKubeClusterConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addRemoteKubeClusterConfig,
		UpdateFunc: scc.updateRemoteKubeClusterConfig,
		DeleteFunc: scc.deleteRemoteKubeClusterConfig,
	})

	return scc, nil
}

func (scc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := scc.queue.Get()
	if quit {
		return false
	}
	defer scc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := scc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		scc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	scc.queue.AddRateLimited(key)

	return true
}

func (scc *Controller) runWorker(ctx context.Context) {
	for scc.processNextItem(ctx) {
	}
}

func (scc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		scc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), scc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, scc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (scc *Controller) enqueue(rkcc *scyllav1alpha1.RemoteKubeClusterConfig) {
	key, err := keyFunc(rkcc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", rkcc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	scc.queue.Add(key)
}

func (scc *Controller) addRemoteKubeClusterConfig(obj interface{}) {
	rkcc := obj.(*scyllav1alpha1.RemoteKubeClusterConfig)
	klog.V(4).InfoS("Observed addition of RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	scc.enqueue(rkcc)
}

func (scc *Controller) updateRemoteKubeClusterConfig(old, cur interface{}) {
	oldRkcc := old.(*scyllav1alpha1.RemoteKubeClusterConfig)
	currentRkcc := cur.(*scyllav1alpha1.RemoteKubeClusterConfig)

	if currentRkcc.UID != oldRkcc.UID {
		key, err := keyFunc(oldRkcc)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldRkcc, err))
			return
		}
		scc.deleteRemoteKubeClusterConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldRkcc,
		})
	}

	klog.V(4).InfoS("Observed update of RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(oldRkcc))
	scc.enqueue(currentRkcc)
}

func (scc *Controller) deleteRemoteKubeClusterConfig(obj interface{}) {
	rkcc, ok := obj.(*scyllav1alpha1.RemoteKubeClusterConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		rkcc, ok = tombstone.Obj.(*scyllav1alpha1.RemoteKubeClusterConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a RemoteKubeClusterConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	scc.enqueue(rkcc)
}
