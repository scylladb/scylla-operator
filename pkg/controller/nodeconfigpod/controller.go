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
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "NodeConfigPodController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc          = controllerhelpers.DeletionHandlingObjectToNamespacedName
	podControllerGVK = corev1.SchemeGroupVersion.WithKind("Pod")
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

	queue    workqueue.TypedRateLimitingInterface[types.NamespacedName]
	handlers *controllerhelpers.Handlers[*corev1.Pod]
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
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
				Name: "NodeConfigCM",
			},
		),
	}

	var err error
	ncpc.handlers, err = controllerhelpers.NewHandlers[*corev1.Pod](
		ncpc.queue,
		keyFunc,
		scheme.Scheme,
		podControllerGVK,
		kubeinterfaces.NamespacedGetList[*corev1.Pod]{
			GetFunc: func(namespace, name string) (*corev1.Pod, error) {
				return ncpc.podLister.Pods(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*corev1.Pod, err error) {
				return ncpc.podLister.Pods(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
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

func (ncpc *Controller) enqueueScyllaPodFunc() controllerhelpers.EnqueueFuncType {
	return ncpc.handlers.EnqueueWithFilterFunc(controllerhelpers.IsScyllaPod)
}

func (ncpc *Controller) enqueueAllScyllaPodsOnNode(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	node := obj.(*corev1.Node)

	allPods, err := ncpc.podLister.List(naming.ScyllaSelector())
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	var pods []*corev1.Pod
	for _, pod := range allPods {
		if pod.Spec.NodeName == node.Name {
			pods = append(pods, pod)
		}
	}

	klog.V(4).InfoSDepth(depth, "Enqueuing all pods on Node", "Pods", len(pods), "Node", klog.KObj(node))
	for _, pod := range pods {
		ncpc.handlers.Enqueue(depth+1, pod, op)
	}

	return
}

func (ncpc *Controller) enqueueAllScyllaPodsForNodeConfig(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	nodeConfig := obj.(*scyllav1alpha1.NodeConfig)

	allNodes, err := ncpc.nodeLister.List(labels.Everything())
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	var nodes []*corev1.Node
	for _, node := range allNodes {
		matching, err := controllerhelpers.IsNodeConfigSelectingNode(nodeConfig, node)
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return
		}

		if matching {
			nodes = append(nodes, node)
		}
	}

	klog.V(4).InfoS("Enqueuing all Scylla Pods for NodeConfig", "NodeConfig", klog.KObj(nodeConfig), "NodeCount", len(nodes))
	for _, node := range nodes {
		ncpc.enqueueAllScyllaPodsOnNode(depth+1, node, op)
	}
}

func (ncpc *Controller) addPod(obj interface{}) {
	ncpc.handlers.HandleAdd(
		obj.(*corev1.Pod),
		ncpc.enqueueScyllaPodFunc(),
	)
}

func (ncpc *Controller) updatePod(old, cur interface{}) {
	ncpc.handlers.HandleUpdate(
		old.(*corev1.Pod),
		cur.(*corev1.Pod),
		ncpc.enqueueScyllaPodFunc(),
		nil,
	)
}

func (ncpc *Controller) addConfigMap(obj interface{}) {
	ncpc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		ncpc.handlers.EnqueueOwnerFunc(ncpc.enqueueScyllaPodFunc()),
	)
}

func (ncpc *Controller) updateConfigMap(old, cur interface{}) {
	ncpc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		ncpc.handlers.EnqueueOwnerFunc(ncpc.enqueueScyllaPodFunc()),
		ncpc.deleteConfigMap,
	)
}

func (ncpc *Controller) deleteConfigMap(obj interface{}) {
	ncpc.handlers.HandleDelete(
		obj,
		ncpc.handlers.EnqueueOwnerFunc(ncpc.enqueueScyllaPodFunc()),
	)
}

func (ncpc *Controller) updateNode(old, cur interface{}) {
	ncpc.handlers.HandleUpdate(
		old.(*corev1.Node),
		cur.(*corev1.Node),
		ncpc.enqueueAllScyllaPodsOnNode,
		nil,
	)
}

func (ncpc *Controller) addNodeConfig(obj interface{}) {
	ncpc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.NodeConfig),
		ncpc.enqueueAllScyllaPodsForNodeConfig,
	)
}

func (ncpc *Controller) updateNodeConfig(old, cur interface{}) {
	ncpc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.NodeConfig),
		cur.(*scyllav1alpha1.NodeConfig),
		ncpc.enqueueAllScyllaPodsForNodeConfig,
		ncpc.deleteNodeConfig,
	)
}

func (ncpc *Controller) deleteNodeConfig(obj interface{}) {
	ncpc.handlers.HandleDelete(
		obj,
		ncpc.enqueueAllScyllaPodsForNodeConfig,
	)
}

func (ncpc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := ncpc.queue.Get()
	if quit {
		return false
	}
	defer ncpc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := ncpc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		ncpc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	ncpc.queue.AddRateLimited(key)

	return true
}

func (ncpc *Controller) runWorker(ctx context.Context) {
	for ncpc.processNextItem(ctx) {
	}
}

func (ncpc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

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

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, ncpc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}
