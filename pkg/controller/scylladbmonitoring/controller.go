package scylladbmonitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1informers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions/monitoring/v1"
	monitoringv1listers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
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
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaDBMonitoringController"
)

var (
	keyFunc                         = cache.DeletionHandlingMetaNamespaceKeyFunc
	scylladbMonitoringControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBMonitoring")
)

type Controller struct {
	kubeClient           kubernetes.Interface
	scyllaV1alpha1Client scyllav1alpha1client.ScyllaV1alpha1Interface
	monitoringClient     monitoringv1client.MonitoringV1Interface

	scyllaOperatorConfigLister scyllav1alpha1listers.ScyllaOperatorConfigLister
	configMapLister            corev1listers.ConfigMapLister
	secretLister               corev1listers.SecretLister
	serviceLister              corev1listers.ServiceLister
	serviceAccountLister       corev1listers.ServiceAccountLister
	roleBindingLister          rbacv1listers.RoleBindingLister
	pdbLister                  policyv1listers.PodDisruptionBudgetLister
	deploymentLister           appsv1listers.DeploymentLister
	ingressLister              networkingv1listers.IngressLister

	scyllaDBMonitoringInformer scyllav1alpha1informers.ScyllaDBMonitoringInformer

	prometheusLister     monitoringv1listers.PrometheusLister
	prometheusRuleLister monitoringv1listers.PrometheusRuleLister
	serviceMonitorLister monitoringv1listers.ServiceMonitorLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.TypedRateLimitingInterface[string]
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBMonitoring]

	keyGetter crypto.RSAKeyGetter
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaV1alpha1Client scyllav1alpha1client.ScyllaV1alpha1Interface,
	monitoringClient monitoringv1client.MonitoringV1Interface,
	scyllaOperatorConfigInformer scyllav1alpha1informers.ScyllaOperatorConfigInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	secretInformer corev1informers.SecretInformer,
	serviceInformer corev1informers.ServiceInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	ingressInformer networkingv1informers.IngressInformer,
	scyllaDBMonitoringInformer scyllav1alpha1informers.ScyllaDBMonitoringInformer,
	prometheusInformer monitoringv1informers.PrometheusInformer,
	prometheusRuleInformer monitoringv1informers.PrometheusRuleInformer,
	serviceMonitorInformer monitoringv1informers.ServiceMonitorInformer,
	keyGetter crypto.RSAKeyGetter,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	smc := &Controller{
		kubeClient:           kubeClient,
		scyllaV1alpha1Client: scyllaV1alpha1Client,
		monitoringClient:     monitoringClient,

		scyllaOperatorConfigLister: scyllaOperatorConfigInformer.Lister(),
		secretLister:               secretInformer.Lister(),
		configMapLister:            configMapInformer.Lister(),
		serviceLister:              serviceInformer.Lister(),
		serviceAccountLister:       serviceAccountInformer.Lister(),
		roleBindingLister:          roleBindingInformer.Lister(),
		pdbLister:                  pdbInformer.Lister(),
		deploymentLister:           deploymentInformer.Lister(),
		ingressLister:              ingressInformer.Lister(),

		scyllaDBMonitoringInformer: scyllaDBMonitoringInformer,

		prometheusLister:     prometheusInformer.Lister(),
		prometheusRuleLister: prometheusRuleInformer.Lister(),
		serviceMonitorLister: serviceMonitorInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaOperatorConfigInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			serviceInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
			roleBindingInformer.Informer().HasSynced,
			pdbInformer.Informer().HasSynced,
			deploymentInformer.Informer().HasSynced,
			ingressInformer.Informer().HasSynced,

			scyllaDBMonitoringInformer.Informer().HasSynced,

			prometheusInformer.Informer().HasSynced,
			prometheusRuleInformer.Informer().HasSynced,
			serviceMonitorInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbmonitoring-controller"}),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "scylladbmonitoring",
			},
		),

		keyGetter: keyGetter,
	}

	if err := scyllaDBMonitoringInformer.Informer().AddIndexers(cache.Indexers{
		scyllaDBMonitoringBySecretIndexName:    indexScyllaDBMonitoringBySecret,
		scyllaDBMonitoringByConfigMapIndexName: indexScyllaDBMonitoringByConfigMap,
	}); err != nil {
		return nil, fmt.Errorf("can't add indexers to ScyllaDBMonitoring informer: %w", err)
	}

	var err error
	smc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBMonitoring](
		smc.queue,
		keyFunc,
		scheme.Scheme,
		scylladbMonitoringControllerGVK,
		kubeinterfaces.NamespacedGetList[*scyllav1alpha1.ScyllaDBMonitoring]{
			GetFunc: func(namespace, name string) (*scyllav1alpha1.ScyllaDBMonitoring, error) {
				return smc.scyllaDBMonitoringInformer.Lister().ScyllaDBMonitorings(namespace).Get(name)
			},
			ListFunc: func(namespace string, selector labels.Selector) (ret []*scyllav1alpha1.ScyllaDBMonitoring, err error) {
				return smc.scyllaDBMonitoringInformer.Lister().ScyllaDBMonitorings(namespace).List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	scyllaOperatorConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addScyllaOperatorConfig,
		UpdateFunc: smc.updateScyllaOperatorConfig,
		DeleteFunc: smc.deleteScyllaOperatorConfig,
	})

	scyllaDBMonitoringInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addScyllaDBMonitoring,
		UpdateFunc: smc.updateScyllaDBMonitoring,
		DeleteFunc: smc.deleteScyllaDBMonitoring,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addConfigMap,
		UpdateFunc: smc.updateConfigMap,
		DeleteFunc: smc.deleteConfigMap,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addSecret,
		UpdateFunc: smc.updateSecret,
		DeleteFunc: smc.deleteSecret,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addService,
		UpdateFunc: smc.updateService,
		DeleteFunc: smc.deleteService,
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addServiceAccount,
		UpdateFunc: smc.updateServiceAccount,
		DeleteFunc: smc.deleteServiceAccount,
	})

	pdbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPodDisruptionBudget,
		UpdateFunc: smc.updatePodDisruptionBudget,
		DeleteFunc: smc.deletePodDisruptionBudget,
	})

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addDeployment,
		UpdateFunc: smc.updateDeployment,
		DeleteFunc: smc.deleteDeployment,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addIngress,
		UpdateFunc: smc.updateIngress,
		DeleteFunc: smc.deleteIngress,
	})

	prometheusInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPrometheus,
		UpdateFunc: smc.updatePrometheus,
		DeleteFunc: smc.deletePrometheus,
	})

	prometheusRuleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPrometheusRule,
		UpdateFunc: smc.updatePrometheusRule,
		DeleteFunc: smc.deletePrometheusRule,
	})

	serviceMonitorInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addServiceMonitor,
		UpdateFunc: smc.updateServiceMonitor,
		DeleteFunc: smc.deleteServiceMonitor,
	})

	return smc, nil
}

func (smc *Controller) addScyllaDBMonitoring(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBMonitoring),
		smc.handlers.Enqueue,
	)
}

func (smc *Controller) updateScyllaDBMonitoring(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBMonitoring),
		cur.(*scyllav1alpha1.ScyllaDBMonitoring),
		smc.handlers.Enqueue,
		smc.deleteScyllaDBMonitoring,
	)
}

func (smc *Controller) deleteScyllaDBMonitoring(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.Enqueue,
	)
}

func (smc *Controller) addScyllaOperatorConfig(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaOperatorConfig),
		smc.handlers.EnqueueAll,
	)
}

func (smc *Controller) updateScyllaOperatorConfig(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaOperatorConfig),
		cur.(*scyllav1alpha1.ScyllaOperatorConfig),
		smc.handlers.EnqueueAll,
		smc.deleteScyllaOperatorConfig,
	)
}

func (smc *Controller) deleteScyllaOperatorConfig(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueAll,
	)
}
func (smc *Controller) addConfigMap(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*corev1.ConfigMap),
		combineEnqueueFuncs(
			smc.enqueueByConfigMapRef,
			smc.handlers.EnqueueOwner,
		),
	)
}

func (smc *Controller) updateConfigMap(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*corev1.ConfigMap),
		cur.(*corev1.ConfigMap),
		combineEnqueueFuncs(
			smc.enqueueByConfigMapRef,
			smc.handlers.EnqueueOwner,
		),
		smc.deleteConfigMap,
	)
}

func (smc *Controller) deleteConfigMap(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		combineEnqueueFuncs(
			smc.enqueueByConfigMapRef,
			smc.handlers.EnqueueOwner,
		),
	)
}

func (smc *Controller) addSecret(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*corev1.Secret),
		combineEnqueueFuncs(
			smc.enqueueBySecretRef,
			smc.handlers.EnqueueOwner,
		),
	)
}

func (smc *Controller) updateSecret(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*corev1.Secret),
		cur.(*corev1.Secret),
		combineEnqueueFuncs(
			smc.enqueueBySecretRef,
			smc.handlers.EnqueueOwner,
		),
		smc.deleteSecret,
	)
}

func (smc *Controller) deleteSecret(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		combineEnqueueFuncs(
			smc.enqueueBySecretRef,
			smc.handlers.EnqueueOwner,
		),
	)
}

func (smc *Controller) addService(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*corev1.Service),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateService(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		smc.handlers.EnqueueOwner,
		smc.deleteService,
	)
}

func (smc *Controller) deleteService(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addServiceAccount(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*corev1.ServiceAccount),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateServiceAccount(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*corev1.ServiceAccount),
		cur.(*corev1.ServiceAccount),
		smc.handlers.EnqueueOwner,
		smc.deleteServiceAccount,
	)
}

func (smc *Controller) deleteServiceAccount(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addPodDisruptionBudget(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*policyv1.PodDisruptionBudget),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*policyv1.PodDisruptionBudget),
		cur.(*policyv1.PodDisruptionBudget),
		smc.handlers.EnqueueOwner,
		smc.deletePodDisruptionBudget,
	)
}

func (smc *Controller) deletePodDisruptionBudget(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addDeployment(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*appsv1.Deployment),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateDeployment(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*appsv1.Deployment),
		cur.(*appsv1.Deployment),
		smc.handlers.EnqueueOwner,
		smc.deleteDeployment,
	)
}

func (smc *Controller) deleteDeployment(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addIngress(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*networkingv1.Ingress),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateIngress(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*networkingv1.Ingress),
		cur.(*networkingv1.Ingress),
		smc.handlers.EnqueueOwner,
		smc.deleteIngress,
	)
}

func (smc *Controller) deleteIngress(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addPrometheus(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*monitoringv1.Prometheus),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updatePrometheus(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*monitoringv1.Prometheus),
		cur.(*monitoringv1.Prometheus),
		smc.handlers.EnqueueOwner,
		smc.deletePrometheus,
	)
}

func (smc *Controller) deletePrometheus(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addPrometheusRule(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*monitoringv1.PrometheusRule),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updatePrometheusRule(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*monitoringv1.PrometheusRule),
		cur.(*monitoringv1.PrometheusRule),
		smc.handlers.EnqueueOwner,
		smc.deletePrometheusRule,
	)
}

func (smc *Controller) deletePrometheusRule(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addServiceMonitor(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*monitoringv1.ServiceMonitor),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateServiceMonitor(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*monitoringv1.ServiceMonitor),
		cur.(*monitoringv1.ServiceMonitor),
		smc.handlers.EnqueueOwner,
		smc.deleteServiceMonitor,
	)
}

func (smc *Controller) deleteServiceMonitor(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) enqueueBySecretRef(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	_, ok := obj.(*corev1.Secret)
	if !ok {
		apimachineryutilruntime.HandleError(fmt.Errorf("expected %T, got %T", &corev1.Secret{}, obj))
		return
	}

	name := obj.GetName()
	indexedSDBMs, err := smc.scyllaDBMonitoringInformer.Informer().GetIndexer().ByIndex(scyllaDBMonitoringBySecretIndexName, name)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("can't get ScyllaDBMonitoring for Secret %q: %w", name, err))
	}

	for _, indexedSDBM := range indexedSDBMs {
		sdbm, ok := indexedSDBM.(*scyllav1alpha1.ScyllaDBMonitoring)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("expected *scyllav1alpha1.ScyllaDBMonitoring, got %T", indexedSDBM))
			continue
		}
		klog.V(4).InfoS("Enqueuing ScyllaDBMonitoring for Secret", "Secret", name, "ScyllaDBMonitoring", klog.KObj(sdbm))
		smc.handlers.Enqueue(depth+1, sdbm, op)
	}
}

func (smc *Controller) enqueueByConfigMapRef(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
	_, ok := obj.(*corev1.ConfigMap)
	if !ok {
		apimachineryutilruntime.HandleError(fmt.Errorf("expected %T, got %T", &corev1.ConfigMap{}, obj))
		return
	}

	name := obj.GetName()
	indexedSDBMs, err := smc.scyllaDBMonitoringInformer.Informer().GetIndexer().ByIndex(scyllaDBMonitoringByConfigMapIndexName, name)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("can't get ScyllaDBMonitoring for ConfigMap %q: %w", name, err))
	}

	for _, indexedSDBM := range indexedSDBMs {
		sdbm, ok := indexedSDBM.(*scyllav1alpha1.ScyllaDBMonitoring)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("expected %T, got %T", &scyllav1alpha1.ScyllaDBMonitoring{}, indexedSDBM))
			continue
		}
		klog.V(4).InfoS("Enqueuing ScyllaDBMonitoring for ConfigMap", "ConfigMap", name, "ScyllaDBMonitoring", klog.KObj(sdbm))
		smc.handlers.Enqueue(depth+1, sdbm, op)
	}
}

func (smc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := smc.queue.Get()
	if quit {
		return false
	}
	defer smc.queue.Done(key)

	err := smc.sync(ctx, key)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		smc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	smc.queue.AddRateLimited(key)

	return true
}

func (smc *Controller) runWorker(ctx context.Context) {
	for smc.processNextItem(ctx) {
	}
}

func (smc *Controller) Run(ctx context.Context, workers int) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaDBMonitoring")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaDBMonitoring")
		smc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaDBMonitoring")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), smc.cachesToSync...) {
		return
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apimachineryutilwait.UntilWithContext(ctx, smc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func combineEnqueueFuncs(funcs ...controllerhelpers.EnqueueFuncType) controllerhelpers.EnqueueFuncType {
	return func(depth int, obj kubeinterfaces.ObjectInterface, op controllerhelpers.HandlerOperationType) {
		for _, fn := range funcs {
			fn(depth+1, obj, op)
		}
	}
}
