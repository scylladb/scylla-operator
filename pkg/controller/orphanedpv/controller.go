package orphanedpv

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
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
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "OrphanedPVController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// Controller watches all PVs actively belonging to a ScyllaDatacenter and replace scylla node
// on any PV that is orphaned, if enabled on the ScyllaDatacenter.
// Orphaned PV is a volume that is hard bound to a node that doesn't exist anymore.
// The controller is based on a ScyllaDatacenter key and listing matching PVs because using a PV key and trying to
// find a corresponding ScyllaDatacenter would be quite hard, there are no ownerRefs and we can't
// propagate the "enabled" information from a ScyllaDatacenter to a PVC annotation because PVCs are not
// reconciled.
// TODO: When we support auto-replacing nodes, we could replace it with generic controller
//
//	deleting PVs bound to nodes that don't exists anymore, without knowing about ScyllaDatacenters.
//	It would also process PVs instead of ScyllaDatacenters which is currently complicating the logic
//	that has to handle multiple PVs at once, artificial requeues / not watching PVs and different error paths.
type Controller struct {
	kubeClient kubernetes.Interface

	pvLister     corev1listers.PersistentVolumeLister
	pvcLister    corev1listers.PersistentVolumeClaimLister
	nodeLister   corev1listers.NodeLister
	scyllaLister scyllav1alpha1listers.ScyllaDatacenterLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	wg sync.WaitGroup
}

func NewController(
	kubeClient kubernetes.Interface,
	pvInformer corev1informers.PersistentVolumeInformer,
	pvcInformer corev1informers.PersistentVolumeClaimInformer,
	nodeInformer corev1informers.NodeInformer,
	scyllaDatacenterInformer scyllav1alpha1informers.ScyllaDatacenterInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"orphanedpv_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	opc := &Controller{
		kubeClient:   kubeClient,
		pvLister:     pvInformer.Lister(),
		pvcLister:    pvcInformer.Lister(),
		nodeLister:   nodeInformer.Lister(),
		scyllaLister: scyllaDatacenterInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			pvInformer.Informer().HasSynced,
			pvcInformer.Informer().HasSynced,
			nodeInformer.Informer().HasSynced,
			scyllaDatacenterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "orphanedpv-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "orphanedpv"),
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: opc.updateNode,
		DeleteFunc: opc.deleteNode,
	})

	scyllaDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    opc.addScyllaDatacenter,
		UpdateFunc: opc.updateScyllaDatacenter,
		DeleteFunc: opc.deleteScyllaDatacenter,
	})

	return opc, nil
}

func (opc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := opc.queue.Get()
	if quit {
		return false
	}
	defer opc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	syncErr := opc.sync(ctx, key.(string))
	if syncErr == nil {
		opc.queue.Forget(key)
		return true
	}

	// Make sure we always have an aggregate to process and all nested errors are flattened.
	allErrors := utilerrors.Flatten(utilerrors.NewAggregate([]error{syncErr}))
	var remainingErrors []error
	for _, err := range allErrors.Errors() {
		switch {
		case errors.Is(err, &controllerhelpers.RequeueError{}):
			klog.V(2).InfoS("Re-queuing for recheck", "Key", key, "Reason", err)

		case apierrors.IsConflict(err):
			klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

		case apierrors.IsAlreadyExists(err):
			klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

		default:
			remainingErrors = append(remainingErrors, err)
		}
	}

	err := utilerrors.NewAggregate(remainingErrors)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	opc.queue.AddRateLimited(key)

	return true
}

func (opc *Controller) runWorker(ctx context.Context) {
	for opc.processNextItem(ctx) {
	}
}

func (opc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "OrphanedPV")

	defer func() {
		klog.InfoS("Shutting down controller", "controller", "OrphanedPV")
		opc.queue.ShutDown()
		opc.wg.Wait()
		klog.InfoS("Shut down controller", "controller", "OrphanedPV")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), opc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		opc.wg.Add(1)
		go func() {
			defer opc.wg.Done()
			wait.UntilWithContext(ctx, opc.runWorker, time.Second)
		}()
	}

	// Make sure to reconcile if we were to miss any event given the current architecture of this controller.
	opc.wg.Add(1)
	go func() {
		defer opc.wg.Done()
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			klog.V(4).InfoS("Periodically enqueuing all ScyllaDatacenters")

			scs, err := opc.scyllaLister.ScyllaDatacenters(corev1.NamespaceAll).List(labels.Everything())
			if err != nil {
				utilruntime.HandleError(err)
				return
			}

			for _, sc := range scs {
				opc.enqueue(sc)
			}
		}, 30*time.Minute)
	}()

	<-ctx.Done()
}

func (opc *Controller) enqueue(sd *scyllav1alpha1.ScyllaDatacenter) {
	key, err := keyFunc(sd)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sd, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaDatacenter", klog.KObj(sd))
	opc.queue.Add(key)
}

func (opc *Controller) enqueueAllScyllaDatacentersOnBackground() {
	opc.wg.Add(1)
	go func() {
		klog.V(4).InfoS("Enqueuing all ScyllaDatacenters")

		// This gets called from an informer handler which doesn't wait for cache sync,
		// but any ScyllaDatacenter that won't list here is gonna be queued later on addition
		// by the ScyllaDatacenter handler.
		sds, err := opc.scyllaLister.ScyllaDatacenters(corev1.NamespaceAll).List(labels.Everything())
		if err != nil {
			utilruntime.HandleError(err)
			return
		}

		for _, sd := range sds {
			opc.enqueue(sd)
		}
	}()
}

func (opc *Controller) updateNode(old, cur interface{}) {
	oldNode := old.(*corev1.Node)
	currentNode := cur.(*corev1.Node)

	if currentNode.UID != oldNode.UID {
		key, err := keyFunc(oldNode)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNode, err))
			return
		}
		opc.deleteNode(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNode,
		})
	}
}

func (opc *Controller) deleteNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		node, ok = tombstone.Obj.(*corev1.Node)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Node %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Node", "Node", klog.KObj(node))

	// We can't run a long running task in the handler because it'd block the informer.
	// Add on background.
	opc.enqueueAllScyllaDatacentersOnBackground()
}

func (opc *Controller) addScyllaDatacenter(obj interface{}) {
	sd := obj.(*scyllav1alpha1.ScyllaDatacenter)
	klog.V(4).InfoS("Observed addition of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	opc.enqueue(sd)
}

func (opc *Controller) updateScyllaDatacenter(old, cur interface{}) {
	oldSD := old.(*scyllav1alpha1.ScyllaDatacenter)
	currentSD := cur.(*scyllav1alpha1.ScyllaDatacenter)

	if currentSD.UID != oldSD.UID {
		key, err := keyFunc(oldSD)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSD, err))
			return
		}
		opc.deleteScyllaDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSD,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(oldSD))
	opc.enqueue(currentSD)
}

func (opc *Controller) deleteScyllaDatacenter(obj interface{}) {
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
	opc.enqueue(sd)
}
