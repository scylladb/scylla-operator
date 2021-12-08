// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "NodeConfigPodController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc       = cache.DeletionHandlingMetaNamespaceKeyFunc
	controllerGVK = corev1.SchemeGroupVersion.WithKind("Pod")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	podLister        corev1listers.PodLister
	configMapLister  corev1listers.ConfigMapLister
	nodeLister       corev1listers.NodeLister
	nodeConfigLister scyllav1alpha1listers.NodeConfigLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	podInformer corev1informers.PodInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	nodeInformer corev1informers.NodeInformer,
	nodeConfigInformer scyllav1alpha1informers.NodeConfigInformer,
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
	ncpc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		podLister:        podInformer.Lister(),
		configMapLister:  configMapInformer.Lister(),
		nodeLister:       nodeInformer.Lister(),
		nodeConfigLister: nodeConfigInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			podInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			nodeInformer.Informer().HasSynced,
			nodeConfigInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "NodeConfigCM-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeConfigCM"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncpc.addPod,
		UpdateFunc: ncpc.updatePod,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncpc.addConfigMap,
		UpdateFunc: ncpc.updateConfigMap,
		DeleteFunc: ncpc.deleteConfigMap,
	})

	nodeConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncpc.addNodeConfig,
		UpdateFunc: ncpc.updateNodeConfig,
		DeleteFunc: ncpc.deleteNodeConfig,
	})

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: ncpc.updateNode,
	})

	return ncpc, nil
}

func (ncpc *Controller) enqueue(pod *corev1.Pod) {
	key, err := keyFunc(pod)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", pod, err))
		return
	}

	klog.V(4).InfoS("Enqueuing Pod", "Pod", klog.KObj(pod))
	ncpc.queue.Add(key)
}

func (ncpc *Controller) resolvePod(obj metav1.Object) *corev1.Pod {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != controllerGVK.Kind {
		return nil
	}

	pod, err := ncpc.podLister.Pods(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if pod.UID != controllerRef.UID {
		return nil
	}

	return pod
}

func (ncpc *Controller) enqueueOwner(obj metav1.Object) {
	pod := ncpc.resolvePod(obj)
	if pod == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "Pod", klog.KObj(pod))
	ncpc.enqueue(pod)
}

func (ncpc *Controller) enqueueAllScyllaPodsOnNode(node *corev1.Node) {
	allPods, err := ncpc.podLister.List(naming.ScyllaSelector())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	var pods []*corev1.Pod
	for _, pod := range allPods {
		if pod.Spec.NodeName == node.Name {
			pods = append(pods, pod)
		}
	}

	klog.V(4).InfoS("Enqueuing all Scylla pods on Node", "Node", klog.KObj(node), "Count", len(pods))

	for _, pod := range pods {
		ncpc.enqueue(pod)
	}
}

func (ncpc *Controller) enqueueAllScyllaPodsForNodeConfig(nodeConfig *scyllav1alpha1.NodeConfig) {
	allNodes, err := ncpc.nodeLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	var nodes []*corev1.Node
	for _, node := range allNodes {
		matching, err := controllerhelpers.IsNodeConfigSelectingNode(nodeConfig, node)
		if err != nil {
			utilruntime.HandleError(err)
			return
		}

		if matching {
			nodes = append(nodes, node)
		}
	}

	klog.V(4).InfoS("Enqueuing all Scylla pods for NodeConfig", "NodeConfig", klog.KObj(nodeConfig), "NodeCount", len(nodes))
	for _, node := range nodes {
		ncpc.enqueueAllScyllaPodsOnNode(node)
	}
}

func (ncpc *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)

	// TODO: extract and use a better label, verify the container
	if pod.Labels == nil {
		return
	}

	_, isScyllaPod := pod.Labels[naming.ClusterNameLabel]
	if !isScyllaPod {
		return
	}

	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	ncpc.enqueue(pod)
}

func (ncpc *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	// TODO: extract and use a better label, verify the container
	if currentPod.Labels == nil {
		return
	}

	_, isScyllaPod := currentPod.Labels[naming.ClusterNameLabel]
	if !isScyllaPod {
		return
	}

	klog.V(4).InfoS("Observed update of Pod", "Pod", klog.KObj(oldPod))
	ncpc.enqueue(currentPod)
}

func (ncpc *Controller) addConfigMap(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(cm))
	ncpc.enqueueOwner(cm)
}

func (ncpc *Controller) updateConfigMap(old, cur interface{}) {
	oldCM := old.(*corev1.ConfigMap)
	currentCM := cur.(*corev1.ConfigMap)

	if currentCM.UID != oldCM.UID {
		key, err := keyFunc(oldCM)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldCM, err))
			return
		}
		ncpc.deleteConfigMap(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldCM,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldCM))
	ncpc.enqueueOwner(currentCM)
}

func (ncpc *Controller) deleteConfigMap(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		cm, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ConfigMap %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ConfigMap", "ConfigMap", klog.KObj(cm))
	ncpc.enqueueOwner(cm)
}

func (ncpc *Controller) updateNode(old, cur interface{}) {
	oldNode := old.(*corev1.Node)
	currentNode := cur.(*corev1.Node)

	klog.V(4).InfoS("Observed update of Node", "Node", klog.KObj(oldNode))
	ncpc.enqueueAllScyllaPodsOnNode(currentNode)
}

func (ncpc *Controller) addNodeConfig(obj interface{}) {
	nodeConfig := obj.(*scyllav1alpha1.NodeConfig)
	klog.V(4).InfoS("Observed addition of NodeConfig", "NodeConfig", klog.KObj(nodeConfig))
	ncpc.enqueueAllScyllaPodsForNodeConfig(nodeConfig)
}

func (ncpc *Controller) updateNodeConfig(old, cur interface{}) {
	oldNodeConfig := old.(*scyllav1alpha1.NodeConfig)
	currentNodeConfig := cur.(*scyllav1alpha1.NodeConfig)

	if currentNodeConfig.UID != oldNodeConfig.UID {
		key, err := keyFunc(oldNodeConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNodeConfig, err))
			return
		}
		ncpc.deleteNodeConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNodeConfig,
		})
	}

	klog.V(4).InfoS("Observed update of NodeConfig", "NodeConfig", klog.KObj(oldNodeConfig))
	ncpc.enqueueAllScyllaPodsForNodeConfig(currentNodeConfig)
}

func (ncpc *Controller) deleteNodeConfig(obj interface{}) {
	nodeConfig, ok := obj.(*scyllav1alpha1.NodeConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		nodeConfig, ok = tombstone.Obj.(*scyllav1alpha1.NodeConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a NodeConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of NodeConfig", "NodeConfig", klog.KObj(nodeConfig))
	ncpc.enqueueAllScyllaPodsForNodeConfig(nodeConfig)
}

func (ncpc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := ncpc.queue.Get()
	if quit {
		return false
	}
	defer ncpc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := ncpc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		ncpc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	ncpc.queue.AddRateLimited(key)

	return true
}

func (ncpc *Controller) runWorker(ctx context.Context) {
	for ncpc.processNextItem(ctx) {
	}
}

func (ncpc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		ncpc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), ncpc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, ncpc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}
