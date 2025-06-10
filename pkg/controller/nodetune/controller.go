// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/kubelet"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
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
	"k8s.io/klog/v2"
)

const (
	ControllerName = "NodeConfigDaemonController"

	maxSyncDuration = 30 * time.Second
)

var (
	controllerKey = types.NamespacedName{Namespace: "", Name: "key"}
	keyFunc       = controllerhelpers.DeletionHandlingObjectToNamespacedName

	nodeConfigGVK          = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	criClient                 cri.Client
	kubeletPodResourcesClient kubelet.PodResourcesClient

	nodeConfigLister          scyllav1alpha1listers.NodeConfigLister
	localScyllaPodsLister     corev1listers.PodLister
	namespacedDaemonSetLister appsv1listers.DaemonSetLister
	namespacedJobLister       batchv1listers.JobLister
	selfPodLister             corev1listers.PodLister

	namespace      string
	podName        string
	nodeName       string
	nodeUID        types.UID
	nodeConfigName string
	nodeConfigUID  types.UID
	scyllaImage    string
	operatorImage  string

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.TypedRateLimitingInterface[types.NamespacedName]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	criClient cri.Client,
	kubeletPodResourcesClient kubelet.PodResourcesClient,
	nodeConfigInformer scyllav1alpha1informers.NodeConfigInformer,
	localScyllaPodsInformer corev1informers.PodInformer,
	namespacedDaemonSetInformer appsv1informers.DaemonSetInformer,
	namespacedJobInformer batchv1informers.JobInformer,
	selfPodInformer corev1informers.PodInformer,
	namespace string,
	podName string,
	nodeName string,
	nodeUID types.UID,
	nodeConfigName string,
	nodeConfigUID types.UID,
	scyllaImage string,
	operatorImage string,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	snc := &Controller{
		kubeClient:                kubeClient,
		scyllaClient:              scyllaClient,
		criClient:                 criClient,
		kubeletPodResourcesClient: kubeletPodResourcesClient,

		nodeConfigLister:          nodeConfigInformer.Lister(),
		localScyllaPodsLister:     localScyllaPodsInformer.Lister(),
		namespacedDaemonSetLister: namespacedDaemonSetInformer.Lister(),
		namespacedJobLister:       namespacedJobInformer.Lister(),
		selfPodLister:             selfPodInformer.Lister(),

		namespace:      namespace,
		podName:        podName,
		nodeName:       nodeName,
		nodeUID:        nodeUID,
		nodeConfigName: nodeConfigName,
		nodeConfigUID:  nodeConfigUID,
		scyllaImage:    scyllaImage,
		operatorImage:  operatorImage,

		cachesToSync: []cache.InformerSynced{
			nodeConfigInformer.Informer().HasSynced,
			localScyllaPodsInformer.Informer().HasSynced,
			namespacedDaemonSetInformer.Informer().HasSynced,
			namespacedJobInformer.Informer().HasSynced,
			selfPodInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "nodeconfigdaemon-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
				Name: "nodeconfigdaemon",
			},
		),
	}

	nodeConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addNodeConfig,
		UpdateFunc: snc.updateNodeConfig,
		DeleteFunc: snc.deleteNodeConfig,
	})

	localScyllaPodsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addPod,
		UpdateFunc: snc.updatePod,
	})

	namespacedJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addJob,
		UpdateFunc: snc.updateJob,
		DeleteFunc: snc.deleteJob,
	})

	// Start right away, Scylla might not be scheduled yet, but Node can already be tuned.
	snc.enqueue()

	return snc, nil
}

func (ncdc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := ncdc.queue.Get()
	if quit {
		return false
	}
	defer ncdc.queue.Done(key)

	ctx, cancel := context.WithTimeoutCause(ctx, maxSyncDuration, fmt.Errorf("exceeded max sync duration (%v)", maxSyncDuration))
	defer cancel()
	err := ncdc.sync(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		ncdc.queue.Forget(key)
		return true
	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	ncdc.queue.AddRateLimited(key)

	return true
}

func (ncdc *Controller) runWorker(ctx context.Context) {
	for ncdc.processNextItem(ctx) {
	}
}

func (ncdc *Controller) Run(ctx context.Context) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		ncdc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), ncdc.cachesToSync...) {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		apimachineryutilwait.UntilWithContext(ctx, ncdc.runWorker, time.Second)
	}()

	<-ctx.Done()
}

func (ncdc *Controller) enqueue() {
	ncdc.queue.Add(controllerKey)
}

func (ncdc *Controller) addNodeConfig(obj interface{}) {
	nc := obj.(*scyllav1alpha1.NodeConfig)

	if !ncdc.isNodeConfigControlled(nc) {
		klog.V(5).InfoS("Not enqueueing NodeConfig not controlled by us", "NodeConfig", klog.KObj(nc), "RV", nc.ResourceVersion)
		return
	}

	klog.V(4).InfoS("Observed addition of NodeConfig", "NodeConfig", klog.KObj(nc), "RV", nc.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) updateNodeConfig(old, cur interface{}) {
	oldNC := old.(*scyllav1alpha1.NodeConfig)
	currentNC := cur.(*scyllav1alpha1.NodeConfig)

	if currentNC.UID != oldNC.UID {
		key, err := keyFunc(oldNC)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNC, err))
			return
		}
		ncdc.deleteNodeConfig(controllerhelpers.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNC,
		})
	}

	controlled := ncdc.isNodeConfigControlled(currentNC)
	if !controlled {
		klog.V(5).InfoS("Not enqueueing NodeConfig not controlled by us", "NodeConfig", klog.KObj(currentNC), "RV", currentNC.ResourceVersion)
		return
	}

	klog.V(4).InfoS(
		"Observed update of NodeConfig",
		"NodeConfig", klog.KObj(currentNC),
		"RV", fmt.Sprintf("%s->%s", oldNC.ResourceVersion, currentNC.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldNC.UID, currentNC.UID),
	)
	ncdc.enqueue()
}

func (ncdc *Controller) deleteNodeConfig(obj interface{}) {
	nc, ok := obj.(*scyllav1alpha1.NodeConfig)
	if !ok {
		tombstone, ok := obj.(controllerhelpers.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		nc, ok = tombstone.Obj.(*scyllav1alpha1.NodeConfig)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a NodeConfig %#v", obj))
			return
		}
	}

	controlled := ncdc.isNodeConfigControlled(nc)
	if !controlled {
		klog.V(5).InfoS("Not enqueueing NodeConfig not controlled by us", "NodeConfig", klog.KObj(nc), "RV", nc.ResourceVersion)
		return
	}

	klog.V(4).InfoS("Observed deletion of NodeConfig", "NodeConfig", klog.KObj(nc), "RV", nc.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	ncdc.enqueue()
}

func (ncdc *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	klog.V(4).InfoS(
		"Observed update of Pod",
		"Pod", klog.KObj(currentPod),
		"RV", fmt.Sprintf("%s-%s", oldPod.ResourceVersion, currentPod.ResourceVersion),
		"UID", fmt.Sprintf("%s-%s", oldPod.UID, currentPod.UID),
	)
	ncdc.enqueue()
}

func (ncdc *Controller) ownsObject(obj metav1.Object) (bool, error) {
	selfRef, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return false, fmt.Errorf("can't get self controller ref: %w", err)
	}

	objControllerRef := metav1.GetControllerOfNoCopy(obj)

	klog.V(5).InfoS("checking object owner", "ObjectRef", objControllerRef, "SelfRef", selfRef)

	return apiequality.Semantic.DeepEqual(objControllerRef, selfRef), nil
}

func (ncdc *Controller) addJob(obj interface{}) {
	job := obj.(*batchv1.Job)

	owned, err := ncdc.ownsObject(job)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	if !owned {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(job), "RV", job.ResourceVersion)
		return
	}

	klog.V(4).InfoS("Observed addition of Job", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) updateJob(old, cur interface{}) {
	oldJob := old.(*batchv1.Job)
	currentJob := cur.(*batchv1.Job)

	if currentJob.UID != oldJob.UID {
		key, err := keyFunc(oldJob)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldJob, err))
			return
		}
		ncdc.deleteJob(controllerhelpers.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldJob,
		})
	}

	owned, err := ncdc.ownsObject(currentJob)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	if !owned {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(currentJob), "RV", currentJob.ResourceVersion)
		return
	}

	klog.V(4).InfoS(
		"Observed update of Job",
		"Job", klog.KObj(currentJob),
		"RV", fmt.Sprintf("%s->%s", oldJob.ResourceVersion, currentJob.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldJob.UID, currentJob.UID),
	)
	ncdc.enqueue()
}

func (ncdc *Controller) deleteJob(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		tombstone, ok := obj.(controllerhelpers.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		job, ok = tombstone.Obj.(*batchv1.Job)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}

	owned, err := ncdc.ownsObject(job)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	if !owned {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(job), "RV", job.ResourceVersion)
		return
	}

	klog.V(4).InfoS("Observed deletion of Job", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) newOwningDSControllerRef() (*metav1.OwnerReference, error) {
	pod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get self Pod %q: %w", naming.ManualRef(ncdc.namespace, ncdc.podName), err)
	}

	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return nil, fmt.Errorf("pod %q doesn't have a controller refference", naming.ObjRef(pod))
	}

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse GroupVersion %q: %w", ref.APIVersion, err)
	}

	if gv.Group != daemonSetControllerGVK.Group {
		return nil, fmt.Errorf("pod's onwer ref group %q doesn't match the expected group %q", gv.Group, daemonSetControllerGVK.Group)

	}
	if ref.Kind != daemonSetControllerGVK.Kind {
		return nil, fmt.Errorf("pod's onwer ref kind %q doesn't match the expected kind %q", ref.Kind, daemonSetControllerGVK.Kind)
	}

	return ref, nil
}

func (ncdc *Controller) newNodeConfigObjectRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion:      nodeConfigGVK.Version,
		Kind:            nodeConfigGVK.Kind,
		Name:            ncdc.nodeConfigName,
		Namespace:       corev1.NamespaceAll,
		UID:             ncdc.nodeConfigUID,
		ResourceVersion: "",
	}
}

func (ncdc *Controller) isNodeConfigControlled(nc *scyllav1alpha1.NodeConfig) bool {
	return nc.Name == ncdc.nodeConfigName && nc.UID == ncdc.nodeConfigUID
}
