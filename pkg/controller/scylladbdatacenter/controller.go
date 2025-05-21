package scylladbdatacenter

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
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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
	ControllerName = "ScyllaDBDatacenterController"

	artificialDelayForCachesToCatchUp = 10 * time.Second
)

var (
	keyFunc                  = cache.DeletionHandlingMetaNamespaceKeyFunc
	statefulSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

type Controller struct {
	operatorImage   string
	cqlsIngressPort int

	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	podLister                corev1listers.PodLister
	serviceLister            corev1listers.ServiceLister
	secretLister             corev1listers.SecretLister
	configMapLister          corev1listers.ConfigMapLister
	serviceAccountLister     corev1listers.ServiceAccountLister
	roleBindingLister        rbacv1listers.RoleBindingLister
	statefulSetLister        appsv1listers.StatefulSetLister
	pdbLister                policyv1listers.PodDisruptionBudgetLister
	ingressLister            networkingv1listers.IngressLister
	scyllaDBDatacenterLister scyllav1alpha1listers.ScyllaDBDatacenterLister
	jobLister                batchv1listers.JobLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.RateLimitingInterface
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBDatacenter]

	keyGetter crypto.RSAKeyGetter
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	podInformer corev1informers.PodInformer,
	serviceInformer corev1informers.ServiceInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	statefulSetInformer appsv1informers.StatefulSetInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	ingressInformer networkingv1informers.IngressInformer,
	jobInformer batchv1informers.JobInformer,
	scyllaDBDatacenterInformer scyllav1alpha1informers.ScyllaDBDatacenterInformer,
	operatorImage string,
	cqlsIngressPort int,
	keyGetter crypto.RSAKeyGetter,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	sdcc := &Controller{
		operatorImage:   operatorImage,
		cqlsIngressPort: cqlsIngressPort,

		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		podLister:                podInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		secretLister:             secretInformer.Lister(),
		configMapLister:          configMapInformer.Lister(),
		serviceAccountLister:     serviceAccountInformer.Lister(),
		roleBindingLister:        roleBindingInformer.Lister(),
		statefulSetLister:        statefulSetInformer.Lister(),
		pdbLister:                pdbInformer.Lister(),
		ingressLister:            ingressInformer.Lister(),
		scyllaDBDatacenterLister: scyllaDBDatacenterInformer.Lister(),
		jobLister:                jobInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			podInformer.Informer().HasSynced,
			serviceInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
			roleBindingInformer.Informer().HasSynced,
			statefulSetInformer.Informer().HasSynced,
			pdbInformer.Informer().HasSynced,
			ingressInformer.Informer().HasSynced,
			scyllaDBDatacenterInformer.Informer().HasSynced,
			jobInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbdatacenter-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scylladbdatacenter"),

		keyGetter: keyGetter,
	}

	var err error
	sdcc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBDatacenter](
		sdcc.queue,
		keyFunc,
		scheme.Scheme,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.ScyllaDBDatacenter]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.ScyllaDBDatacenter, error) {
				return sdcc.scyllaDBDatacenterLister.ScyllaDBDatacenters(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.ScyllaDBDatacenter, err error) {
				return sdcc.scyllaDBDatacenterLister.ScyllaDBDatacenters(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addService,
		UpdateFunc: sdcc.updateService,
		DeleteFunc: sdcc.deleteService,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addSecret,
		UpdateFunc: sdcc.updateSecret,
		DeleteFunc: sdcc.deleteSecret,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addConfigMap,
		UpdateFunc: sdcc.updateConfigMap,
		DeleteFunc: sdcc.deleteConfigMap,
	})

	// We need pods events to know if a pod is ready after replace operation.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addPod,
		UpdateFunc: sdcc.updatePod,
		DeleteFunc: sdcc.deletePod,
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addServiceAccount,
		UpdateFunc: sdcc.updateServiceAccount,
		DeleteFunc: sdcc.deleteServiceAccount,
	})

	roleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addRoleBinding,
		UpdateFunc: sdcc.updateRoleBinding,
		DeleteFunc: sdcc.deleteRoleBinding,
	})

	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addStatefulSet,
		UpdateFunc: sdcc.updateStatefulSet,
		DeleteFunc: sdcc.deleteStatefulSet,
	})

	pdbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addPodDisruptionBudget,
		UpdateFunc: sdcc.updatePodDisruptionBudget,
		DeleteFunc: sdcc.deletePodDisruptionBudget,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addIngress,
		UpdateFunc: sdcc.updateIngress,
		DeleteFunc: sdcc.deleteIngress,
	})

	scyllaDBDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addScyllaDBDatacenter,
		UpdateFunc: sdcc.updateScyllaDBDatacenter,
		DeleteFunc: sdcc.deleteScyllaDBDatacenter,
	})

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sdcc.addJob,
		UpdateFunc: sdcc.updateJob,
		DeleteFunc: sdcc.deleteJob,
	})

	return sdcc, nil
}

func (sdcc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := sdcc.queue.Get()
	if quit {
		return false
	}
	defer sdcc.queue.Done(key)

	err := sdcc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		sdcc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	sdcc.queue.AddRateLimited(key)

	return true
}

func (sdcc *Controller) runWorker(ctx context.Context) {
	for sdcc.processNextItem(ctx) {
	}
}

func (sdcc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		sdcc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), sdcc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, sdcc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (sdcc *Controller) resolveScyllaDBDatacenterController(obj metav1.Object) *scyllav1alpha1.ScyllaDBDatacenter {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != scyllav1alpha1.ScyllaDBDatacenterGVK.Kind {
		return nil
	}

	sc, err := sdcc.scyllaDBDatacenterLister.ScyllaDBDatacenters(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sc.UID != controllerRef.UID {
		return nil
	}

	return sc
}

func (sdcc *Controller) resolveStatefulSetController(obj metav1.Object) *appsv1.StatefulSet {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != statefulSetControllerGVK.Kind {
		return nil
	}

	sts, err := sdcc.statefulSetLister.StatefulSets(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sts.UID != controllerRef.UID {
		return nil
	}

	return sts
}

func (sdcc *Controller) resolveScyllaDBDatacenterControllerThroughStatefulSet(obj metav1.Object) *scyllav1alpha1.ScyllaDBDatacenter {
	sts := sdcc.resolveStatefulSetController(obj)
	if sts == nil {
		return nil
	}
	sdc := sdcc.resolveScyllaDBDatacenterController(sts)
	if sdc == nil {
		return nil
	}

	return sdc
}

func (sdcc *Controller) enqueueOwnerThroughStatefulSetOwner(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	sts := sdcc.resolveStatefulSetController(obj)
	if sts == nil {
		return
	}

	sdc := sdcc.resolveScyllaDBDatacenterController(sts)
	if sdc == nil {
		return
	}

	klog.V(4).InfoS("Enqueuing owner of StatefulSet", "StatefulSet", klog.KObj(sdc), "ScyllaDBDatacenter", klog.KObj(sdc))
	sdcc.handlers.Enqueue(depth+1, sdc, op)
}

func (sdcc *Controller) addService(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.Service),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateService(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteService,
	)
}

func (sdcc *Controller) deleteService(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addSecret(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateSecret(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteSecret,
	)
}

func (sdcc *Controller) deleteSecret(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addConfigMap(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateConfigMap(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteConfigMap,
	)
}

func (sdcc *Controller) deleteConfigMap(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addServiceAccount(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.ServiceAccount),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateServiceAccount(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.ServiceAccount),
		cur.(*corev1.ServiceAccount),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteServiceAccount,
	)
}

func (sdcc *Controller) deleteServiceAccount(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addRoleBinding(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*rbacv1.RoleBinding),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateRoleBinding(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*rbacv1.RoleBinding),
		cur.(*rbacv1.RoleBinding),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteRoleBinding,
	)
}

func (sdcc *Controller) deleteRoleBinding(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addPod(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.Pod),
		sdcc.enqueueOwnerThroughStatefulSetOwner,
	)
}

func (sdcc *Controller) updatePod(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.Pod),
		cur.(*corev1.Pod),
		sdcc.enqueueOwnerThroughStatefulSetOwner,
		sdcc.deletePod,
	)
}

func (sdcc *Controller) deletePod(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.enqueueOwnerThroughStatefulSetOwner,
	)
}

func (sdcc *Controller) addStatefulSet(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*appsv1.StatefulSet),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateStatefulSet(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*appsv1.StatefulSet),
		cur.(*appsv1.StatefulSet),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteStatefulSet,
	)
}

func (sdcc *Controller) deleteStatefulSet(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addPodDisruptionBudget(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*policyv1.PodDisruptionBudget),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*policyv1.PodDisruptionBudget),
		cur.(*policyv1.PodDisruptionBudget),
		sdcc.handlers.EnqueueOwner,
		sdcc.deletePodDisruptionBudget,
	)
}

func (sdcc *Controller) deletePodDisruptionBudget(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addIngress(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*networkingv1.Ingress),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateIngress(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*networkingv1.Ingress),
		cur.(*networkingv1.Ingress),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteIngress,
	)
}

func (sdcc *Controller) deleteIngress(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) addScyllaDBDatacenter(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBDatacenter),
		sdcc.handlers.Enqueue,
	)
}

func (sdcc *Controller) updateScyllaDBDatacenter(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBDatacenter),
		cur.(*scyllav1alpha1.ScyllaDBDatacenter),
		sdcc.handlers.Enqueue,
		sdcc.deleteScyllaDBDatacenter,
	)
}

func (sdcc *Controller) deleteScyllaDBDatacenter(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.Enqueue,
	)
}

func (sdcc *Controller) addJob(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*batchv1.Job),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateJob(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*batchv1.Job),
		cur.(*batchv1.Job),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteJob,
	)
}

func (sdcc *Controller) deleteJob(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}
