// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllav2alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v2alpha1"
	scyllav2alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v2alpha1"
	scyllav2alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v2alpha1"
	scyllamultiregionclient "github.com/scylladb/scylla-operator/pkg/multiregionclient/scylla/clientset/versioned"
	scyllav1alpha1multiregioninformers "github.com/scylladb/scylla-operator/pkg/multiregionclient/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1multiregionlisters "github.com/scylladb/scylla-operator/pkg/multiregionclient/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	ControllerName = "ScyllaClusterController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 1 * time.Minute
)

var (
	keyFunc       = cache.DeletionHandlingMetaNamespaceKeyFunc
	controllerGVK = scyllav2alpha1.GroupVersion.WithKind("ScyllaCluster")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav2alpha1client.ScyllaV2alpha1Interface

	scyllaMultiregionClient scyllamultiregionclient.RemoteInterface

	scyllaLister scyllav2alpha1listers.ScyllaClusterLister

	scyllaDatacenterMultiregionInformer scyllav1alpha1multiregioninformers.ScyllaDatacenterRemoteInformer
	scyllaDatacenterMultiregionLister   scyllav1alpha1multiregionlisters.ScyllaDatacenterRemoteLister

	// remoteNamespaceInformer             core.RemoteNamespaceInformer

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav2alpha1client.ScyllaV2alpha1Interface,
	scyllaClusterInformer scyllav2alpha1informers.ScyllaClusterInformer,
	scyllaMultiregionClient scyllamultiregionclient.RemoteInterface,
	scyllaDatacenterMultiregionInformer scyllav1alpha1multiregioninformers.ScyllaDatacenterRemoteInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllacluster_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	scc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaMultiregionClient: scyllaMultiregionClient,

		scyllaLister: scyllaClusterInformer.Lister(),

		scyllaDatacenterMultiregionInformer: scyllaDatacenterMultiregionInformer,
		scyllaDatacenterMultiregionLister:   scyllaDatacenterMultiregionInformer.Lister(),

		// remoteNamespaceInformer:             remoteNamespaceInformer,

		cachesToSync: []cache.InformerSynced{
			scyllaClusterInformer.Informer().HasSynced,
			scyllaDatacenterMultiregionInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllacluster-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllacluster"),
	}

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addScyllaCluster,
		UpdateFunc: scc.updateScyllaCluster,
		DeleteFunc: scc.deleteScyllaCluster,
	})

	// remoteNamespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    scc.addRemoteNamespace,
	// 	UpdateFunc: scc.updateRemoteNamespace,
	// 	DeleteFunc: scc.deleteRemoteNamespace,
	// })

	scyllaDatacenterMultiregionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addRemoteScyllaDatacenter,
		UpdateFunc: scc.updateRemoteScyllaDatacenter,
		DeleteFunc: scc.deleteRemoteScyllaDatacenter,
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

	klog.InfoS("Starting controller", "controller", "ScyllaCluster")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaCluster")
		scc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaCluster")
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

func (scc *Controller) enqueue(sc *scyllav2alpha1.ScyllaCluster) {
	key, err := keyFunc(sc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaCluster", klog.KObj(sc))
	scc.queue.Add(key)
}

func (scc *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav2alpha1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	scc.enqueue(sc)
}

func (scc *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav2alpha1.ScyllaCluster)
	currentSC := cur.(*scyllav2alpha1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		scc.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	scc.enqueue(currentSC)
}

func (scc *Controller) deleteScyllaCluster(obj interface{}) {
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
	scc.enqueue(sc)
}

func (scc *Controller) addRemoteScyllaDatacenter(obj interface{}) {
	sd := obj.(*scyllav1alpha1.ScyllaDatacenter)
	klog.V(4).InfoS("Observed addition of remote ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	scc.enqueueParent(sd)
}

func (scc *Controller) updateRemoteScyllaDatacenter(old, cur interface{}) {
	oldSD := old.(*scyllav1alpha1.ScyllaDatacenter)
	currentSD := cur.(*scyllav1alpha1.ScyllaDatacenter)

	if currentSD.UID != oldSD.UID {
		key, err := keyFunc(oldSD)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSD, err))
			return
		}
		scc.deleteRemoteScyllaDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSD,
		})
	}

	klog.V(4).InfoS("Observed update of remote ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(oldSD))
	scc.enqueueParent(currentSD)
}

func (scc *Controller) deleteRemoteScyllaDatacenter(obj interface{}) {
	sd, ok := obj.(*scyllav1alpha1.ScyllaDatacenter)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sd, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDatacenter)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaDatacenter %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	scc.enqueueParent(sd)
}

func (scc *Controller) addRemoteNamespace(obj interface{}) {
	ns := obj.(*corev1.Namespace)
	klog.V(4).InfoS("Observed addition of remote Namespace", "Namespace", klog.KObj(ns))
	scc.enqueueParent(ns)
}

func (scc *Controller) updateRemoteNamespace(old, cur interface{}) {
	oldNs := old.(*corev1.Namespace)
	currentNs := cur.(*corev1.Namespace)

	if currentNs.UID != oldNs.UID {
		key, err := keyFunc(oldNs)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNs, err))
			return
		}
		scc.deleteRemoteNamespace(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNs,
		})
	}

	klog.V(4).InfoS("Observed update of remote Namespace", "Namespace", klog.KObj(oldNs))
	scc.enqueueParent(currentNs)
}

func (scc *Controller) deleteRemoteNamespace(obj interface{}) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		ns, ok = tombstone.Obj.(*corev1.Namespace)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Namespace %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Namespace", "Namespace", klog.KObj(ns))
	scc.enqueueParent(ns)
}

func (scc *Controller) enqueueParent(obj metav1.Object) {
	labels := obj.GetLabels()
	name, namespace := labels[naming.ParentClusterNameLabel], labels[naming.ParentClusterNamespaceLabel]
	if len(name) == 0 && len(namespace) == 0 {
		return
	}

	sc, err := scc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaCluster for object %#v", obj))
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing parent", gvk.Kind, klog.KObj(obj), "ScyllaCluster", klog.KObj(sc))
	scc.enqueue(sc)
}
