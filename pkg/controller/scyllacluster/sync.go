package scyllacluster

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scc *Controller) getStatefulSets(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*appsv1.StatefulSet, error) {
	// List all StatefulSets to find even those that no longer match our selector.
	// They will be orphaned in ClaimStatefulSets().
	statefulSets, err := scc.statefulSetLister.StatefulSets(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing StatefulSets.
	canAdoptFunc := func() error {
		fresh, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sc.UID {
			return fmt.Errorf("original ScyllaCluster %v/%v is gone: got uid %v, wanted %v", sc.Namespace, sc.Name, fresh.UID, sc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sc.Namespace, sc.Name, sc.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewStatefulSetControllerRefManager(
		ctx,
		sc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealStatefulSetControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimStatefulSets(statefulSets)
}

func (scc *Controller) getServices(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*corev1.Service, error) {
	// List all Services to find even those that no longer match our selector.
	// They will be orphaned in ClaimServices().
	services, err := scc.serviceLister.Services(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Services.
	canAdoptFunc := func() error {
		fresh, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sc.UID {
			return fmt.Errorf("original ScyllaCluster %v/%v is gone: got uid %v, wanted %v", sc.Namespace, sc.Name, fresh.UID, sc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sc.Namespace, sc.Name, sc.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceControllerRefManager(
		ctx,
		sc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimServices(services)
}

func (scc *Controller) getSecrets(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*corev1.Secret, error) {
	// List all Secrets to find even those that no longer match our selector.
	// They will be orphaned in ClaimSecrets().
	secrets, err := scc.secretLister.Secrets(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Secrets.
	canAdoptFunc := func() error {
		fresh, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sc.UID {
			return fmt.Errorf("original ScyllaCluster %v/%v is gone: got uid %v, wanted %v", sc.Namespace, sc.Name, fresh.UID, sc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sc.Namespace, sc.Name, sc.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewSecretControllerRefManager(
		ctx,
		sc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealSecretControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimSecrets(secrets)
}

func (scc *Controller) getPDBs(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*policyv1beta1.PodDisruptionBudget, error) {
	// List all Pdbs to find even those that no longer match our selector.
	// They will be orphaned in ClaimPdbs().
	pdbs, err := scc.pdbLister.PodDisruptionBudgets(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pdbs.
	canAdoptFunc := func() error {
		fresh, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sc.UID {
			return fmt.Errorf("original ScyllaCluster %v/%v is gone: got uid %v, wanted %v", sc.Namespace, sc.Name, fresh.UID, sc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sc.Namespace, sc.Name, sc.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewPodDisruptionBudgetControllerRefManager(
		ctx,
		sc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealPodDisruptionBudgetControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimPodDisruptionBudgets(pdbs)
}

func (scc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := scc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	statefulSetMap, err := scc.getStatefulSets(ctx, sc)
	if err != nil {
		return err
	}

	serviceMap, err := scc.getServices(ctx, sc)
	if err != nil {
		return err
	}

	secretMap, err := scc.getSecrets(ctx, sc)
	if err != nil {
		return err
	}

	pdbMap, err := scc.getPDBs(ctx, sc)
	if err != nil {
		return err
	}

	status := scc.calculateStatus(sc, statefulSetMap, serviceMap)

	if sc.DeletionTimestamp != nil {
		return scc.updateStatus(ctx, sc, status)
	}

	var errs []error

	status, err = scc.syncAgentToken(ctx, sc, status, secretMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = scc.syncStatefulSets(ctx, key, sc, status, statefulSetMap, serviceMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = scc.syncServices(ctx, sc, status, serviceMap, statefulSetMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = scc.syncPodDisruptionBudgets(ctx, sc, status, pdbMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	err = scc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}
