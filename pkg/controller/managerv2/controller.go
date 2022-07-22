package managerv2

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaManagerController"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc       = cache.DeletionHandlingMetaNamespaceKeyFunc
	controllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaManager")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	serviceLister       corev1listers.ServiceLister
	secretLister        corev1listers.SecretLister
	configMapLister     corev1listers.ConfigMapLister
	deploymentLister    appsv1listers.DeploymentLister
	pdbLister           policyv1listers.PodDisruptionBudgetLister
	scyllaManagerLister scyllav1alpha1listers.ScyllaManagerLister
	scyllaClusterLister scyllav1listers.ScyllaClusterLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	serviceInformer corev1informers.ServiceInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	scyllaManagerInformer scyllav1alpha1informers.ScyllaManagerInformer,
	scyllaClusterInformer scyllav1informers.ScyllaClusterInformer,
) (*Controller, error) {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllamanager_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	smc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		serviceLister:       serviceInformer.Lister(),
		secretLister:        secretInformer.Lister(),
		configMapLister:     configMapInformer.Lister(),
		deploymentLister:    deploymentInformer.Lister(),
		pdbLister:           pdbInformer.Lister(),
		scyllaManagerLister: scyllaManagerInformer.Lister(),
		scyllaClusterLister: scyllaClusterInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			serviceInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			deploymentInformer.Informer().HasSynced,
			pdbInformer.Informer().HasSynced,
			scyllaManagerInformer.Informer().HasSynced,
			scyllaClusterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllamanager-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllamanager"),
	}

	scyllaManagerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addScyllaManager,
		UpdateFunc: smc.updateScyllaManager,
		DeleteFunc: smc.deleteScyllaManager,
	})

	scyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addScyllaCluster,
		UpdateFunc: smc.updateScyllaCluster,
		DeleteFunc: smc.deleteScyllaCluster,
	})

	pdbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPodDisruptionBudget,
		UpdateFunc: smc.updatePodDisruptionBudget,
		DeleteFunc: smc.deletePodDisruptionBudget,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addService,
		UpdateFunc: smc.updateService,
		DeleteFunc: smc.deleteService,
	})

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addSecret,
		UpdateFunc: smc.updateSecret,
		DeleteFunc: smc.deleteSecret,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addConfigMap,
		UpdateFunc: smc.updateConfigMap,
		DeleteFunc: smc.deleteConfigMap,
	})

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addDeployment,
		UpdateFunc: smc.updateDeployment,
		DeleteFunc: smc.deleteDeployment,
	})

	return smc, nil
}

func (smc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaManager")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaManager")
		smc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaManager")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), smc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, smc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (smc *Controller) runWorker(ctx context.Context) {
	for smc.processNextItem(ctx) {
	}
}

func (smc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := smc.queue.Get()
	if quit {
		return false
	}
	defer smc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := smc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		smc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	smc.queue.AddRateLimited(key)

	return true
}

func (smc *Controller) enqueue(sm *scyllav1alpha1.ScyllaManager) {
	key, err := keyFunc(sm)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sm, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaManager", klog.KObj(sm))
	smc.queue.Add(key)
}

func (smc *Controller) enqueueNamespace(namespace string) {
	managers, err := smc.scyllaManagerLister.ScyllaManagers(namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't list ScyllaManagers in namespace %v: %v", namespace, err))
		return
	}

	for _, sm := range managers {
		smc.enqueue(sm)
	}
}

func (smc *Controller) enqueueOwner(obj metav1.Object) {
	sm := smc.resolveScyllaManagerController(obj)
	if sm == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaManager", klog.KObj(sm))
	smc.enqueue(sm)
}

func (smc *Controller) resolveScyllaManagerController(obj metav1.Object) *scyllav1alpha1.ScyllaManager {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != controllerGVK.Kind {
		return nil
	}

	sm, err := smc.scyllaManagerLister.ScyllaManagers(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if sm.UID != controllerRef.UID {
		return nil
	}

	return sm
}

func (smc *Controller) addScyllaManager(obj interface{}) {
	sm := obj.(*scyllav1alpha1.ScyllaManager)
	klog.V(4).InfoS("Observed addition of ScyllaManager", "ScyllaManager", klog.KObj(sm))
	smc.enqueue(sm)
}

func (smc *Controller) updateScyllaManager(old, cur interface{}) {
	oldSM := old.(*scyllav1alpha1.ScyllaManager)
	curSM := cur.(*scyllav1alpha1.ScyllaManager)

	if curSM.UID != oldSM.UID {
		key, err := keyFunc(oldSM)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSM, err))
			return
		}
		smc.deleteScyllaManager(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSM,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaManager", "ScyllaManager", klog.KObj(oldSM))
	smc.enqueue(curSM)
}

func (smc *Controller) deleteScyllaManager(obj interface{}) {
	sm, ok := obj.(*scyllav1alpha1.ScyllaManager)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sm, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaManager)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaManager %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaManager", "ScyllaManager", klog.KObj(sm))
	smc.enqueue(sm)
}

func (smc *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	smc.enqueueNamespace(sc.Namespace)
}

func (smc *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav1.ScyllaCluster)
	currentSC := cur.(*scyllav1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		smc.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	smc.enqueueNamespace(currentSC.Namespace)
}

func (smc *Controller) deleteScyllaCluster(obj interface{}) {
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
	smc.enqueueNamespace(sc.Namespace)
}

func (smc *Controller) addPodDisruptionBudget(obj interface{}) {
	pdb := obj.(*policyv1.PodDisruptionBudget)
	klog.V(4).InfoS("Observed addition of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(pdb))
	smc.enqueueOwner(pdb)
}

func (smc *Controller) updatePodDisruptionBudget(old, cur interface{}) {
	oldPDB := old.(*policyv1.PodDisruptionBudget)
	currentPDB := cur.(*policyv1.PodDisruptionBudget)

	if currentPDB.UID != oldPDB.UID {
		key, err := keyFunc(oldPDB)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPDB, err))
			return
		}
		smc.deletePodDisruptionBudget(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPDB,
		})
	}

	klog.V(4).InfoS("Observed update of PodDisruptionBudget", "PodDisruptionBudget", klog.KObj(oldPDB))
	smc.enqueueOwner(currentPDB)
}

func (smc *Controller) deletePodDisruptionBudget(obj interface{}) {
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
	smc.enqueueOwner(pdb)
}

func (smc *Controller) addService(obj interface{}) {
	svc := obj.(*corev1.Service)
	klog.V(4).InfoS("Observed addition of Service", "Service", klog.KObj(svc))
	smc.enqueueOwner(svc)
}

func (smc *Controller) updateService(old, cur interface{}) {
	oldService := old.(*corev1.Service)
	currentService := cur.(*corev1.Service)

	if currentService.UID != oldService.UID {
		key, err := keyFunc(oldService)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldService, err))
			return
		}
		smc.deleteService(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldService,
		})
	}

	klog.V(4).InfoS("Observed update of Service", "Service", klog.KObj(oldService))
	smc.enqueueOwner(currentService)
}

func (smc *Controller) deleteService(obj interface{}) {
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
	smc.enqueueOwner(svc)
}

func (smc *Controller) addSecret(obj interface{}) {
	secret := obj.(*corev1.Secret)
	klog.V(4).InfoS("Observed addition of Secret", "Secret", klog.KObj(secret))
	smc.enqueueNamespace(secret.Namespace)
}

func (smc *Controller) updateSecret(old, cur interface{}) {
	oldSecret := old.(*corev1.Secret)
	currentSecret := cur.(*corev1.Secret)

	if currentSecret.UID != oldSecret.UID {
		key, err := keyFunc(oldSecret)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSecret, err))
			return
		}
		smc.deleteSecret(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSecret,
		})
	}

	klog.V(4).InfoS("Observed update of Secret", "Secret", klog.KObj(oldSecret))
	smc.enqueueNamespace(currentSecret.Namespace)
}

func (smc *Controller) deleteSecret(obj interface{}) {
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
	smc.enqueueNamespace(secret.Namespace)
}

func (smc *Controller) addConfigMap(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(cm))
	smc.enqueueOwner(cm)
}

func (smc *Controller) updateConfigMap(old, cur interface{}) {
	oldCM := old.(*corev1.ConfigMap)
	currentCM := cur.(*corev1.ConfigMap)

	if currentCM.UID != oldCM.UID {
		key, err := keyFunc(oldCM)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldCM, err))
			return
		}
		smc.deleteConfigMap(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldCM,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldCM))
	smc.enqueueOwner(currentCM)
}

func (smc *Controller) deleteConfigMap(obj interface{}) {
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
	smc.enqueueOwner(cm)
}

func (smc *Controller) addDeployment(obj interface{}) {
	d := obj.(*appsv1.Deployment)
	klog.V(4).InfoS("Observed addition of Deployment", "Deployment", klog.KObj(d))
	smc.enqueueOwner(d)
}

func (smc *Controller) updateDeployment(old, cur interface{}) {
	oldD := old.(*appsv1.Deployment)
	currentD := cur.(*appsv1.Deployment)

	if currentD.UID != oldD.UID {
		key, err := keyFunc(oldD)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldD, err))
			return
		}
		smc.deleteDeployment(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldD,
		})
	}

	klog.V(4).InfoS("Observed update of Deployment", "Deployment", klog.KObj(oldD))
	smc.enqueueOwner(currentD)
}

func (smc *Controller) deleteDeployment(obj interface{}) {
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		d, ok = tombstone.Obj.(*appsv1.Deployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Deployment %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Deployment", "Deployment", klog.KObj(d))
	smc.enqueueOwner(d)
}
