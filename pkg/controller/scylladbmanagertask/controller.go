// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

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
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaDBManagerTaskController"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	// Unfortunately, Scylla Manager calls are synchronous, internally retried and can take ages.
	// Contrary to what it should be, this needs to be quite high.
	// FIXME: https://github.com/scylladb/scylla-operator/issues/2686
	maxSyncDuration = 2 * time.Minute
)

var (
	keyFunc                          = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaDBManagerTaskControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBManagerTask")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	scyllaDBManagerTaskLister                scyllav1alpha1listers.ScyllaDBManagerTaskLister
	scyllaDBManagerClusterRegistrationLister scyllav1alpha1listers.ScyllaDBManagerClusterRegistrationLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.TypedRateLimitingInterface[string]
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBManagerTask]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	scyllaDBManagerTaskInformer scyllav1alpha1informers.ScyllaDBManagerTaskInformer,
	scylladbManagerClusterRegistrationInformer scyllav1alpha1informers.ScyllaDBManagerClusterRegistrationInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	smtc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaDBManagerTaskLister:                scyllaDBManagerTaskInformer.Lister(),
		scyllaDBManagerClusterRegistrationLister: scylladbManagerClusterRegistrationInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaDBManagerTaskInformer.Informer().HasSynced,
			scylladbManagerClusterRegistrationInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbmanagertask-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "scylladbmanagertask",
			},
		),
	}

	var err error
	smtc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBManagerTask](
		smtc.queue,
		keyFunc,
		scheme.Scheme,
		scyllaDBManagerTaskControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.ScyllaDBManagerTask]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.ScyllaDBManagerTask, error) {
				return smtc.scyllaDBManagerTaskLister.ScyllaDBManagerTasks(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.ScyllaDBManagerTask, err error) {
				return smtc.scyllaDBManagerTaskLister.ScyllaDBManagerTasks(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	scyllaDBManagerTaskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smtc.addScyllaDBManagerTask,
		UpdateFunc: smtc.updateScyllaDBManagerTask,
		DeleteFunc: smtc.deleteScyllaDBManagerTask,
	})

	scylladbManagerClusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smtc.addScyllaDBManagerClusterRegistration,
		UpdateFunc: smtc.updateScyllaDBManagerClusterRegistration,
		DeleteFunc: smtc.deleteScyllaDBManagerClusterRegistration,
	})

	return smtc, nil
}

func (smtc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := smtc.queue.Get()
	if quit {
		return false
	}
	defer smtc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := smtc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		smtc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		if controllertools.IsNonRetriable(err) {
			klog.InfoS("Hit non-retriable error. Dropping the item from the queue.", "Error", err)
			smtc.queue.Forget(key)
			return true
		}

		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))

	}

	smtc.queue.AddRateLimited(key)

	return true
}

func (smtc *Controller) runWorker(ctx context.Context) {
	for smtc.processNextItem(ctx) {
	}
}

func (smtc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		smtc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), smtc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, smtc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (smtc *Controller) addScyllaDBManagerTask(obj interface{}) {
	smtc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBManagerTask),
		smtc.handlers.Enqueue,
	)
}

func (smtc *Controller) updateScyllaDBManagerTask(old, cur interface{}) {
	smtc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBManagerTask),
		cur.(*scyllav1alpha1.ScyllaDBManagerTask),
		smtc.handlers.Enqueue,
		smtc.deleteScyllaDBManagerTask,
	)
}

func (smtc *Controller) deleteScyllaDBManagerTask(obj interface{}) {
	smtc.handlers.HandleDelete(
		obj,
		smtc.handlers.Enqueue,
	)
}

func (smtc *Controller) addScyllaDBManagerClusterRegistration(obj interface{}) {
	smtc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		smtc.enqueueThroughScyllaDBManagerClusterRegistration(obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)),
	)
}

func (smtc *Controller) updateScyllaDBManagerClusterRegistration(old, cur interface{}) {
	smtc.handlers.HandleUpdate(
		cur.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		old.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		smtc.enqueueThroughScyllaDBManagerClusterRegistration(cur.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)),
		smtc.deleteScyllaDBManagerClusterRegistration,
	)
}

func (smtc *Controller) deleteScyllaDBManagerClusterRegistration(obj interface{}) {
	smtc.handlers.HandleDelete(
		obj,
		smtc.enqueueThroughScyllaDBManagerClusterRegistration(obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)),
	)
}

func (smtc *Controller) enqueueThroughScyllaDBManagerClusterRegistration(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) controllerhelpers.EnqueueFuncType {
	return smtc.handlers.EnqueueAllFunc(smtc.handlers.EnqueueWithFilterFunc(func(smt *scyllav1alpha1.ScyllaDBManagerTask) bool {
		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return false
		}

		return smcr.Name == smcrName
	}))
}
