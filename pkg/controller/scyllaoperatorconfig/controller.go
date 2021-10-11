package scyllaoperatorconfig

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig/resource"
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
	ControllerName = "ScyllaOperatorConfigController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	scyllaOperatorConfigLister scyllav1alpha1listers.ScyllaOperatorConfigLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	wg sync.WaitGroup
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	scyllaOperatorConfigInformer scyllav1alpha1informers.ScyllaOperatorConfigInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllaoperatorconfig_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	opc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaOperatorConfigLister: scyllaOperatorConfigInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaOperatorConfigInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "orphanedpv-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "orphanedpv"),
	}

	// Reconcile default ScyllaOperatorConfig right away.
	opc.enqueue(resource.DefaultScyllaOperatorConfig())

	scyllaOperatorConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    opc.addScyllaOperatorConfig,
		UpdateFunc: opc.updateScyllaOperatorConfig,
		DeleteFunc: opc.deleteScyllaOperatorConfig,
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
		case errors.Is(err, &helpers.RequeueError{}):
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

	klog.InfoS("Starting controller", "controller", ControllerName)

	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		opc.queue.ShutDown()
		opc.wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
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

	<-ctx.Done()
}

func (opc *Controller) enqueue(soc *scyllav1alpha1.ScyllaOperatorConfig) {
	key, err := keyFunc(soc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", soc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaOperatorConfig", klog.KObj(soc))
	opc.queue.Add(key)
}

func (opc *Controller) addScyllaOperatorConfig(obj interface{}) {
	soc := obj.(*scyllav1alpha1.ScyllaOperatorConfig)
	klog.V(4).InfoS("Observed addition of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(soc))
	opc.enqueue(soc)
}

func (opc *Controller) updateScyllaOperatorConfig(old, cur interface{}) {
	oldSoc := old.(*scyllav1alpha1.ScyllaOperatorConfig)
	currentSoc := cur.(*scyllav1alpha1.ScyllaOperatorConfig)

	if currentSoc.UID != oldSoc.UID {
		key, err := keyFunc(oldSoc)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSoc, err))
			return
		}
		opc.deleteScyllaOperatorConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSoc,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(oldSoc))
	opc.enqueue(currentSoc)
}

func (opc *Controller) deleteScyllaOperatorConfig(obj interface{}) {
	soc, ok := obj.(*scyllav1alpha1.ScyllaOperatorConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		soc, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaOperatorConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaOperatorConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(soc))
	opc.enqueue(soc)
}
