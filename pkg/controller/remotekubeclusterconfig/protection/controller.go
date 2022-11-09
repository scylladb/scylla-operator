// Copyright (c) 2022 ScyllaDB.

package protection

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav2alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v2alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	scyllav2alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "RemoteKubeClusterConfigProtectionController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	scyllaClusterLister           scyllav2alpha1listers.ScyllaClusterLister
	remoteKubeClusterConfigLister scyllav1alpha1listers.RemoteKubeClusterConfigLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	scyllaClusterInformer scyllav2alpha1informers.ScyllaClusterInformer,
	remoteKubeClusterConfigInformer scyllav1alpha1informers.RemoteKubeClusterConfigInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"remotekubeclusterconfigprotection_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	scpc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaClusterLister:           scyllaClusterInformer.Lister(),
		remoteKubeClusterConfigLister: remoteKubeClusterConfigInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaClusterInformer.Informer().HasSynced,
			remoteKubeClusterConfigInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "remotekubeclusterconfigprotection-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "remotekubeclusterconfigprotection"),
	}

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scpc.addScyllaCluster,
		UpdateFunc: scpc.updateScyllaCluster,
		DeleteFunc: scpc.deleteScyllaCluster,
	})

	remoteKubeClusterConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scpc.addRemoteKubeClusterConfig,
		UpdateFunc: scpc.updateRemoteKubeClusterConfig,
		DeleteFunc: scpc.deleteRemoteKubeClusterConfig,
	})

	return scpc, nil
}

func (rkccpc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := rkccpc.queue.Get()
	if quit {
		return false
	}
	defer rkccpc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := rkccpc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		rkccpc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	rkccpc.queue.AddRateLimited(key)

	return true
}

func (rkccpc *Controller) runWorker(ctx context.Context) {
	for rkccpc.processNextItem(ctx) {
	}
}

func (rkccpc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		rkccpc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), rkccpc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, rkccpc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (rkccpc *Controller) enqueue(rkcc *scyllav1alpha1.RemoteKubeClusterConfig) {
	key, err := keyFunc(rkcc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", rkcc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	rkccpc.queue.Add(key)
}

func (rkccpc *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav2alpha1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	rkccpc.enqueueScyllaCluster(sc)
}

func (rkccpc *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav2alpha1.ScyllaCluster)
	currentSC := cur.(*scyllav2alpha1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		rkccpc.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	rkccpc.enqueueScyllaCluster(currentSC)
}

func (rkccpc *Controller) deleteScyllaCluster(obj interface{}) {
	sc, ok := obj.(*scyllav2alpha1.ScyllaCluster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sc, ok = tombstone.Obj.(*scyllav2alpha1.ScyllaCluster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaCluster %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	rkccpc.enqueueScyllaCluster(sc)
}

func (rkccpc *Controller) addRemoteKubeClusterConfig(obj interface{}) {
	rkcc := obj.(*scyllav1alpha1.RemoteKubeClusterConfig)
	klog.V(4).InfoS("Observed addition of RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	rkccpc.enqueue(rkcc)
}

func (rkccpc *Controller) updateRemoteKubeClusterConfig(old, cur interface{}) {
	oldRkcc := old.(*scyllav1alpha1.RemoteKubeClusterConfig)
	currentRkcc := cur.(*scyllav1alpha1.RemoteKubeClusterConfig)

	if currentRkcc.UID != oldRkcc.UID {
		key, err := keyFunc(oldRkcc)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldRkcc, err))
			return
		}
		rkccpc.deleteRemoteKubeClusterConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldRkcc,
		})
	}

	klog.V(4).InfoS("Observed update of RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(oldRkcc))
	rkccpc.enqueue(currentRkcc)
}

func (rkccpc *Controller) deleteRemoteKubeClusterConfig(obj interface{}) {
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
	rkccpc.enqueue(rkcc)
}

func (rkccpc *Controller) enqueueScyllaCluster(sc *scyllav2alpha1.ScyllaCluster) {
	for _, dc := range sc.Spec.Datacenters {
		if dc.RemoteKubeClusterConfigRef == nil {
			continue
		}

		rkcc, err := rkccpc.remoteKubeClusterConfigLister.Get(dc.RemoteKubeClusterConfigRef.Name)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't find RemoteKubeClusterConfig with name %q", dc.RemoteKubeClusterConfigRef.Name))
			return
		}

		klog.V(4).InfoS("Enqueuing through ScyllaCluster", "RemoteKubeClusterConfig", klog.KObj(rkcc), "ScyllaCluster", klog.KObj(sc))
		rkccpc.enqueue(rkcc)
	}
}
