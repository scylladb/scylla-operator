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
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
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
	ControllerName = "OrphanedPVController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// Controller watches all PVs actively belonging to a ScyllaDBDatacenter and replace scylla node
// on any PV that is orphaned, if enabled on the ScyllaDBDatacenter.
// Orphaned PV is a volume that is hard bound to a node that doesn't exist anymore.
// The controller is based on a ScyllaDBDatacenter key and listing matching PVs because using a PV key and trying to
// find a corresponding ScyllaDBDatacenter would be quite hard, there are no ownerRefs and we can't
// propagate the "enabled" information from a ScyllaDBDatacenter to a PVC annotation because PVCs are not
// reconciled.
// TODO: When we support auto-replacing nodes, we could replace it with generic controller
//
//	deleting PVs bound to nodes that don't exists anymore, without knowing about ScyllaDBDatacenter.
//	It would also process PVs instead of ScyllaDBDatacenter which is currently complicating the logic
//	that has to handle multiple PVs at once, artificial requeues / not watching PVs and different error paths.
type Controller struct {
	kubeClient kubernetes.Interface

	pvLister                 corev1listers.PersistentVolumeLister
	pvcLister                corev1listers.PersistentVolumeClaimLister
	nodeLister               corev1listers.NodeLister
	scyllaDBDatacenterLister scyllav1alpha1listers.ScyllaDBDatacenterLister

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
	scyllaDBDatacenterInformer scyllav1alpha1informers.ScyllaDBDatacenterInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	opc := &Controller{
		kubeClient:               kubeClient,
		pvLister:                 pvInformer.Lister(),
		pvcLister:                pvcInformer.Lister(),
		nodeLister:               nodeInformer.Lister(),
		scyllaDBDatacenterLister: scyllaDBDatacenterInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			pvInformer.Informer().HasSynced,
			pvcInformer.Informer().HasSynced,
			nodeInformer.Informer().HasSynced,
			scyllaDBDatacenterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "orphanedpv-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "orphanedpv"),
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: opc.updateNode,
		DeleteFunc: opc.deleteNode,
	})

	scyllaDBDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    opc.addScyllaDBDatacenter,
		UpdateFunc: opc.updateScyllaDBDatacenter,
		DeleteFunc: opc.deleteScyllaDBDatacenter,
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
	allErrors := apimachineryutilerrors.Flatten(apimachineryutilerrors.NewAggregate([]error{syncErr}))
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

	err := apimachineryutilerrors.NewAggregate(remainingErrors)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	opc.queue.AddRateLimited(key)

	return true
}

func (opc *Controller) runWorker(ctx context.Context) {
	for opc.processNextItem(ctx) {
	}
}

func (opc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

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

	for range workers {
		opc.wg.Add(1)
		go func() {
			defer opc.wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, opc.runWorker, time.Second)
		}()
	}

	// Make sure to reconcile if we were to miss any event given the current architecture of this controller.
	opc.wg.Add(1)
	go func() {
		defer opc.wg.Done()
		apimachineryutilwait.UntilWithContext(ctx, func(ctx context.Context) {
			klog.V(4).InfoS("Periodically enqueuing all ScyllaClusters")

			sdcs, err := opc.scyllaDBDatacenterLister.ScyllaDBDatacenters(corev1.NamespaceAll).List(labels.Everything())
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				return
			}

			for _, sdc := range sdcs {
				opc.enqueue(sdc)
			}
		}, 30*time.Minute)
	}()

	<-ctx.Done()
}

func (opc *Controller) enqueue(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	key, err := keyFunc(sdc)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sdc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaDBDatacenter", klog.KObj(sdc))
	opc.queue.Add(key)
}

func (opc *Controller) enqueueAllScyllaDBDatacentersOnBackground() {
	opc.wg.Add(1)
	go func() {
		klog.V(4).InfoS("Enqueuing all ScyllaDBDatacenters")

		// This gets called from an informer handler which doesn't wait for cache sync,
		// but any ScyllaDBDatacenter that won't list here is gonna be queued later on addition
		// by the ScyllaDBDatacenter handler.
		sdcs, err := opc.scyllaDBDatacenterLister.ScyllaDBDatacenters(corev1.NamespaceAll).List(labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return
		}

		for _, sdc := range sdcs {
			opc.enqueue(sdc)
		}
	}()
}

func (opc *Controller) updateNode(old, cur interface{}) {
	oldNode := old.(*corev1.Node)
	currentNode := cur.(*corev1.Node)

	if currentNode.UID != oldNode.UID {
		key, err := keyFunc(oldNode)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNode, err))
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
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		node, ok = tombstone.Obj.(*corev1.Node)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Node %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Node", "Node", klog.KObj(node))

	// We can't run a long running task in the handler because it'd block the informer.
	// Add on background.
	opc.enqueueAllScyllaDBDatacentersOnBackground()
}

func (opc *Controller) addScyllaDBDatacenter(obj interface{}) {
	sdc := obj.(*scyllav1alpha1.ScyllaDBDatacenter)
	klog.V(4).InfoS("Observed addition of ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KObj(sdc))
	opc.enqueue(sdc)
}

func (opc *Controller) updateScyllaDBDatacenter(old, cur interface{}) {
	oldSDC := old.(*scyllav1alpha1.ScyllaDBDatacenter)
	currentSDC := cur.(*scyllav1alpha1.ScyllaDBDatacenter)

	if currentSDC.UID != oldSDC.UID {
		key, err := keyFunc(oldSDC)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSDC, err))
			return
		}
		opc.deleteScyllaDBDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSDC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KObj(oldSDC))
	opc.enqueue(currentSDC)
}

func (opc *Controller) deleteScyllaDBDatacenter(obj interface{}) {
	sdc, ok := obj.(*scyllav1alpha1.ScyllaDBDatacenter)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sdc, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDBDatacenter)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaDBDatacenter %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KObj(sdc))
	opc.enqueue(sdc)
}
