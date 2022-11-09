// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	networkingv1informers "k8s.io/client-go/informers/networking/v1"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaDatacenterController"

	artificialDelayForCachesToCatchUp = 10 * time.Second
)

var (
	keyFunc                       = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaDatacenterControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDatacenter")
	statefulSetControllerGVK      = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

type Controller struct {
	operatorImage   string
	cqlsIngressPort int

	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	podLister            corev1listers.PodLister
	serviceLister        corev1listers.ServiceLister
	secretLister         corev1listers.SecretLister
	configMapLister      corev1listers.ConfigMapLister
	serviceAccountLister corev1listers.ServiceAccountLister
	roleBindingLister    rbacv1listers.RoleBindingLister
	statefulSetLister    appsv1listers.StatefulSetLister
	pdbLister            policyv1listers.PodDisruptionBudgetLister
	ingressLister        networkingv1listers.IngressLister
	scyllaLister         scyllav1alpha1listers.ScyllaDatacenterLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
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
	scyllaDatacenterInformer scyllav1alpha1informers.ScyllaDatacenterInformer,
	operatorImage string,
	cqlsIngressPort int,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllacluster_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	scc := &Controller{
		operatorImage:   operatorImage,
		cqlsIngressPort: cqlsIngressPort,

		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		podLister:            podInformer.Lister(),
		serviceLister:        serviceInformer.Lister(),
		secretLister:         secretInformer.Lister(),
		configMapLister:      configMapInformer.Lister(),
		serviceAccountLister: serviceAccountInformer.Lister(),
		roleBindingLister:    roleBindingInformer.Lister(),
		statefulSetLister:    statefulSetInformer.Lister(),
		pdbLister:            pdbInformer.Lister(),
		ingressLister:        ingressInformer.Lister(),
		scyllaLister:         scyllaDatacenterInformer.Lister(),

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
			scyllaDatacenterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllacluster-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllacluster"),
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

	// We need pods events to know if a pod is ready after replace operation.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addPod,
		UpdateFunc: scc.updatePod,
		DeleteFunc: scc.deletePod,
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

	scyllaDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addScyllaDatacenter,
		UpdateFunc: scc.updateScyllaDatacenter,
		DeleteFunc: scc.deleteScyllaDatacenter,
	})

	return scc, nil
}

func (sdc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := sdc.queue.Get()
	if quit {
		return false
	}
	defer sdc.queue.Done(key)

	err := sdc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		sdc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	sdc.queue.AddRateLimited(key)

	return true
}

func (sdc *Controller) runWorker(ctx context.Context) {
	for sdc.processNextItem(ctx) {
	}
}

func (sdc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaDatacenter")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaDatacenter")
		sdc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaDatacenter")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), sdc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, sdc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (sdc *Controller) resolveScyllaDatacenterController(obj metav1.Object) *scyllav1alpha1.ScyllaDatacenter {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != scyllaDatacenterControllerGVK.Kind {
		return nil
	}

	sd, err := sdc.scyllaLister.ScyllaDatacenters(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sd.UID != controllerRef.UID {
		return nil
	}

	return sd
}

func (sdc *Controller) resolveStatefulSetController(obj metav1.Object) *appsv1.StatefulSet {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != statefulSetControllerGVK.Kind {
		return nil
	}

	sts, err := sdc.statefulSetLister.StatefulSets(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sts.UID != controllerRef.UID {
		return nil
	}

	return sts
}

func (sdc *Controller) resolveScyllaDatacenterControllerThroughStatefulSet(obj metav1.Object) *scyllav1alpha1.ScyllaDatacenter {
	sts := sdc.resolveStatefulSetController(obj)
	if sts == nil {
		return nil
	}
	sd := sdc.resolveScyllaDatacenterController(sts)
	if sd == nil {
		return nil
	}

	return sd
}

func (sdc *Controller) enqueue(sd *scyllav1alpha1.ScyllaDatacenter) {
	key, err := keyFunc(sd)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sd, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaDatacenter", klog.KObj(sd))
	sdc.queue.Add(key)
}

func (sdc *Controller) enqueueOwner(obj metav1.Object) {
	sd := sdc.resolveScyllaDatacenterController(obj)
	if sd == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaDatacenter", klog.KObj(sd))
	sdc.enqueue(sd)
}

func (sdc *Controller) enqueueOwnerThroughStatefulSet(obj metav1.Object) {
	sd := sdc.resolveScyllaDatacenterControllerThroughStatefulSet(obj)
	if sd == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS(fmt.Sprintf("%s added", gvk.Kind), gvk.Kind, klog.KObj(obj))
	sdc.enqueue(sd)
}

func (sdc *Controller) enqueueScyllaDatacenterFromPod(pod *corev1.Pod) {
	sd := sdc.resolveScyllaDatacenterControllerThroughStatefulSet(pod)
	if sd == nil {
		return
	}

	klog.V(4).InfoS("Pod added", "ScyllaDatacenter", klog.KObj(sd))
	sdc.enqueue(sd)
}

func (sdc *Controller) addService(obj interface{}) {
	svc := obj.(*corev1.Service)
	klog.V(4).InfoS("Observed addition of Service", "Service", klog.KObj(svc))
	sdc.enqueueOwner(svc)
}

func (sdc *Controller) updateService(old, cur interface{}) {
	oldService := old.(*corev1.Service)
	currentService := cur.(*corev1.Service)

	if currentService.UID != oldService.UID {
		key, err := keyFunc(oldService)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldService, err))
			return
		}
		sdc.deleteService(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldService,
		})
	}

	klog.V(4).InfoS("Observed update of Service", "Service", klog.KObj(oldService))
	sdc.enqueueOwner(currentService)
}

func (sdc *Controller) deleteService(obj interface{}) {
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
	sdc.enqueueOwner(svc)
}

func (sdc *Controller) addSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)
	klog.V(4).InfoS("Observed addition of Secret", "Secret", klog.KObj(secret))
	sdc.enqueueOwner(secret)
}

func (sdc *Controller) updateSecret(old, cur interface{}) {
	oldSecret := old.(*corev1.Secret)
	currentSecret := cur.(*corev1.Secret)

	if currentSecret.UID != oldSecret.UID {
		key, err := keyFunc(oldSecret)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSecret, err))
			return
		}
		sdc.deleteSecret(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSecret,
		})
	}

	klog.V(4).InfoS("Observed update of Secret", "Secret", klog.KObj(oldSecret))
	sdc.enqueueOwner(currentSecret)
}

func (sdc *Controller) deleteSecret(obj interface{}) {
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
	sdc.enqueueOwner(secret)
}

func (sdc *Controller) addConfigMap(obj interface{}) {
	configMap := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(configMap))
	sdc.enqueueOwner(configMap)
}

func (sdc *Controller) updateConfigMap(old, cur interface{}) {
	oldConfigMap := old.(*corev1.ConfigMap)
	currentConfigMap := cur.(*corev1.ConfigMap)

	if currentConfigMap.UID != oldConfigMap.UID {
		key, err := keyFunc(oldConfigMap)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldConfigMap, err))
			return
		}
		sdc.deleteConfigMap(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldConfigMap,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldConfigMap))
	sdc.enqueueOwner(currentConfigMap)
}

func (sdc *Controller) deleteConfigMap(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		configMap, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ConfigMap %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ConfigMap", "ConfigMap", klog.KObj(configMap))
	sdc.enqueueOwner(configMap)
}

func (sdc *Controller) addServiceAccount(obj interface{}) {
	sa := obj.(*corev1.ServiceAccount)
	klog.V(4).InfoS("Observed addition of ServiceAccount", "ServiceAccount", klog.KObj(sa))
	sdc.enqueueOwner(sa)
}

func (sdc *Controller) updateServiceAccount(old, cur interface{}) {
	oldSA := old.(*corev1.ServiceAccount)
	currentSA := cur.(*corev1.ServiceAccount)

	if currentSA.UID != oldSA.UID {
		key, err := keyFunc(oldSA)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSA, err))
			return
		}
		sdc.deleteServiceAccount(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSA,
		})
	}

	klog.V(4).InfoS("Observed update of ServiceAccount", "ServiceAccount", klog.KObj(oldSA))
	sdc.enqueueOwner(currentSA)
}

func (sdc *Controller) deleteServiceAccount(obj interface{}) {
	svc, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		svc, ok = tombstone.Obj.(*corev1.ServiceAccount)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ServiceAccount %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ServiceAccount", "ServiceAccount", klog.KObj(svc))
	sdc.enqueueOwner(svc)
}

func (sdc *Controller) addRoleBinding(obj interface{}) {
	roleBinding := obj.(*rbacv1.RoleBinding)
	klog.V(4).InfoS("Observed addition of RoleBinding", "RoleBinding", klog.KObj(roleBinding))
	sdc.enqueueOwner(roleBinding)
}

func (sdc *Controller) updateRoleBinding(old, cur interface{}) {
	oldRoleBinding := old.(*rbacv1.RoleBinding)
	currentRoleBinding := cur.(*rbacv1.RoleBinding)

	if currentRoleBinding.UID != oldRoleBinding.UID {
		key, err := keyFunc(oldRoleBinding)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldRoleBinding, err))
			return
		}
		sdc.deleteRoleBinding(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldRoleBinding,
		})
	}

	klog.V(4).InfoS("Observed update of RoleBinding", "RoleBinding", klog.KObj(oldRoleBinding))
	sdc.enqueueOwner(currentRoleBinding)
}

func (sdc *Controller) deleteRoleBinding(obj interface{}) {
	roleBinding, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		roleBinding, ok = tombstone.Obj.(*rbacv1.RoleBinding)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a RoleBinding %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of RoleBinding", "RoleBinding", klog.KObj(roleBinding))
	sdc.enqueueOwner(roleBinding)
}

func (sdc *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	sdc.enqueueScyllaDatacenterFromPod(pod)
}

func (sdc *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	if currentPod.UID != oldPod.UID {
		key, err := keyFunc(oldPod)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPod, err))
			return
		}
		sdc.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPod,
		})
	}

	klog.V(4).InfoS("Observed update of Pod", "Pod", klog.KObj(oldPod))
	sdc.enqueueScyllaDatacenterFromPod(currentPod)
}

func (sdc *Controller) deletePod(obj interface{}) {
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
	klog.V(4).InfoS("Observed deletion of Pod", "Pod", klog.KObj(pod), "RV", pod.ResourceVersion)
	sdc.enqueueScyllaDatacenterFromPod(pod)
}

func (sdc *Controller) addStatefulSet(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	klog.V(4).InfoS("Observed addition of StatefulSet", "StatefulSet", klog.KObj(sts), "RV", sts.ResourceVersion)
	sdc.enqueueOwner(sts)
}

func (sdc *Controller) updateStatefulSet(old, cur interface{}) {
	oldSts := old.(*appsv1.StatefulSet)
	currentSts := cur.(*appsv1.StatefulSet)

	if currentSts.UID != oldSts.UID {
		key, err := keyFunc(oldSts)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSts, err))
			return
		}
		sdc.deleteStatefulSet(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSts,
		})
	}

	klog.V(4).InfoS("Observed update of StatefulSet", "StatefulSet", klog.KObj(oldSts), "NewRV", currentSts.ResourceVersion)
	sdc.enqueueOwner(currentSts)
}

func (sdc *Controller) deleteStatefulSet(obj interface{}) {
	sts, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sts, ok = tombstone.Obj.(*appsv1.StatefulSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a StatefulSet %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of StatefulSet", "StatefulSet", klog.KObj(sts), "RV", sts.ResourceVersion)
	sdc.enqueueOwner(sts)
}

func (sdc *Controller) addPodDisruptionBudget(obj interface{}) {
	pdb := obj.(*policyv1.PodDisruptionBudget)
	klog.V(4).InfoS("Observed addition of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(pdb))
	sdc.enqueueOwner(pdb)
}

func (sdc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	oldPDB := old.(*policyv1.PodDisruptionBudget)
	currentPDB := cur.(*policyv1.PodDisruptionBudget)

	if currentPDB.UID != oldPDB.UID {
		key, err := keyFunc(oldPDB)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPDB, err))
			return
		}
		sdc.deletePodDisruptionBudget(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPDB,
		})
	}

	klog.V(4).InfoS("Observed update of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(oldPDB))
	sdc.enqueueOwner(currentPDB)
}

func (sdc *Controller) deletePodDisruptionBudget(obj interface{}) {
	pdb, ok := obj.(*policyv1.PodDisruptionBudget)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		pdb, ok = tombstone.Obj.(*policyv1.PodDisruptionBudget)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a PodDisruptionBudget %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(pdb))
	sdc.enqueueOwner(pdb)
}

func (scc *Controller) addIngress(obj interface{}) {
	ingress := obj.(*networkingv1.Ingress)
	klog.V(4).InfoS("Observed addition of Ingress", "Ingress", klog.KObj(ingress))
	scc.enqueueOwner(ingress)
}

func (scc *Controller) updateIngress(old, cur interface{}) {
	oldIngress := old.(*networkingv1.Ingress)
	currentIngress := cur.(*networkingv1.Ingress)

	if currentIngress.UID != oldIngress.UID {
		key, err := keyFunc(oldIngress)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldIngress, err))
			return
		}
		scc.deleteIngress(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldIngress,
		})
	}

	klog.V(4).InfoS("Observed update of Ingress", "Ingress", klog.KObj(oldIngress))
	scc.enqueueOwner(currentIngress)
}

func (scc *Controller) deleteIngress(obj interface{}) {
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		ingress, ok = tombstone.Obj.(*networkingv1.Ingress)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Ingress %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Ingress", "Ingress", klog.KObj(ingress))
	scc.enqueueOwner(ingress)
}

func (sdc *Controller) addScyllaDatacenter(obj interface{}) {
	sd := obj.(*scyllav1alpha1.ScyllaDatacenter)
	klog.V(4).InfoS("Observed addition of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	sdc.enqueue(sd)
}

func (sdc *Controller) updateScyllaDatacenter(old, cur interface{}) {
	oldSD := old.(*scyllav1alpha1.ScyllaDatacenter)
	currentSD := cur.(*scyllav1alpha1.ScyllaDatacenter)

	if currentSD.UID != oldSD.UID {
		key, err := keyFunc(oldSD)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSD, err))
			return
		}
		sdc.deleteScyllaDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSD,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(oldSD))
	sdc.enqueue(currentSD)
}

func (sdc *Controller) deleteScyllaDatacenter(obj interface{}) {
	sd, ok := obj.(*scyllav1alpha1.ScyllaDatacenter)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sd, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDatacenter)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaDatacenter %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	sdc.enqueue(sd)
}
