package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	ControllerName = "ScyllaManagerController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc                    = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaClusterControllerGVK = scyllav1.GroupVersion.WithKind("ScyllaCluster")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1client.ScyllaV1Interface

	secretLister corev1listers.SecretLister
	scyllaLister scyllav1listers.ScyllaClusterLister

	managerClient *mermaidclient.Client

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1client.ScyllaV1Interface,
	secretInformer corev1informers.SecretInformer,
	scyllaClusterInformer scyllav1informers.ScyllaClusterInformer,
	managerClient *mermaidclient.Client,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"manager_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	c := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		secretLister: secretInformer.Lister(),
		scyllaLister: scyllaClusterInformer.Lister(),

		managerClient: managerClient,

		cachesToSync: []cache.InformerSynced{
			secretInformer.Informer().HasSynced,
			scyllaClusterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "manager-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "manager"),
	}

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addScyllaCluster,
		UpdateFunc: c.updateScyllaCluster,
		DeleteFunc: c.deleteScyllaCluster,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addSecret,
		UpdateFunc: c.updateSecret,
		DeleteFunc: c.deleteSecret,
	})

	return c, nil
}

func (c *Controller) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := c.sync(ctx, key.(string))
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

func (c *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaManager")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaManager")
		c.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaManager")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, c.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (c *Controller) resolveScyllaClusterController(obj metav1.Object) *scyllav1.ScyllaCluster {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != scyllaClusterControllerGVK.Kind {
		return nil
	}

	sc, err := c.scyllaLister.ScyllaClusters(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sc.UID != controllerRef.UID {
		return nil
	}

	return sc
}

func (c *Controller) enqueueOwner(obj metav1.Object) {
	sc := c.resolveScyllaClusterController(obj)
	if sc == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaCluster", klog.KObj(sc))
	c.enqueue(sc)
}

func (c *Controller) enqueue(sc *scyllav1.ScyllaCluster) {
	key, err := keyFunc(sc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaCluster", klog.KObj(sc))
	c.queue.Add(key)
}

func (c *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	c.enqueue(sc)
}

func (c *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav1.ScyllaCluster)
	currentSC := cur.(*scyllav1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		c.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	c.enqueue(currentSC)
}

func (c *Controller) deleteScyllaCluster(obj interface{}) {
	sc, ok := obj.(*scyllav1.ScyllaCluster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sc, ok = tombstone.Obj.(*scyllav1.ScyllaCluster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaCluster %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	c.enqueue(sc)
}

func (c *Controller) addSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)
	klog.V(4).InfoS("Observed addition of Secret", "Secret", klog.KObj(secret))
	c.enqueueOwner(secret)
}

func (c *Controller) updateSecret(old, cur interface{}) {
	oldSecret := old.(*corev1.Secret)
	currentSecret := cur.(*corev1.Secret)

	if currentSecret.UID != oldSecret.UID {
		key, err := keyFunc(oldSecret)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSecret, err))
			return
		}
		c.deleteSecret(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSecret,
		})
	}

	klog.V(4).InfoS("Observed update of Secret", "Secret", klog.KObj(oldSecret))
	c.enqueueOwner(currentSecret)
}

func (c *Controller) deleteSecret(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		secret, ok = tombstone.Obj.(*corev1.Secret)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Secret %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Secret", "Secret", klog.KObj(secret))
	c.enqueueOwner(secret)
}
