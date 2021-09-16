// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "NodeConfigController"

	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc                = cache.DeletionHandlingMetaNamespaceKeyFunc
	controllerGVK          = scyllav1alpha1.GroupVersion.WithKind("ScyllaNodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	criClient cri.Client

	scyllaNodeConfigLister scyllav1alpha1listers.ScyllaNodeConfigLister
	podLister              corev1listers.PodLister
	configMapLister        corev1listers.ConfigMapLister
	daemonSetLister        appsv1listers.DaemonSetLister
	jobLister              batchv1listers.JobLister

	nodeName       string
	nodeConfigName string
	scyllaImage    string

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	criClient cri.Client,
	nodeConfigInformer scyllav1alpha1informers.ScyllaNodeConfigInformer,
	podInformer corev1informers.PodInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	daemonSetInformer appsv1informers.DaemonSetInformer,
	jobInformer batchv1informers.JobInformer,
	nodeName string,
	nodeConfigName string,
	scyllaImage string,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"nodeconfig_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	snc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,
		criClient:    criClient,

		scyllaNodeConfigLister: nodeConfigInformer.Lister(),
		podLister:              podInformer.Lister(),
		configMapLister:        configMapInformer.Lister(),
		daemonSetLister:        daemonSetInformer.Lister(),
		jobLister:              jobInformer.Lister(),

		nodeName:       nodeName,
		nodeConfigName: nodeConfigName,
		scyllaImage:    scyllaImage,

		cachesToSync: []cache.InformerSynced{
			nodeConfigInformer.Informer().HasSynced,
			podInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			daemonSetInformer.Informer().HasSynced,
			jobInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllanodeconfig-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllanodeconfig"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addPod,
		UpdateFunc: snc.updatePod,
		DeleteFunc: snc.deletePod,
	})

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addJob,
		UpdateFunc: snc.updateJob,
		DeleteFunc: snc.deleteJob,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addConfigMap,
		UpdateFunc: snc.updateConfigMap,
		DeleteFunc: snc.deleteConfigMap,
	})

	return snc, nil
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

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		c.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.cachesToSync...) {
		return
	}

	wg.Add(1)
	// Running tuning script in parallel on the same node is pointless.
	go func() {
		defer wg.Done()
		wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}()

	<-ctx.Done()
}

func (c *Controller) enqueueOwner(obj metav1.Object) {
	snt := c.resolveNodeConfigController(obj)
	if snt == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaNodeConfig", klog.KObj(snt))
	c.enqueue(snt)
}

func (c *Controller) enqueue(snt *scyllav1alpha1.ScyllaNodeConfig) {
	key, err := keyFunc(snt)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", snt, err))
		return
	}

	klog.V(4).InfoS("Enqueuing ScyllaNodeConfig", "key", key)
	c.queue.Add(key)
}

func (c *Controller) resolveNodeConfigController(obj metav1.Object) *scyllav1alpha1.ScyllaNodeConfig {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != controllerGVK.Kind {
		return nil
	}

	snt, err := c.scyllaNodeConfigLister.Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if snt.UID != controllerRef.UID {
		return nil
	}

	return snt
}

func (c *Controller) enqueueThroughDaemonSetControllerOrScyllaPod(pod *corev1.Pod) {
	// Informers should only notify about Scylla Pods on the same node.
	if !naming.ScyllaSelector().Matches(labels.Set(pod.GetLabels())) {
		return
	}

	if pod.Spec.NodeName != c.nodeName {
		return
	}

	var err error
	snc, err := c.scyllaNodeConfigLister.Get(c.nodeConfigName)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing through Scylla Pod", "Pod", klog.KObj(pod), "ScyllaNodeConfig", klog.KObj(snc))
	c.enqueue(snc)
	return
}

func (c *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(pod)
}

func (c *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	if currentPod.UID != oldPod.UID {
		key, err := keyFunc(oldPod)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPod, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPod,
		})
	}

	klog.V(4).InfoS("Observed update of Pod", "Pod", klog.KObj(oldPod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(currentPod)
}

func (c *Controller) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Pod %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Pod", "Pod", klog.KObj(pod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(pod)
}

func (c *Controller) addJob(obj interface{}) {
	job := obj.(*batchv1.Job)
	klog.V(4).InfoS("Observed addition of Job", "Job", klog.KObj(job))
	c.enqueueOwner(job)
}

func (c *Controller) updateJob(old, cur interface{}) {
	oldJob := old.(*batchv1.Job)
	currentJob := cur.(*batchv1.Job)

	if currentJob.UID != oldJob.UID {
		key, err := keyFunc(oldJob)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldJob, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldJob,
		})
	}

	klog.V(4).InfoS("Observed update of Job", "Job", klog.KObj(oldJob))
	c.enqueueOwner(currentJob)
}

func (c *Controller) deleteJob(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		job, ok = tombstone.Obj.(*batchv1.Job)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Job", "Job", klog.KObj(job))
	c.enqueueOwner(job)
}

func (c *Controller) addConfigMap(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(cm))
	c.enqueueOwner(cm)
}

func (c *Controller) updateConfigMap(old, cur interface{}) {
	oldConfigMap := old.(*corev1.ConfigMap)
	currentConfigMap := cur.(*corev1.ConfigMap)

	if currentConfigMap.UID != oldConfigMap.UID {
		key, err := keyFunc(oldConfigMap)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldConfigMap, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldConfigMap,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldConfigMap))
	c.enqueueOwner(currentConfigMap)
}

func (c *Controller) deleteConfigMap(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		configMap, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ConfigMap", "ConfigMap", klog.KObj(configMap))
	c.enqueueOwner(configMap)
}
