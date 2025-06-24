// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

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
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ControllerName = "ScyllaDBManagerClusterRegistrationController"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	// Unfortunately, Scylla Manager calls are synchronous, internally retried and can take ages.
	// Contrary to what it should be, this needs to be quite high.
	// FIXME: https://github.com/scylladb/scylla-operator/issues/2686
	maxSyncDuration = 2 * time.Minute
)

var (
	keyFunc                                         = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaDBManagerClusterRegistrationControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBManagerClusterRegistration")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	scyllaDBManagerClusterRegistrationLister scyllav1alpha1listers.ScyllaDBManagerClusterRegistrationLister
	scyllaDBDatacenterLister                 scyllav1alpha1listers.ScyllaDBDatacenterLister
	secretLister                             corev1listers.SecretLister
	namespaceLister                          corev1listers.NamespaceLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.TypedRateLimitingInterface[string]
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBManagerClusterRegistration]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	scyllaDBManagerClusterRegistrationInformer scyllav1alpha1informers.ScyllaDBManagerClusterRegistrationInformer,
	scyllaDBDatacenterInformer scyllav1alpha1informers.ScyllaDBDatacenterInformer,
	secretInformer corev1informers.SecretInformer,
	namespaceInformer corev1informers.NamespaceInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	smcrc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaDBManagerClusterRegistrationLister: scyllaDBManagerClusterRegistrationInformer.Lister(),
		scyllaDBDatacenterLister:                 scyllaDBDatacenterInformer.Lister(),
		secretLister:                             secretInformer.Lister(),
		namespaceLister:                          namespaceInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaDBManagerClusterRegistrationInformer.Informer().HasSynced,
			scyllaDBDatacenterInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			namespaceInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbmanagerclusterregistration-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "scylladbmanagerclusterregistration",
			},
		),
	}

	var err error
	smcrc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBManagerClusterRegistration](
		smcrc.queue,
		keyFunc,
		scheme.Scheme,
		scyllaDBManagerClusterRegistrationControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.ScyllaDBManagerClusterRegistration]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
				return smcrc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.ScyllaDBManagerClusterRegistration, err error) {
				return smcrc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	scyllaDBDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smcrc.addScyllaDBDatacenter,
		UpdateFunc: smcrc.updateScyllaDBDatacenter,
		DeleteFunc: smcrc.deleteScyllaDBDatacenter,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smcrc.addSecret,
		UpdateFunc: smcrc.updateSecret,
		DeleteFunc: smcrc.deleteSecret,
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smcrc.addNamespace,
		UpdateFunc: smcrc.updateNamespace,
		DeleteFunc: smcrc.deleteNamespace,
	})

	scyllaDBManagerClusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smcrc.addScyllaDBManagerClusterRegistration,
		UpdateFunc: smcrc.updateScyllaDBManagerClusterRegistration,
		DeleteFunc: smcrc.deleteScyllaDBManagerClusterRegistration,
	})

	return smcrc, nil
}

func (smcrc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := smcrc.queue.Get()
	if quit {
		return false
	}
	defer smcrc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := smcrc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		smcrc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	case controllertools.IsNonRetriable(err):
		klog.InfoS("Hit non-retriable error. Dropping the item from the queue.", "Error", err)
		smcrc.queue.Forget(key)
		return true

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))

	}

	smcrc.queue.AddRateLimited(key)

	return true
}

func (smcrc *Controller) runWorker(ctx context.Context) {
	for smcrc.processNextItem(ctx) {
	}
}

func (smcrc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		smcrc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), smcrc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, smcrc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (smcrc *Controller) addScyllaDBManagerClusterRegistration(obj interface{}) {
	smcrc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		smcrc.handlers.Enqueue,
	)
}

func (smcrc *Controller) updateScyllaDBManagerClusterRegistration(old, cur interface{}) {
	smcrc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		cur.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration),
		smcrc.handlers.Enqueue,
		smcrc.deleteScyllaDBManagerClusterRegistration,
	)
}

func (smcrc *Controller) deleteScyllaDBManagerClusterRegistration(obj interface{}) {
	smcrc.handlers.HandleDelete(
		obj,
		smcrc.handlers.Enqueue,
	)
}

func (smcrc *Controller) addScyllaDBDatacenter(obj interface{}) {
	smcrc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBDatacenter),
		smcrc.enqueueThroughScyllaDBDatacenter,
	)
}

func (smcrc *Controller) updateScyllaDBDatacenter(old, cur interface{}) {
	smcrc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBDatacenter),
		cur.(*scyllav1alpha1.ScyllaDBDatacenter),
		smcrc.enqueueThroughScyllaDBDatacenter,
		smcrc.deleteScyllaDBDatacenter,
	)
}

func (smcrc *Controller) deleteScyllaDBDatacenter(obj interface{}) {
	smcrc.handlers.HandleDelete(
		obj,
		smcrc.enqueueThroughScyllaDBDatacenter,
	)
}

func (smcrc *Controller) addSecret(obj interface{}) {
	smcrc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		smcrc.enqueueThroughOwner,
	)
}

func (smcrc *Controller) updateSecret(old, cur interface{}) {
	smcrc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		smcrc.enqueueThroughOwner,
		smcrc.deleteSecret,
	)
}

func (smcrc *Controller) deleteSecret(obj interface{}) {
	smcrc.handlers.HandleDelete(
		obj.(*corev1.Secret),
		smcrc.enqueueThroughOwner,
	)
}

func (smcrc *Controller) addNamespace(obj interface{}) {
	smcrc.handlers.HandleAdd(
		obj.(*corev1.Namespace),
		smcrc.enqueueThroughGlobalScyllaDBManagerNamespace,
	)
}

func (smcrc *Controller) updateNamespace(old, cur interface{}) {
	smcrc.handlers.HandleUpdate(
		old.(*corev1.Namespace),
		cur.(*corev1.Namespace),
		smcrc.enqueueThroughGlobalScyllaDBManagerNamespace,
		smcrc.deleteNamespace,
	)
}

func (smcrc *Controller) deleteNamespace(obj interface{}) {
	smcrc.handlers.HandleDelete(
		obj,
		smcrc.enqueueThroughGlobalScyllaDBManagerNamespace,
	)
}

func (smcrc *Controller) enqueueThroughScyllaDBDatacenter(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	sdc := obj.(*scyllav1alpha1.ScyllaDBDatacenter)

	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	smcr, err := smcrc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(sdc.Namespace).Get(smcrName)
	if err != nil {
		return
	}

	klog.V(4).InfoSDepth(depth, "Enqueuing ScyllaDBManagerClusterRegistration for ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KObj(sdc), "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))
	smcrc.handlers.Enqueue(depth+1, smcr, op)
}

func (smcrc *Controller) enqueueThroughOwner(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return
	}

	switch controllerRef.Kind {
	case scyllav1alpha1.ScyllaDBDatacenterGVK.Kind:
		sdc, err := smcrc.scyllaDBDatacenterLister.ScyllaDBDatacenters(obj.GetNamespace()).Get(controllerRef.Name)
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return
		}

		smcrc.enqueueThroughScyllaDBDatacenter(depth+1, sdc, op)
		return

	default:
		// Nothing to do.
		return

	}
}

func (smcrc *Controller) enqueueThroughGlobalScyllaDBManagerNamespace(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	ns := obj.(*corev1.Namespace)

	if ns.Name != naming.ScyllaManagerNamespace {
		return
	}

	smcrs, err := smcrc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(corev1.NamespaceAll).List(naming.GlobalScyllaDBManagerClusterRegistrationSelector())
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing ScyllaDBManagerClusterRegistrations for global ScyllaDB Manager Namespace")
	for _, smcr := range smcrs {
		smcrc.handlers.Enqueue(depth+1, smcr, op)
	}
}
