package sidecar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ControllerName = "SidecarController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	namespace   string
	serviceName string
	secretName  string
	hostAddr    string

	kubeClient          kubernetes.Interface
	singleServiceLister corev1listers.ServiceLister
	secretLister        corev1listers.SecretLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
	key   string
}

func NewController(
	namespace,
	serviceName string,
	secretName string,
	hostAddr string,
	kubeClient kubernetes.Interface,
	singleServiceInformer corev1informers.ServiceInformer,
	secretInformer corev1informers.SecretInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllasidecar_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, fmt.Errorf("can't register ratelimitter metrics: %w", err)
		}
	}

	// Sanity check.
	if len(namespace) == 0 {
		return nil, fmt.Errorf("service namespace can't be empty")
	}
	if len(serviceName) == 0 {
		return nil, fmt.Errorf("service name can't be empty")
	}

	// This is a singleton controller.
	key, err := keyFunc(&metav1.ObjectMeta{
		Namespace: namespace,
		Name:      serviceName,
	})
	if err != nil {
		return nil, fmt.Errorf("can't get key: %w", err)
	}

	scc := &Controller{
		namespace:   namespace,
		serviceName: serviceName,
		secretName:  secretName,
		hostAddr:    hostAddr,

		kubeClient:          kubeClient,
		singleServiceLister: singleServiceInformer.Lister(),
		secretLister:        secretInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			singleServiceInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllasidecar-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllasidecar"),
		key:   key,
	}

	singleServiceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addService,
		UpdateFunc: scc.updateService,
		DeleteFunc: scc.deleteService,
	})

	return scc, nil
}
func (c *Controller) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	if key != c.key {
		utilruntime.HandleError(fmt.Errorf("got unsupported key %q (singleton key is %q)", key, c.key))
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := c.sync(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		c.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	c.queue.AddRateLimited(key)

	return true
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *Controller) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "Controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "Controller", ControllerName)
		c.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "Controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.cachesToSync...) {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}()

	<-ctx.Done()
}

func (c *Controller) enqueue(svc *corev1.Service) {
	key, err := keyFunc(svc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", svc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "Service", klog.KObj(svc))
	c.queue.Add(key)
}

func (c *Controller) addService(obj interface{}) {
	svc := obj.(*corev1.Service)
	klog.V(4).InfoS("Observed addition of Service", "Service", klog.KObj(svc))
	c.enqueue(svc)
}

func (c *Controller) updateService(old, cur interface{}) {
	oldService := old.(*corev1.Service)
	currentService := cur.(*corev1.Service)

	if currentService.UID != oldService.UID {
		key, err := keyFunc(oldService)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldService, err))
			return
		}
		c.deleteService(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldService,
		})
	}

	klog.V(4).InfoS("Observed update of Service", "Service", klog.KObj(oldService))
	c.enqueue(currentService)
}

func (c *Controller) deleteService(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		svc, ok = tombstone.Obj.(*corev1.Service)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Service %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Service", "Service", klog.KObj(svc))
	c.enqueue(svc)
}
