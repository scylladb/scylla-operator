// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	nodeconfigresources "github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/resource"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
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
	controllerGVK          = scyllav1alpha1.GroupVersion.WithKind("ScyllaNodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	scyllaNodeConfigLister     scyllav1alpha1listers.ScyllaNodeConfigLister
	scyllaOperatorConfigLister scyllav1alpha1listers.ScyllaOperatorConfigLister
	clusterRoleLister          rbacv1listers.ClusterRoleLister
	clusterRoleBindingLister   rbacv1listers.ClusterRoleBindingLister
	daemonSetLister            appsv1listers.DaemonSetLister
	deploymentLister           appsv1listers.DeploymentLister
	configMapLister            corev1listers.ConfigMapLister
	namespaceLister            corev1listers.NamespaceLister
	nodeLister                 corev1listers.NodeLister
	serviceAccountLister       corev1listers.ServiceAccountLister
	podLister                  corev1listers.PodLister
	jobLister                  batchv1listers.JobLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	operatorImage string
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	scyllaNodeConfigInformer scyllav1alpha1informers.ScyllaNodeConfigInformer,
	scyllaOperatorConfigInformer scyllav1alpha1informers.ScyllaOperatorConfigInformer,
	clusterRoleInformer rbacv1informers.ClusterRoleInformer,
	clusterRoleBindingInformer rbacv1informers.ClusterRoleBindingInformer,
	daemonSetInformer appsv1informers.DaemonSetInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	namespaceInformer corev1informers.NamespaceInformer,
	nodeInformer corev1informers.NodeInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	podInformer corev1informers.PodInformer,
	jobInformer batchv1informers.JobInformer,
	operatorImage string,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllanodeconfig_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	sncc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaNodeConfigLister:     scyllaNodeConfigInformer.Lister(),
		scyllaOperatorConfigLister: scyllaOperatorConfigInformer.Lister(),
		clusterRoleLister:          clusterRoleInformer.Lister(),
		clusterRoleBindingLister:   clusterRoleBindingInformer.Lister(),
		daemonSetLister:            daemonSetInformer.Lister(),
		configMapLister:            configMapInformer.Lister(),
		namespaceLister:            namespaceInformer.Lister(),
		nodeLister:                 nodeInformer.Lister(),
		serviceAccountLister:       serviceAccountInformer.Lister(),
		podLister:                  podInformer.Lister(),
		jobLister:                  jobInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			scyllaNodeConfigInformer.Informer().HasSynced,
			scyllaOperatorConfigInformer.Informer().HasSynced,
			clusterRoleInformer.Informer().HasSynced,
			clusterRoleBindingInformer.Informer().HasSynced,
			daemonSetInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			namespaceInformer.Informer().HasSynced,
			nodeInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
			podInformer.Informer().HasSynced,
			jobInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "ScyllaNodeConfig-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ScyllaNodeConfig"),

		operatorImage: operatorImage,
	}

	sncc.enqueue(nodeconfigresources.DefaultScyllaNodeConfig())

	scyllaNodeConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addNodeConfig,
		UpdateFunc: sncc.updateConfig,
		DeleteFunc: sncc.deleteConfig,
	})

	scyllaOperatorConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addOperatorConfig,
		UpdateFunc: sncc.updateOperatorConfig,
		DeleteFunc: sncc.deleteOperatorConfig,
	})

	clusterRoleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addClusterRole,
		UpdateFunc: sncc.updateClusterRole,
		DeleteFunc: sncc.deleteClusterRole,
	})

	clusterRoleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addClusterRoleBinding,
		UpdateFunc: sncc.updateClusterRoleBinding,
		DeleteFunc: sncc.deleteClusterRoleBinding,
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addServiceAccount,
		UpdateFunc: sncc.updateServiceAccount,
		DeleteFunc: sncc.deleteServiceAccount,
	})

	daemonSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addDaemonSet,
		UpdateFunc: sncc.updateDaemonSet,
		DeleteFunc: sncc.deleteDaemonSet,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addPod,
		UpdateFunc: sncc.updatePod,
		DeleteFunc: sncc.deletePod,
	})

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addJob,
		UpdateFunc: sncc.updateJob,
		DeleteFunc: sncc.deleteJob,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sncc.addConfigMap,
		UpdateFunc: sncc.updateConfigMap,
		DeleteFunc: sncc.deleteConfigMap,
	})

	return sncc, nil
}

func (sncc *Controller) addDaemonSet(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	klog.V(4).InfoS("Observed addition of DaemonSet", "DaemonSet", klog.KObj(ds))
	sncc.enqueueOwner(ds)
}

func (sncc *Controller) updateDaemonSet(old, cur interface{}) {
	oldDaemonSet := old.(*appsv1.DaemonSet)
	currentDaemonSet := cur.(*appsv1.DaemonSet)

	if currentDaemonSet.UID != oldDaemonSet.UID {
		key, err := keyFunc(oldDaemonSet)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldDaemonSet, err))
			return
		}
		sncc.deleteDaemonSet(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldDaemonSet,
		})
	}

	klog.V(4).InfoS("Observed update of DaemonSet", "DaemonSet", klog.KObj(oldDaemonSet))
	sncc.enqueueOwner(currentDaemonSet)
}

func (sncc *Controller) deleteDaemonSet(obj interface{}) {
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
	sncc.enqueueOwner(ds)
}

func (sncc *Controller) addServiceAccount(obj interface{}) {
	sa := obj.(*corev1.ServiceAccount)
	klog.V(4).InfoS("Observed addition of ServiceAccount", "ServiceAccount", klog.KObj(sa))
	sncc.enqueueOwner(sa)
}

func (sncc *Controller) updateServiceAccount(old, cur interface{}) {
	oldServiceAccount := old.(*corev1.ServiceAccount)
	currentServiceAccount := cur.(*corev1.ServiceAccount)

	if currentServiceAccount.UID != oldServiceAccount.UID {
		key, err := keyFunc(oldServiceAccount)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldServiceAccount, err))
			return
		}
		sncc.deleteServiceAccount(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldServiceAccount,
		})
	}

	klog.V(4).InfoS("Observed update of ServiceAccount", "ServiceAccount", klog.KObj(oldServiceAccount))
	sncc.enqueueOwner(currentServiceAccount)
}

func (sncc *Controller) deleteServiceAccount(obj interface{}) {
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
	sncc.enqueueOwner(sa)
}

func (sncc *Controller) addClusterRoleBinding(obj interface{}) {
	crb := obj.(*rbacv1.ClusterRoleBinding)
	klog.V(4).InfoS("Observed addition of ClusterRoleBinding", "ClusterRoleBinding", klog.KObj(crb))
	sncc.enqueueOwner(crb)
}

func (sncc *Controller) updateClusterRoleBinding(old, cur interface{}) {
	oldClusterRoleBinding := old.(*rbacv1.ClusterRoleBinding)
	currentClusterRoleBinding := cur.(*rbacv1.ClusterRoleBinding)

	if currentClusterRoleBinding.UID != oldClusterRoleBinding.UID {
		key, err := keyFunc(oldClusterRoleBinding)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldClusterRoleBinding, err))
			return
		}
		sncc.deleteClusterRoleBinding(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldClusterRoleBinding,
		})
	}

	klog.V(4).InfoS("Observed update of ClusterRoleBinding", "ClusterRoleBinding", klog.KObj(oldClusterRoleBinding))
	sncc.enqueueOwner(currentClusterRoleBinding)
}

func (sncc *Controller) deleteClusterRoleBinding(obj interface{}) {
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
	sncc.enqueueOwner(crb)
}

func (sncc *Controller) addClusterRole(obj interface{}) {
	cr := obj.(*rbacv1.ClusterRole)
	klog.V(4).InfoS("Observed addition of ClusterRole", "ClusterRole", klog.KObj(cr))
	sncc.enqueueOwner(cr)
}

func (sncc *Controller) updateClusterRole(old, cur interface{}) {
	oldClusterRole := old.(*rbacv1.ClusterRole)
	currentClusterRole := cur.(*rbacv1.ClusterRole)

	if currentClusterRole.UID != oldClusterRole.UID {
		key, err := keyFunc(oldClusterRole)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldClusterRole, err))
			return
		}
		sncc.deleteClusterRole(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldClusterRole,
		})
	}

	klog.V(4).InfoS("Observed update of ClusterRole", "ClusterRole", klog.KObj(oldClusterRole))
	sncc.enqueueOwner(currentClusterRole)
}

func (sncc *Controller) deleteClusterRole(obj interface{}) {
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
	sncc.enqueueOwner(cr)
}

func (sncc *Controller) addNodeConfig(obj interface{}) {
	nodeConfig := obj.(*scyllav1alpha1.ScyllaNodeConfig)
	klog.V(4).InfoS("Observed addition of ScyllaNodeConfig", "ScyllaNodeConfig", klog.KObj(nodeConfig))
	sncc.enqueue(nodeConfig)
}

func (sncc *Controller) updateConfig(old, cur interface{}) {
	oldNodeConfig := old.(*scyllav1alpha1.ScyllaNodeConfig)
	currentNodeConfig := cur.(*scyllav1alpha1.ScyllaNodeConfig)

	if currentNodeConfig.UID != oldNodeConfig.UID {
		key, err := keyFunc(oldNodeConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldNodeConfig, err))
			return
		}
		sncc.deleteConfig(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNodeConfig,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaNodeConfig", "ScyllaNodeConfig", klog.KObj(oldNodeConfig))
	sncc.enqueue(currentNodeConfig)
}

func (sncc *Controller) deleteConfig(obj interface{}) {
	nodeConfig, ok := obj.(*scyllav1alpha1.ScyllaNodeConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		nodeConfig, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaNodeConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaNodeConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaNodeConfig", "ScyllaNodeConfig", klog.KObj(nodeConfig))
	sncc.enqueue(nodeConfig)
}

func (sncc *Controller) addOperatorConfig(obj interface{}) {
	operatorConfig := obj.(*scyllav1alpha1.ScyllaOperatorConfig)
	klog.V(4).InfoS("Observed addition of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(operatorConfig))
	sncc.enqueueAll()
}

func (sncc *Controller) updateOperatorConfig(old, cur interface{}) {
	oldOperatorConfig := old.(*scyllav1alpha1.ScyllaOperatorConfig)

	klog.V(4).InfoS("Observed update of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(oldOperatorConfig))
	sncc.enqueueAll()
}

func (sncc *Controller) deleteOperatorConfig(obj interface{}) {
	operatorConfig, ok := obj.(*scyllav1alpha1.ScyllaOperatorConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		operatorConfig, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaOperatorConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaOperatorConfig %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(operatorConfig))
	sncc.enqueueAll()
}

func (c *Controller) resolveDaemonSetController(obj metav1.Object) *appsv1.DaemonSet {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != daemonSetControllerGVK.Kind {
		return nil
	}

	ds, err := c.daemonSetLister.DaemonSets(obj.GetNamespace()).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if ds.UID != controllerRef.UID {
		return nil
	}

	return ds
}

func (c *Controller) enqueueThroughDaemonSetControllerOrScyllaPod(pod *corev1.Pod) {
	ds := c.resolveDaemonSetController(pod)
	if ds == nil {
		if !naming.ScyllaSelector().Matches(labels.Set(pod.GetLabels())) {
			return
		}

		// Enqueue owner responsible for optimizing this Scylla Pod.
		sncs, err := c.scyllaNodeConfigLister.List(labels.Everything())
		if err != nil {
			utilruntime.HandleError(err)
			return
		}

		for _, snc := range sncs {
			sncDs, err := c.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).Get(snc.Name)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}

			sncPods, err := c.podLister.Pods(naming.ScyllaOperatorNodeTuningNamespace).List(labels.SelectorFromSet(sncDs.Spec.Selector.MatchLabels))
			if err != nil {
				utilruntime.HandleError(err)
				return
			}

			owner := nodeconfigresources.DefaultScyllaNodeConfig()
			for _, sncPod := range sncPods {
				if sncPod.Spec.NodeName == pod.Spec.NodeName {
					klog.V(4).InfoS("Enqueuing through Scylla Pod", "Pod", klog.KObj(pod), "ScyllaNodeConfig", klog.KObj(snc))
					owner = snc
				}
			}

			// Enqueue default ScyllaNodeConfig, it has to drop CM first if ownership changed.
			if owner.Name != nodeconfigresources.DefaultScyllaNodeConfig().Name {
				c.enqueue(nodeconfigresources.DefaultScyllaNodeConfig())
			}

			c.enqueue(owner)
		}

		return
	}

	snc := c.resolveNodeConfigController(ds)
	if snc == nil {
		return
	}

	// Enqueue default ScyllaNodeConfig. It has to drop CM first if ownership changed.
	if snc.Name != nodeconfigresources.DefaultScyllaNodeConfig().Name {
		c.enqueue(nodeconfigresources.DefaultScyllaNodeConfig())
	}
	klog.V(4).InfoS("Enqueuing through owner", "Pod", klog.KObj(pod), "ScyllaNodeConfig", klog.KObj(snc))
	c.enqueue(snc)
}

func (c *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(pod)
}

func (c *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	if currentPod.UID != oldPod.UID {
		key, err := keyFunc(oldPod)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldPod, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldPod,
		})
	}

	klog.V(4).InfoS("Observed update of Pod", "Pod", klog.KObj(oldPod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(currentPod)
}

func (c *Controller) deletePod(obj interface{}) {
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
	klog.V(4).InfoS("Observed deletion of Pod", "Pod", klog.KObj(pod))
	c.enqueueThroughDaemonSetControllerOrScyllaPod(pod)
}

func (c *Controller) addJob(obj interface{}) {
	job := obj.(*batchv1.Job)
	klog.V(4).InfoS("Observed addition of Job", "Job", klog.KObj(job))
	c.enqueueOwner(job)
}

func (c *Controller) updateJob(old, cur interface{}) {
	oldJob := old.(*batchv1.Job)
	currentJob := cur.(*batchv1.Job)

	if currentJob.UID != oldJob.UID {
		key, err := keyFunc(oldJob)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldJob, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldJob,
		})
	}

	klog.V(4).InfoS("Observed update of Job", "Job", klog.KObj(oldJob))
	c.enqueueOwner(currentJob)
}

func (c *Controller) deleteJob(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		job, ok = tombstone.Obj.(*batchv1.Job)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of Job", "Job", klog.KObj(job))
	c.enqueueOwner(job)
}

func (c *Controller) addConfigMap(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	klog.V(4).InfoS("Observed addition of ConfigMap", "ConfigMap", klog.KObj(cm))
	c.enqueueOwner(cm)
}

func (c *Controller) updateConfigMap(old, cur interface{}) {
	oldConfigMap := old.(*corev1.ConfigMap)
	currentConfigMap := cur.(*corev1.ConfigMap)

	if currentConfigMap.UID != oldConfigMap.UID {
		key, err := keyFunc(oldConfigMap)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldConfigMap, err))
			return
		}
		c.deletePod(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldConfigMap,
		})
	}

	klog.V(4).InfoS("Observed update of ConfigMap", "ConfigMap", klog.KObj(oldConfigMap))
	c.enqueueOwner(currentConfigMap)
}

func (c *Controller) deleteConfigMap(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		configMap, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}

	klog.V(4).InfoS("Observed deletion of ConfigMap", "ConfigMap", klog.KObj(configMap))
	c.enqueueOwner(configMap)
}

func (sncc *Controller) resolveNodeConfigController(obj metav1.Object) *scyllav1alpha1.ScyllaNodeConfig {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != controllerGVK.Kind {
		return nil
	}

	snt, err := sncc.scyllaNodeConfigLister.Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if snt.UID != controllerRef.UID {
		return nil
	}

	return snt
}

func (sncc *Controller) enqueueOwner(obj metav1.Object) {
	snc := sncc.resolveNodeConfigController(obj)
	if snc == nil {
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing owner", gvk.Kind, klog.KObj(obj), "ScyllaNodeConfig", klog.KObj(snc))
	sncc.enqueue(snc)
}

func (sncc *Controller) enqueue(soc *scyllav1alpha1.ScyllaNodeConfig) {
	key, err := keyFunc(soc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", soc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaNodeConfig", klog.KObj(soc))
	sncc.queue.Add(key)
}

func (sncc *Controller) enqueueAll() {
	socs, err := sncc.scyllaNodeConfigLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't list all ScyllaOperatorConfigs: %w", err))
		return
	}

	for _, soc := range socs {
		sncc.enqueue(soc)
	}
}

func (sncc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := sncc.queue.Get()
	if quit {
		return false
	}
	defer sncc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := sncc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		sncc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	sncc.queue.AddRateLimited(key)

	return true
}

func (sncc *Controller) runWorker(ctx context.Context) {
	for sncc.processNextItem(ctx) {
	}
}

func (sncc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		sncc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), sncc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, sncc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}
