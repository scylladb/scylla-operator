// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/util/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "NodeConfigController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 30 * time.Second
)

var (
	keyFunc                = cache.DeletionHandlingMetaNamespaceKeyFunc
	controllerGVK          = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	nodeConfigLister           scyllav1alpha1listers.NodeConfigLister
	scyllaOperatorConfigLister scyllav1alpha1listers.ScyllaOperatorConfigLister
	clusterRoleLister          rbacv1listers.ClusterRoleLister
	clusterRoleBindingLister   rbacv1listers.ClusterRoleBindingLister
	daemonSetLister            appsv1listers.DaemonSetLister
	namespaceLister            corev1listers.NamespaceLister
	nodeLister                 corev1listers.NodeLister
	serviceAccountLister       corev1listers.ServiceAccountLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	operatorImage string
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	nodeConfigInformer scyllav1alpha1informers.NodeConfigInformer,
	scyllaOperatorConfigInformer scyllav1alpha1informers.ScyllaOperatorConfigInformer,
	clusterRoleInformer rbacv1informers.ClusterRoleInformer,
	clusterRoleBindingInformer rbacv1informers.ClusterRoleBindingInformer,
	daemonSetInformer appsv1informers.DaemonSetInformer,
	namespaceInformer corev1informers.NamespaceInformer,
	nodeInformer corev1informers.NodeInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	operatorImage string,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"nodeconfig_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}
	ncc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		nodeConfigLister:           nodeConfigInformer.Lister(),
		scyllaOperatorConfigLister: scyllaOperatorConfigInformer.Lister(),
		clusterRoleLister:          clusterRoleInformer.Lister(),
		clusterRoleBindingLister:   clusterRoleBindingInformer.Lister(),
		daemonSetLister:            daemonSetInformer.Lister(),
		namespaceLister:            namespaceInformer.Lister(),
		nodeLister:                 nodeInformer.Lister(),
		serviceAccountLister:       serviceAccountInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			nodeConfigInformer.Informer().HasSynced,
			scyllaOperatorConfigInformer.Informer().HasSynced,
			clusterRoleInformer.Informer().HasSynced,
			clusterRoleBindingInformer.Informer().HasSynced,
			daemonSetInformer.Informer().HasSynced,
			namespaceInformer.Informer().HasSynced,
			nodeInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "NodeConfig-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeConfig"),

		operatorImage: operatorImage,
	}

	nodeConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addNodeConfig,
		UpdateFunc: ncc.updateNodeConfig,
		DeleteFunc: ncc.deleteNodeConfig,
	})

	// TODO: react to label changes on nodes
	// nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    ncc.addNode,
	// 	UpdateFunc: ncc.updateNode,
	// 	DeleteFunc: ncc.deleteNode,
	// })

	scyllaOperatorConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addOperatorConfig,
		UpdateFunc: ncc.updateOperatorConfig,
	})

	clusterRoleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addClusterRole,
		UpdateFunc: ncc.updateClusterRole,
		DeleteFunc: ncc.deleteClusterRole,
	})

	clusterRoleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addClusterRoleBinding,
		UpdateFunc: ncc.updateClusterRoleBinding,
		DeleteFunc: ncc.deleteClusterRoleBinding,
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addServiceAccount,
		UpdateFunc: ncc.updateServiceAccount,
		DeleteFunc: ncc.deleteServiceAccount,
	})

	daemonSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addDaemonSet,
		UpdateFunc: ncc.updateDaemonSet,
		DeleteFunc: ncc.deleteDaemonSet,
	})

	return ncc, nil
}

func (ncc *Controller) addDaemonSet(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	klog.V(4).InfoS("Observed addition of DaemonSet", "DaemonSet", klog.KObj(ds))
	ncc.enqueueOwner(ds)
}

func (ncc *Controller) updateDaemonSet(old, cur interface{}) {
	oldDaemonSet := old.(*appsv1.DaemonSet)
	currentDaemonSet := cur.(*appsv1.DaemonSet)

	if currentDaemonSet.UID != oldDaemonSet.UID {
		key, err := keyFunc(oldDaemonSet)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldDaemonSet, err))
			return
		}
		ncc.deleteDaemonSet(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldDaemonSet,
		})
	}

	klog.V(4).InfoS("Observed update of DaemonSet", "DaemonSet", klog.KObj(oldDaemonSet))
	ncc.enqueueOwner(currentDaemonSet)
}

func (ncc *Controller) deleteDaemonSet(obj interface{}) {
	ds, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		ds, ok = tombstone.Obj.(*appsv1.DaemonSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a DaemonSet %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of DaemonSet", "DaemonSet", klog.KObj(ds))
	ncc.enqueueOwner(ds)
}

func (ncc *Controller) addServiceAccount(obj interface{}) {
	sa := obj.(*corev1.ServiceAccount)
	klog.V(4).InfoS("Observed addition of ServiceAccount", "ServiceAccount", klog.KObj(sa))
	ncc.enqueueOwner(sa)
}

func (ncc *Controller) updateServiceAccount(old, cur interface{}) {
	oldServiceAccount := old.(*corev1.ServiceAccount)
	currentServiceAccount := cur.(*corev1.ServiceAccount)

	if currentServiceAccount.UID != oldServiceAccount.UID {
		key, err := keyFunc(oldServiceAccount)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldServiceAccount, err))
			return
		}
		ncc.deleteServiceAccount(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldServiceAccount,
		})
	}

	klog.V(4).InfoS("Observed update of ServiceAccount", "ServiceAccount", klog.KObj(oldServiceAccount))
	ncc.enqueueOwner(currentServiceAccount)
}

func (ncc *Controller) deleteServiceAccount(obj interface{}) {
	sa, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sa, ok = tombstone.Obj.(*corev1.ServiceAccount)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ServiceAccount %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ServiceAccount", "ServiceAccount", klog.KObj(sa))
	ncc.enqueueOwner(sa)
}

func (ncc *Controller) addClusterRoleBinding(obj interface{}) {
	crb := obj.(*rbacv1.ClusterRoleBinding)
	klog.V(4).InfoS("Observed addition of ClusterRoleBinding", "ClusterRoleBinding", klog.KObj(crb))
	ncc.enqueueOwner(crb)
}

func (ncc *Controller) updateClusterRoleBinding(old, cur interface{}) {
	oldClusterRoleBinding := old.(*rbacv1.ClusterRoleBinding)
	currentClusterRoleBinding := cur.(*rbacv1.ClusterRoleBinding)

	if currentClusterRoleBinding.UID != oldClusterRoleBinding.UID {
		key, err := keyFunc(oldClusterRoleBinding)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldClusterRoleBinding, err))
			return
		}
		ncc.deleteClusterRoleBinding(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldClusterRoleBinding,
		})
	}

	klog.V(4).InfoS("Observed update of ClusterRoleBinding", "ClusterRoleBinding", klog.KObj(oldClusterRoleBinding))
	ncc.enqueueOwner(currentClusterRoleBinding)
}

func (ncc *Controller) deleteClusterRoleBinding(obj interface{}) {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		crb, ok = tombstone.Obj.(*rbacv1.ClusterRoleBinding)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ClusterRoleBinding %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ClusterRoleBinding", "ClusterRoleBinding", klog.KObj(crb))
	ncc.enqueueOwner(crb)
}

func (ncc *Controller) addClusterRole(obj interface{}) {
	cr := obj.(*rbacv1.ClusterRole)
	klog.V(4).InfoS("Observed addition of ClusterRole", "ClusterRole", klog.KObj(cr))
	ncc.enqueueOwner(cr)
}

func (ncc *Controller) updateClusterRole(old, cur interface{}) {
	oldClusterRole := old.(*rbacv1.ClusterRole)
	currentClusterRole := cur.(*rbacv1.ClusterRole)

	if currentClusterRole.UID != oldClusterRole.UID {
		key, err := keyFunc(oldClusterRole)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldClusterRole, err))
			return
		}
		ncc.deleteClusterRole(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldClusterRole,
		})
	}

	klog.V(4).InfoS("Observed update of ClusterRole", "ClusterRole", klog.KObj(oldClusterRole))
	ncc.enqueueOwner(currentClusterRole)
}

func (ncc *Controller) deleteClusterRole(obj interface{}) {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		cr, ok = tombstone.Obj.(*rbacv1.ClusterRole)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ClusterRole %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ClusterRole", "ClusterRole", klog.KObj(cr))
	ncc.enqueueOwner(cr)
}

func (ncc *Controller) addNodeConfig(obj interface{}) {
	nodeConfig := obj.(*scyllav1alpha1.NodeConfig)
	klog.V(4).InfoS("Observed addition of NodeConfig", "NodeConfig", klog.KObj(nodeConfig))
	ncc.enqueue(nodeConfig)
}

func (ncc *Controller) updateNodeConfig(old, cur interface{}) {
	oldNodeConfig := old.(*scyllav1alpha1.NodeConfig)
	currentNodeConfig := cur.(*scyllav1alpha1.NodeConfig)

	if currentNodeConfig.UID != oldNodeConfig.UID {
		key, err := keyFunc(oldNodeConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNodeConfig, err))
			return
		}
		ncc.deleteNodeConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNodeConfig,
		})
	}

	klog.V(4).InfoS("Observed update of NodeConfig", "NodeConfig", klog.KObj(oldNodeConfig))
	ncc.enqueue(currentNodeConfig)
}

func (ncc *Controller) deleteNodeConfig(obj interface{}) {
	nodeConfig, ok := obj.(*scyllav1alpha1.NodeConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		nodeConfig, ok = tombstone.Obj.(*scyllav1alpha1.NodeConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a NodeConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of NodeConfig", "NodeConfig", klog.KObj(nodeConfig))
	ncc.enqueue(nodeConfig)
}

func (ncc *Controller) addOperatorConfig(obj interface{}) {
	operatorConfig := obj.(*scyllav1alpha1.ScyllaOperatorConfig)
	klog.V(4).InfoS("Observed addition of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(operatorConfig))
	ncc.enqueueAll()
}

func (ncc *Controller) updateOperatorConfig(old, cur interface{}) {
	oldOperatorConfig := old.(*scyllav1alpha1.ScyllaOperatorConfig)

	klog.V(4).InfoS("Observed update of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(oldOperatorConfig))
	ncc.enqueueAll()
}

func (ncc *Controller) resolveNodeConfigController(obj metav1.Object) *scyllav1alpha1.NodeConfig {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != controllerGVK.Kind {
		return nil
	}

	nc, err := ncc.nodeConfigLister.Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if nc.UID != controllerRef.UID {
		return nil
	}

	return nc
}

func (ncc *Controller) enqueueOwner(obj metav1.Object) {
	nc := ncc.resolveNodeConfigController(obj)
	if nc == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "NodeConfig", klog.KObj(nc))
	ncc.enqueue(nc)
}

func (ncc *Controller) enqueue(soc *scyllav1alpha1.NodeConfig) {
	key, err := keyFunc(soc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", soc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "NodeConfig", klog.KObj(soc))
	ncc.queue.Add(key)
}

func (ncc *Controller) enqueueAll() {
	ncs, err := ncc.nodeConfigLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't list all NodeConfigs: %w", err))
		return
	}

	for _, soc := range ncs {
		ncc.enqueue(soc)
	}
}

func (ncc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := ncc.queue.Get()
	if quit {
		return false
	}
	defer ncc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := ncc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		ncc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	ncc.queue.AddRateLimited(key)

	return true
}

func (ncc *Controller) runWorker(ctx context.Context) {
	for ncc.processNextItem(ctx) {
	}
}

func (ncc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		ncc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), ncc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, ncc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}
