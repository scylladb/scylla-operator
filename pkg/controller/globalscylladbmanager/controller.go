// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	*controllertools.Observer

	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	scyllaDBManagerClusterRegistrationLister scyllav1alpha1listers.ScyllaDBManagerClusterRegistrationLister
	scyllaDBDatacenterLister                 scyllav1alpha1listers.ScyllaDBDatacenterLister
	namespaceLister                          corev1listers.NamespaceLister
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	scyllaDBManagerClusterRegistrationInformer scyllav1alpha1informers.ScyllaDBManagerClusterRegistrationInformer,
	scyllaDBDatacenterInformer scyllav1alpha1informers.ScyllaDBDatacenterInformer,
	namespaceInformer corev1informers.NamespaceInformer,
) (*Controller, error) {
	gsmc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaDBManagerClusterRegistrationLister: scyllaDBManagerClusterRegistrationInformer.Lister(),
		scyllaDBDatacenterLister:                 scyllaDBDatacenterInformer.Lister(),
		namespaceLister:                          namespaceInformer.Lister(),
	}

	observer := controllertools.NewObserver(
		"globalscylladbmanager-controller",
		kubeClient.CoreV1().Events(corev1.NamespaceAll),
		gsmc.sync,
	)

	scyllaDBManagerClusterRegistrationHandler, err := scyllaDBManagerClusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    gsmc.addScyllaDBManagerClusterRegistration,
		UpdateFunc: gsmc.updateScyllaDBManagerClusterRegistration,
		DeleteFunc: gsmc.deleteScyllaDBManagerClusterRegistration,
	})
	if err != nil {
		return nil, fmt.Errorf("can't add ScyllaDBManagerClusterRegistration handler: %w", err)
	}
	observer.AddCachesToSync(scyllaDBManagerClusterRegistrationHandler.HasSynced)

	scyllaDBDatacenterHandler, err := scyllaDBDatacenterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    gsmc.addScyllaDBDatacenter,
		UpdateFunc: gsmc.updateScyllaDBDatacenter,
		DeleteFunc: gsmc.deleteScyllaDBDatacenter,
	})
	if err != nil {
		return nil, fmt.Errorf("can't add ScyllaDBDatacenter handler: %w", err)
	}
	observer.AddCachesToSync(scyllaDBDatacenterHandler.HasSynced)

	namespaceHandler, err := namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    gsmc.addNamespace,
		UpdateFunc: gsmc.updateNamespace,
		DeleteFunc: gsmc.deleteNamespace,
	})
	if err != nil {
		return nil, fmt.Errorf("can't add Namespace handler: %w", err)
	}
	observer.AddCachesToSync(namespaceHandler.HasSynced)

	gsmc.Observer = observer

	// Start immediately, global ScyllaDB Manager may already be deployed.
	gsmc.Enqueue()

	return gsmc, nil
}

func (gsmc *Controller) addScyllaDBManagerClusterRegistration(obj interface{}) {
	smcr := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)

	if !isManagedByGlobalScyllaDBManagerInstance(smcr) {
		klog.V(4).InfoS("Not enqueueing ScyllaDBManagerClusterRegistration not owned by global ScyllaDB Manager", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "RV", smcr.ResourceVersion)
		return
	}

	klog.V(4).InfoS(
		"Observed addition of ScyllaDBManagerClusterRegistration",
		"ScyllaDBManagerClusterRegistration", klog.KObj(smcr),
		"RV", smcr.ResourceVersion,
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) updateScyllaDBManagerClusterRegistration(old, cur interface{}) {
	oldSMCR := old.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
	currentSMCR := cur.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)

	if currentSMCR.UID != oldSMCR.UID {
		key, err := keyFunc(oldSMCR)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get key for object %#v: %w", oldSMCR, err))
			return
		}

		gsmc.deleteScyllaDBManagerClusterRegistration(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSMCR,
		})
	}

	if !isManagedByGlobalScyllaDBManagerInstance(currentSMCR) {
		klog.V(4).InfoS("Not enqueueing ScyllaDBManagerClusterRegistration not owned by global ScyllaDB Manager", "ScyllaDBManagerClusterRegistration", klog.KObj(currentSMCR), "RV", currentSMCR.ResourceVersion)
		return
	}

	klog.V(4).InfoS(
		"Observed update of ScyllaDBManagerClusterRegistration",
		"ScyllaDBManagerClusterRegistration", klog.KObj(currentSMCR),
		"RV", fmt.Sprintf("%s->%s", oldSMCR.ResourceVersion, currentSMCR.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldSMCR.UID, currentSMCR.UID),
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) deleteScyllaDBManagerClusterRegistration(obj interface{}) {
	smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
	if !ok {
		var tombstone cache.DeletedFinalStateUnknown
		tombstone, ok = obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get object from tombstone %#v", obj))
			return
		}
		smcr, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contains an object that is not a ScyllaDBManagerClusterRegistration %#v", obj))
			return
		}
	}

	if !isManagedByGlobalScyllaDBManagerInstance(smcr) {
		klog.V(4).InfoS("Not enqueueing ScyllaDBManagerClusterRegistration not owned by global ScyllaDB Manager", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "RV", smcr.ResourceVersion)
		return
	}

	klog.V(4).InfoS(
		"Observed deletion of ScyllaDBManagerClusterRegistration",
		"ScyllaDBManagerClusterRegistration", klog.KObj(smcr),
		"RV", smcr.ResourceVersion,
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) addScyllaDBDatacenter(obj interface{}) {
	sdc := obj.(*scyllav1alpha1.ScyllaDBDatacenter)

	klog.V(4).InfoS(
		"Observed addition of ScyllaDBDatacenter",
		"ScyllaDBDatacenter", klog.KObj(sdc),
		"RV", sdc.ResourceVersion,
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) updateScyllaDBDatacenter(old, cur interface{}) {
	oldSDC := old.(*scyllav1alpha1.ScyllaDBDatacenter)
	currentSDC := cur.(*scyllav1alpha1.ScyllaDBDatacenter)

	if currentSDC.UID != oldSDC.UID {
		key, err := keyFunc(oldSDC)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get key for object %#v: %w", oldSDC, err))
			return
		}

		gsmc.deleteScyllaDBDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSDC,
		})
	}

	klog.V(4).InfoS(
		"Observed update of ScyllaDBDatacenter",
		"ScyllaDBDatacenter", klog.KObj(currentSDC),
		"RV", fmt.Sprintf("%s->%s", oldSDC.ResourceVersion, currentSDC.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldSDC.UID, currentSDC.UID),
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) deleteScyllaDBDatacenter(obj interface{}) {
	sdc, ok := obj.(*scyllav1alpha1.ScyllaDBDatacenter)
	if !ok {
		var tombstone cache.DeletedFinalStateUnknown
		tombstone, ok = obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get object from tombstone %#v", obj))
			return
		}
		sdc, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDBDatacenter)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contains an object that is not a ScyllaDBDatacenter %#v", obj))
			return
		}
	}

	klog.V(4).InfoS(
		"Observed deletion of ScyllaDBDatacenter",
		"ScyllaDBDatacenter", klog.KObj(sdc),
		"RV", sdc.ResourceVersion,
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) addNamespace(obj interface{}) {
	ns := obj.(*corev1.Namespace)

	if !isGlobalScyllaDBManagerNamespace(ns) {
		return
	}

	klog.V(4).InfoS(
		"Observed addition of Namespace",
		"Namespace", klog.KObj(ns),
		"RV", ns.ResourceVersion,
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) updateNamespace(old, cur interface{}) {
	oldNS := old.(*corev1.Namespace)
	currentNS := cur.(*corev1.Namespace)

	if currentNS.UID != oldNS.UID {
		key, err := keyFunc(oldNS)
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get key for object %#v: %w", oldNS, err))
			return
		}

		gsmc.deleteNamespace(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldNS,
		})
	}

	if !isGlobalScyllaDBManagerNamespace(currentNS) {
		return
	}

	klog.V(4).InfoS(
		"Observed update of Namespace",
		"Namespace", klog.KObj(currentNS),
		"RV", fmt.Sprintf("%s->%s", oldNS.ResourceVersion, currentNS.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldNS.UID, currentNS.UID),
	)
	gsmc.Enqueue()
}

func (gsmc *Controller) deleteNamespace(obj interface{}) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		var tombstone cache.DeletedFinalStateUnknown
		tombstone, ok = obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't get object from tombstone %#v", obj))
			return
		}
		ns, ok = tombstone.Obj.(*corev1.Namespace)
		if !ok {
			apimachineryutilruntime.HandleError(fmt.Errorf("tombstone contains an object that is not a Namespace %#v", obj))
			return
		}
	}

	if !isGlobalScyllaDBManagerNamespace(ns) {
		return
	}

	klog.V(4).InfoS(
		"Observed deletion of Namespace",
		"Namespace", klog.KObj(ns),
		"RV", ns.ResourceVersion,
	)
	gsmc.Enqueue()
}

func isManagedByGlobalScyllaDBManagerInstance(obj metav1.Object) bool {
	return naming.GlobalScyllaDBManagerClusterRegistrationSelector().Matches(labels.Set(obj.GetLabels()))
}

func isGlobalScyllaDBManagerNamespace(ns *corev1.Namespace) bool {
	return ns.Name == naming.ScyllaManagerNamespace
}
