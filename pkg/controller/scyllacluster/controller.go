// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	networkingv1informers "k8s.io/client-go/informers/networking/v1"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaClusterController"
)

var (
	keyFunc                    = controllerhelpers.DeletionHandlingObjectToNamespacedName
	scyllaClusterControllerGVK = scyllav1.GroupVersion.WithKind("ScyllaCluster")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	serviceLister            corev1listers.ServiceLister
	secretLister             corev1listers.SecretLister
	configMapLister          corev1listers.ConfigMapLister
	serviceAccountLister     corev1listers.ServiceAccountLister
	roleBindingLister        rbacv1listers.RoleBindingLister
	statefulSetLister        appsv1listers.StatefulSetLister
	pdbLister                policyv1listers.PodDisruptionBudgetLister
	ingressLister            networkingv1listers.IngressLister
	jobLister                batchv1listers.JobLister
	scyllaClusterLister      scyllav1listers.ScyllaClusterLister
	scyllaDBDatacenterLister scyllav1alpha1listers.ScyllaDBDatacenterLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.TypedRateLimitingInterface[types.NamespacedName]
	handlers *controllerhelpers.Handlers[*scyllav1.ScyllaCluster]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	serviceInformer corev1informers.ServiceInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	statefulSetInformer appsv1informers.StatefulSetInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	ingressInformer networkingv1informers.IngressInformer,
	jobInformer batchv1informers.JobInformer,
	scyllaClusterInformer scyllav1informers.ScyllaClusterInformer,
	scyllaDBDatacenterInformer scyllav1alpha1informers.ScyllaDBDatacenterInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	scc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		serviceLister:            serviceInformer.Lister(),
		secretLister:             secretInformer.Lister(),
		configMapLister:          configMapInformer.Lister(),
		serviceAccountLister:     serviceAccountInformer.Lister(),
		roleBindingLister:        roleBindingInformer.Lister(),
		statefulSetLister:        statefulSetInformer.Lister(),
		pdbLister:                pdbInformer.Lister(),
		ingressLister:            ingressInformer.Lister(),
		scyllaClusterLister:      scyllaClusterInformer.Lister(),
		scyllaDBDatacenterLister: scyllaDBDatacenterInformer.Lister(),
		jobLister:                jobInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			serviceInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
			roleBindingInformer.Informer().HasSynced,
			statefulSetInformer.Informer().HasSynced,
			pdbInformer.Informer().HasSynced,
			ingressInformer.Informer().HasSynced,
			jobInformer.Informer().HasSynced,
			scyllaClusterInformer.Informer().HasSynced,
			scyllaDBDatacenterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllaclustermigration-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
				Name: "scyllaclustermigration",
			},
		),
	}

	var err error
	scc.handlers, err = controllerhelpers.NewHandlers[*scyllav1.ScyllaCluster](
		scc.queue,
		keyFunc,
		scheme.Scheme,
		scyllaClusterControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1.ScyllaCluster]{
			GetFunc: func(namespace, name string) (*scyllav1.ScyllaCluster, error) {
				return scc.scyllaClusterLister.ScyllaClusters(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1.ScyllaCluster, err error) {
				return scc.scyllaClusterLister.ScyllaClusters(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addService,
		UpdateFunc: scc.updateService,
		DeleteFunc: scc.deleteService,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addSecret,
		UpdateFunc: scc.updateSecret,
		DeleteFunc: scc.deleteSecret,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addConfigMap,
		UpdateFunc: scc.updateConfigMap,
		DeleteFunc: scc.deleteConfigMap,
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addServiceAccount,
		UpdateFunc: scc.updateServiceAccount,
		DeleteFunc: scc.deleteServiceAccount,
	})

	roleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addRoleBinding,
		UpdateFunc: scc.updateRoleBinding,
		DeleteFunc: scc.deleteRoleBinding,
	})

	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addStatefulSet,
		UpdateFunc: scc.updateStatefulSet,
		DeleteFunc: scc.deleteStatefulSet,
	})

	pdbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addPodDisruptionBudget,
		UpdateFunc: scc.updatePodDisruptionBudget,
		DeleteFunc: scc.deletePodDisruptionBudget,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addIngress,
		UpdateFunc: scc.updateIngress,
		DeleteFunc: scc.deleteIngress,
	})

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addScyllaCluster,
		UpdateFunc: scc.updateScyllaCluster,
		DeleteFunc: scc.deleteScyllaCluster,
	})

	scyllaDBDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addScyllaDBDatacenter,
		UpdateFunc: scc.updateScyllaDBDatacenter,
		DeleteFunc: scc.deleteScyllaDBDatacenter,
	})

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addJob,
		UpdateFunc: scc.updateJob,
		DeleteFunc: scc.deleteJob,
	})

	return scc, nil
}

func (scmc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := scmc.queue.Get()
	if quit {
		return false
	}
	defer scmc.queue.Done(key)

	err := scmc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		scmc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	scmc.queue.AddRateLimited(key)

	return true
}

func (scmc *Controller) runWorker(ctx context.Context) {
	for scmc.processNextItem(ctx) {
	}
}

func (scmc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		scmc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), scmc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, scmc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (scmc *Controller) addService(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*corev1.Service),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateService(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		scmc.handlers.EnqueueOwner,
		scmc.deleteService,
	)
}

func (scmc *Controller) deleteService(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addSecret(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateSecret(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		scmc.handlers.EnqueueOwner,
		scmc.deleteSecret,
	)
}

func (scmc *Controller) deleteSecret(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addConfigMap(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateConfigMap(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		scmc.handlers.EnqueueOwner,
		scmc.deleteConfigMap,
	)
}

func (scmc *Controller) deleteConfigMap(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addServiceAccount(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*corev1.ServiceAccount),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateServiceAccount(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*corev1.ServiceAccount),
		cur.(*corev1.ServiceAccount),
		scmc.handlers.EnqueueOwner,
		scmc.deleteServiceAccount,
	)
}

func (scmc *Controller) deleteServiceAccount(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addRoleBinding(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*rbacv1.RoleBinding),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateRoleBinding(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*rbacv1.RoleBinding),
		cur.(*rbacv1.RoleBinding),
		scmc.handlers.EnqueueOwner,
		scmc.deleteRoleBinding,
	)
}

func (scmc *Controller) deleteRoleBinding(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addStatefulSet(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*appsv1.StatefulSet),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateStatefulSet(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*appsv1.StatefulSet),
		cur.(*appsv1.StatefulSet),
		scmc.handlers.EnqueueOwner,
		scmc.deleteStatefulSet,
	)
}

func (scmc *Controller) deleteStatefulSet(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addPodDisruptionBudget(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*policyv1.PodDisruptionBudget),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*policyv1.PodDisruptionBudget),
		cur.(*policyv1.PodDisruptionBudget),
		scmc.handlers.EnqueueOwner,
		scmc.deletePodDisruptionBudget,
	)
}

func (scmc *Controller) deletePodDisruptionBudget(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addIngress(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*networkingv1.Ingress),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateIngress(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*networkingv1.Ingress),
		cur.(*networkingv1.Ingress),
		scmc.handlers.EnqueueOwner,
		scmc.deleteIngress,
	)
}

func (scmc *Controller) deleteIngress(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addScyllaDBDatacenter(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBDatacenter),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateScyllaDBDatacenter(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBDatacenter),
		cur.(*scyllav1alpha1.ScyllaDBDatacenter),
		scmc.handlers.EnqueueOwner,
		scmc.deleteScyllaDBDatacenter,
	)
}

func (scmc *Controller) deleteScyllaDBDatacenter(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) addScyllaCluster(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*scyllav1.ScyllaCluster),
		scmc.handlers.Enqueue,
	)
}

func (scmc *Controller) updateScyllaCluster(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*scyllav1.ScyllaCluster),
		cur.(*scyllav1.ScyllaCluster),
		scmc.handlers.Enqueue,
		scmc.deleteScyllaCluster,
	)
}

func (scmc *Controller) deleteScyllaCluster(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.Enqueue,
	)
}

func (scmc *Controller) addJob(obj interface{}) {
	scmc.handlers.HandleAdd(
		obj.(*batchv1.Job),
		scmc.handlers.EnqueueOwner,
	)
}

func (scmc *Controller) updateJob(old, cur interface{}) {
	scmc.handlers.HandleUpdate(
		old.(*batchv1.Job),
		cur.(*batchv1.Job),
		scmc.handlers.EnqueueOwner,
		scmc.deleteJob,
	)
}

func (scmc *Controller) deleteJob(obj interface{}) {
	scmc.handlers.HandleDelete(
		obj,
		scmc.handlers.EnqueueOwner,
	)
}
