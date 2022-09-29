package scyllacluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
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
	ControllerName = "ScyllaClusterController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 40 * time.Second

	artificialDelayForCachesToCatchUp = 10 * time.Second
)

var (
	keyFunc                    = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaClusterControllerGVK = scyllav1.GroupVersion.WithKind("ScyllaCluster")
	statefulSetControllerGVK   = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

type Controller struct {
	operatorImage string

	kubeClient   kubernetes.Interface
	scyllaClient scyllav1client.ScyllaV1Interface

	podLister            corev1listers.PodLister
	serviceLister        corev1listers.ServiceLister
	secretLister         corev1listers.SecretLister
	configMapLister      corev1listers.ConfigMapLister
	serviceAccountLister corev1listers.ServiceAccountLister
	roleBindingLister    rbacv1listers.RoleBindingLister
	statefulSetLister    appsv1listers.StatefulSetLister
	pdbLister            policyv1listers.PodDisruptionBudgetLister
	ingressLister        networkingv1listers.IngressLister
	scyllaLister         scyllav1listers.ScyllaClusterLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1client.ScyllaV1Interface,
	podInformer corev1informers.PodInformer,
	serviceInformer corev1informers.ServiceInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	statefulSetInformer appsv1informers.StatefulSetInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	ingressInformer networkingv1informers.IngressInformer,
	scyllaClusterInformer scyllav1informers.ScyllaClusterInformer,
	operatorImage string,
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
		operatorImage: operatorImage,

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
		scyllaLister:         scyllaClusterInformer.Lister(),

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
			scyllaClusterInformer.Informer().HasSynced,
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

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scc.addScyllaCluster,
		UpdateFunc: scc.updateScyllaCluster,
		DeleteFunc: scc.deleteScyllaCluster,
	})

	return scc, nil
}

func (scc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := scc.queue.Get()
	if quit {
		return false
	}
	defer scc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := scc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		scc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	scc.queue.AddRateLimited(key)

	return true
}

func (scc *Controller) runWorker(ctx context.Context) {
	for scc.processNextItem(ctx) {
	}
}

func (scc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaCluster")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaCluster")
		scc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaCluster")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), scc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, scc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (scc *Controller) resolveScyllaClusterController(obj metav1.Object) *scyllav1.ScyllaCluster {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != scyllaClusterControllerGVK.Kind {
		return nil
	}

	sc, err := scc.scyllaLister.ScyllaClusters(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sc.UID != controllerRef.UID {
		return nil
	}

	return sc
}

func (scc *Controller) resolveStatefulSetController(obj metav1.Object) *appsv1.StatefulSet {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != statefulSetControllerGVK.Kind {
		return nil
	}

	sts, err := scc.statefulSetLister.StatefulSets(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sts.UID != controllerRef.UID {
		return nil
	}

	return sts
}

func (scc *Controller) resolveScyllaClusterControllerThroughStatefulSet(obj metav1.Object) *scyllav1.ScyllaCluster {
	sts := scc.resolveStatefulSetController(obj)
	if sts == nil {
		return nil
	}
	sc := scc.resolveScyllaClusterController(sts)
	if sc == nil {
		return nil
	}

	return sc
}

func (scc *Controller) enqueue(sc *scyllav1.ScyllaCluster) {
	key, err := keyFunc(sc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaCluster", klog.KObj(sc))
	scc.queue.Add(key)
}

func (scc *Controller) enqueueOwner(obj metav1.Object) {
	sc := scc.resolveScyllaClusterController(obj)
	if sc == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaCluster", klog.KObj(sc))
	scc.enqueue(sc)
}

func (scc *Controller) enqueueOwnerThroughStatefulSet(obj metav1.Object) {
	sc := scc.resolveScyllaClusterControllerThroughStatefulSet(obj)
	if sc == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS(fmt.Sprintf("%s added", gvk.Kind), gvk.Kind, klog.KObj(obj))
	scc.enqueue(sc)
}

func (scc *Controller) enqueueScyllaClusterFromPod(pod *corev1.Pod) {
	sc := scc.resolveScyllaClusterControllerThroughStatefulSet(pod)
	if sc == nil {
		return
	}

	klog.V(4).InfoS("Pod added", "ScyllaCluster", klog.KObj(sc))
	scc.enqueue(sc)
}

func (scc *Controller) addService(obj interface{}) {
	svc := obj.(*corev1.Service)
	klog.V(4).InfoS("Observed addition of Service", "Service", klog.KObj(svc))
	scc.enqueueOwner(svc)
}

func (scc *Controller) updateService(old, cur interface{}) {
	oldService := old.(*corev1.Service)
	currentService := cur.(*corev1.Service)

	if currentService.UID != oldService.UID {
		key, err := keyFunc(oldService)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldService, err))
			return
		}
		scc.deleteService(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldService,
		})
	}

	klog.V(4).InfoS("Observed update of Service", "Service", klog.KObj(oldService))
	scc.enqueueOwner(currentService)
}

func (scc *Controller) deleteService(obj interface{}) {
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
	scc.enqueueOwner(svc)
}

func (scc *Controller) addSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)
	klog.V(4).InfoS("Observed addition of Secret", "Secret", klog.KObj(secret))
	scc.enqueueOwner(secret)
}

func (scc *Controller) updateSecret(old, cur interface{}) {
	oldSecret := old.(*corev1.Secret)
	currentSecret := cur.(*corev1.Secret)

	if currentSecret.UID != oldSecret.UID {
		key, err := keyFunc(oldSecret)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSecret, err))
			return
		}
		scc.deleteSecret(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSecret,
		})
	}

	klog.V(4).InfoS("Observed update of Secret", "Secret", klog.KObj(oldSecret))
	scc.enqueueOwner(currentSecret)
}

func (scc *Controller) deleteSecret(obj interface{}) {
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
	scc.enqueueOwner(secret)
}

func (scc *Controller) addConfigMap(obj interface{}) {
	configMap := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(configMap))
	scc.enqueueOwner(configMap)
}

func (scc *Controller) updateConfigMap(old, cur interface{}) {
	oldConfigMap := old.(*corev1.ConfigMap)
	currentConfigMap := cur.(*corev1.ConfigMap)

	if currentConfigMap.UID != oldConfigMap.UID {
		key, err := keyFunc(oldConfigMap)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldConfigMap, err))
			return
		}
		scc.deleteConfigMap(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldConfigMap,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldConfigMap))
	scc.enqueueOwner(currentConfigMap)
}

func (scc *Controller) deleteConfigMap(obj interface{}) {
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
	scc.enqueueOwner(configMap)
}

func (sac *Controller) addServiceAccount(obj interface{}) {
	sa := obj.(*corev1.ServiceAccount)
	klog.V(4).InfoS("Observed addition of ServiceAccount", "ServiceAccount", klog.KObj(sa))
	sac.enqueueOwner(sa)
}

func (sac *Controller) updateServiceAccount(old, cur interface{}) {
	oldSA := old.(*corev1.ServiceAccount)
	currentSA := cur.(*corev1.ServiceAccount)

	if currentSA.UID != oldSA.UID {
		key, err := keyFunc(oldSA)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSA, err))
			return
		}
		sac.deleteServiceAccount(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSA,
		})
	}

	klog.V(4).InfoS("Observed update of ServiceAccount", "ServiceAccount", klog.KObj(oldSA))
	sac.enqueueOwner(currentSA)
}

func (sac *Controller) deleteServiceAccount(obj interface{}) {
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
	sac.enqueueOwner(svc)
}

func (sac *Controller) addRoleBinding(obj interface{}) {
	roleBinding := obj.(*rbacv1.RoleBinding)
	klog.V(4).InfoS("Observed addition of RoleBinding", "RoleBinding", klog.KObj(roleBinding))
	sac.enqueueOwner(roleBinding)
}

func (sac *Controller) updateRoleBinding(old, cur interface{}) {
	oldRoleBinding := old.(*rbacv1.RoleBinding)
	currentRoleBinding := cur.(*rbacv1.RoleBinding)

	if currentRoleBinding.UID != oldRoleBinding.UID {
		key, err := keyFunc(oldRoleBinding)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldRoleBinding, err))
			return
		}
		sac.deleteRoleBinding(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldRoleBinding,
		})
	}

	klog.V(4).InfoS("Observed update of RoleBinding", "RoleBinding", klog.KObj(oldRoleBinding))
	sac.enqueueOwner(currentRoleBinding)
}

func (sac *Controller) deleteRoleBinding(obj interface{}) {
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
	sac.enqueueOwner(roleBinding)
}

func (scc *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	scc.enqueueScyllaClusterFromPod(pod)
}

func (scc *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	if currentPod.UID != oldPod.UID {
		key, err := keyFunc(oldPod)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPod, err))
			return
		}
		scc.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPod,
		})
	}

	klog.V(4).InfoS("Observed update of Pod", "Pod", klog.KObj(oldPod))
	scc.enqueueScyllaClusterFromPod(currentPod)
}

func (scc *Controller) deletePod(obj interface{}) {
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
	scc.enqueueScyllaClusterFromPod(pod)
}

func (scc *Controller) addStatefulSet(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	klog.V(4).InfoS("Observed addition of StatefulSet", "StatefulSet", klog.KObj(sts), "RV", sts.ResourceVersion)
	scc.enqueueOwner(sts)
}

func (scc *Controller) updateStatefulSet(old, cur interface{}) {
	oldSts := old.(*appsv1.StatefulSet)
	currentSts := cur.(*appsv1.StatefulSet)

	if currentSts.UID != oldSts.UID {
		key, err := keyFunc(oldSts)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSts, err))
			return
		}
		scc.deleteStatefulSet(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSts,
		})
	}

	klog.V(4).InfoS("Observed update of StatefulSet", "StatefulSet", klog.KObj(oldSts), "NewRV", currentSts.ResourceVersion)
	scc.enqueueOwner(currentSts)
}

func (scc *Controller) deleteStatefulSet(obj interface{}) {
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
	scc.enqueueOwner(sts)
}

func (scc *Controller) addPodDisruptionBudget(obj interface{}) {
	pdb := obj.(*policyv1.PodDisruptionBudget)
	klog.V(4).InfoS("Observed addition of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(pdb))
	scc.enqueueOwner(pdb)
}

func (scc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	oldPDB := old.(*policyv1.PodDisruptionBudget)
	currentPDB := cur.(*policyv1.PodDisruptionBudget)

	if currentPDB.UID != oldPDB.UID {
		key, err := keyFunc(oldPDB)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPDB, err))
			return
		}
		scc.deletePodDisruptionBudget(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPDB,
		})
	}

	klog.V(4).InfoS("Observed update of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(oldPDB))
	scc.enqueueOwner(currentPDB)
}

func (scc *Controller) deletePodDisruptionBudget(obj interface{}) {
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
	scc.enqueueOwner(pdb)
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

func (scc *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	scc.enqueue(sc)
}

func (scc *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav1.ScyllaCluster)
	currentSC := cur.(*scyllav1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		scc.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	scc.enqueue(currentSC)
}

func (scc *Controller) deleteScyllaCluster(obj interface{}) {
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
	scc.enqueue(sc)
}
