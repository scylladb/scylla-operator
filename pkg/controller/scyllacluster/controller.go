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
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
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

	artificialDelayForCachesToCatchUp = 10 * time.Second
)

var (
	keyFunc                    = cache.DeletionHandlingMetaNamespaceKeyFunc
	scyllaClusterControllerGVK = scyllav1.GroupVersion.WithKind("ScyllaCluster")
	statefulSetControllerGVK   = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

type Controller struct {
	operatorImage   string
	cqlsIngressPort int

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

	queue    workqueue.RateLimitingInterface
	handlers *controllerhelpers.Handlers[*scyllav1.ScyllaCluster]
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

	var err error
	scc.handlers, err = controllerhelpers.NewHandlers[*scyllav1.ScyllaCluster](
		scc.queue,
		keyFunc,
		scheme.Scheme,
		scyllaClusterControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1.ScyllaCluster]{
			GetFunc: func(namespace, name string) (*scyllav1.ScyllaCluster, error) {
				return scc.scyllaLister.ScyllaClusters(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1.ScyllaCluster, err error) {
				return scc.scyllaLister.ScyllaClusters(namespace).List(selector)
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

func (scc *Controller) enqueueOwnerThroughStatefulSetOwner(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	sts := scc.resolveStatefulSetController(obj)
	if sts == nil {
		return
	}

	sc := scc.resolveScyllaClusterController(sts)
	if sc == nil {
		return
	}

	klog.V(4).InfoS("Enqueuing owner of StatefulSet", "StatefulSet", klog.KObj(sc), "ScyllaCluster", klog.KObj(sc))
	scc.handlers.Enqueue(depth+1, sc, op)
}

func (scc *Controller) addService(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Service),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateService(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		scc.handlers.EnqueueOwner,
		scc.deleteService,
	)
}

func (scc *Controller) deleteService(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addSecret(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateSecret(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		scc.handlers.EnqueueOwner,
		scc.deleteSecret,
	)
}

func (scc *Controller) deleteSecret(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addConfigMap(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateConfigMap(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		scc.handlers.EnqueueOwner,
		scc.deleteConfigMap,
	)
}

func (scc *Controller) deleteConfigMap(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addServiceAccount(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.ServiceAccount),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateServiceAccount(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.ServiceAccount),
		cur.(*corev1.ServiceAccount),
		scc.handlers.EnqueueOwner,
		scc.deleteServiceAccount,
	)
}

func (scc *Controller) deleteServiceAccount(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addRoleBinding(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*rbacv1.RoleBinding),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateRoleBinding(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*rbacv1.RoleBinding),
		cur.(*rbacv1.RoleBinding),
		scc.handlers.EnqueueOwner,
		scc.deleteRoleBinding,
	)
}

func (scc *Controller) deleteRoleBinding(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addPod(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*corev1.Pod),
		scc.enqueueOwnerThroughStatefulSetOwner,
	)
}

func (scc *Controller) updatePod(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*corev1.Pod),
		cur.(*corev1.Pod),
		scc.enqueueOwnerThroughStatefulSetOwner,
		scc.deletePod,
	)
}

func (scc *Controller) deletePod(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.enqueueOwnerThroughStatefulSetOwner,
	)
}

func (scc *Controller) addStatefulSet(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*appsv1.StatefulSet),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateStatefulSet(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*appsv1.StatefulSet),
		cur.(*appsv1.StatefulSet),
		scc.handlers.EnqueueOwner,
		scc.deleteStatefulSet,
	)
}

func (scc *Controller) deleteStatefulSet(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addPodDisruptionBudget(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*policyv1.PodDisruptionBudget),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*policyv1.PodDisruptionBudget),
		cur.(*policyv1.PodDisruptionBudget),
		scc.handlers.EnqueueOwner,
		scc.deletePodDisruptionBudget,
	)
}

func (scc *Controller) deletePodDisruptionBudget(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addIngress(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*networkingv1.Ingress),
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) updateIngress(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*networkingv1.Ingress),
		cur.(*networkingv1.Ingress),
		scc.handlers.EnqueueOwner,
		scc.deleteIngress,
	)
}

func (scc *Controller) deleteIngress(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.EnqueueOwner,
	)
}

func (scc *Controller) addScyllaCluster(obj interface{}) {
	scc.handlers.HandleAdd(
		obj.(*scyllav1.ScyllaCluster),
		scc.handlers.Enqueue,
	)
}

func (scc *Controller) updateScyllaCluster(old, cur interface{}) {
	scc.handlers.HandleUpdate(
		old.(*scyllav1.ScyllaCluster),
		cur.(*scyllav1.ScyllaCluster),
		scc.handlers.Enqueue,
		scc.deleteScyllaCluster,
	)
}

func (scc *Controller) deleteScyllaCluster(obj interface{}) {
	scc.handlers.HandleDelete(
		obj,
		scc.handlers.Enqueue,
	)
}
