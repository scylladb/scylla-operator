package controllertools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type ObserverSyncFunc = func(ctx context.Context) error

type Observer struct {
	name          string
	queue         workqueue.RateLimitingInterface
	syncFunc      ObserverSyncFunc
	eventRecorder record.EventRecorder
	cachesToSync  []cache.InformerSynced
}

func NewObserver(name string, eventsClient corev1client.EventInterface, syncFunc ObserverSyncFunc) *Observer {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: eventsClient})

	return &Observer{
		name: name,
		queue: workqueue.NewRateLimitingQueueWithConfig(
			workqueue.DefaultControllerRateLimiter(),
			workqueue.RateLimitingQueueConfig{
				Name: name,
			}),
		syncFunc:      syncFunc,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: name}),
	}
}

func (o *Observer) Name() string {
	return o.name
}

func (o *Observer) singletonKey() string {
	return o.name
}

func (o *Observer) Enqueue() {
	o.queue.Add(o.name)
}

func (o *Observer) AddCachesToSync(caches ...cache.InformerSynced) {
	o.cachesToSync = append(o.cachesToSync, caches...)
}

func (o *Observer) AddHandler(obj interface{}) {
	o.Enqueue()
}

func (o *Observer) UpdateHandler(old, new interface{}) {
	o.Enqueue()
}

func (o *Observer) DeleteHandler(obj interface{}) {
	o.Enqueue()
}

func (o *Observer) GetGenericHandlers() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    o.AddHandler,
		UpdateFunc: o.UpdateHandler,
		DeleteFunc: o.DeleteHandler,
	}
}

func (o *Observer) processNextItem(ctx context.Context) bool {
	key, quit := o.queue.Get()
	if quit {
		return false
	}
	defer o.queue.Done(key)

	if key != o.singletonKey() {
		klog.ErrorS(fmt.Errorf("wrong key %q", key), "SingletonKey", o.singletonKey())
	}

	err := o.syncFunc(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		o.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		if IsNonRetriable(err) {
			klog.InfoS("Hit non-retriable error. Dropping the item from the queue.", "Error", err)
			o.queue.Forget(key)
			return true
		}
		utilruntime.HandleError(fmt.Errorf("sync loop has failed: %w", err))
	}

	o.queue.AddRateLimited(key)

	return true
}

func (o *Observer) runWorker(ctx context.Context) {
	for o.processNextItem(ctx) {
	}
}

func (o *Observer) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting observer", "Name", o.name)

	if !cache.WaitForNamedCacheSync("Observer "+o.name, ctx.Done(), o.cachesToSync...) {
		klog.ErrorS(fmt.Errorf("timed out waiting for caches to sync"), "Observer", o.name)
		return
	}

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down observer", "Name", o.name)
		o.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down observer", "Name", o.name)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		wait.UntilWithContext(ctx, o.runWorker, time.Second)
	}()

	<-ctx.Done()
}
