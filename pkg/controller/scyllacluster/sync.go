package scyllacluster

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
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
		scyllaClusterControllerGVK,
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
		scyllaClusterControllerGVK,
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
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealSecretControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimSecrets(secrets)
}

func (scc *Controller) getConfigMaps(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*corev1.ConfigMap, error) {
	// List all ConfigMaps to find even those that no longer match our selector.
	// They will be orphaned in ClaimConfigMaps().
	configMaps, err := scc.configMapLister.ConfigMaps(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing ConfigMaps.
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
	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		sc,
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(configMaps)
}

func (scc *Controller) getServiceAccounts(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*corev1.ServiceAccount, error) {
	// List all ServiceAccounts to find even those that no longer match our selector.
	// They will be orphaned in ClaimServiceAccount().
	serviceAccounts, err := scc.serviceAccountLister.ServiceAccounts(sc.Namespace).List(labels.Everything())
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
	cm := controllertools.NewServiceAccountControllerRefManager(
		ctx,
		sc,
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceAccountControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimServiceAccounts(serviceAccounts)
}

func (scc *Controller) getRoleBindings(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*rbacv1.RoleBinding, error) {
	// List all RoleBindings to find even those that no longer match our selector.
	// They will be orphaned in ClaimRoleBindings().
	roleBindings, err := scc.roleBindingLister.RoleBindings(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing RoleBindings.
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
	cm := controllertools.NewRoleBindingControllerRefManager(
		ctx,
		sc,
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealRoleBindingControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimRoleBindings(roleBindings)
}

func (scc *Controller) getPDBs(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*policyv1.PodDisruptionBudget, error) {
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
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealPodDisruptionBudgetControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimPodDisruptionBudgets(pdbs)
}

func (scc *Controller) getIngresses(ctx context.Context, sc *scyllav1.ScyllaCluster) (map[string]*networkingv1.Ingress, error) {
	// List all Ingresses to find even those that no longer match our selector.
	// They will be orphaned in ClaimIngress().
	ingresses, err := scc.ingressLister.Ingresses(sc.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Ingresses.
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
	cm := controllertools.NewIngressControllerRefManager(
		ctx,
		sc,
		scyllaClusterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealIngressControl{
			KubeClient: scc.kubeClient,
			Recorder:   scc.eventRecorder,
		},
	)
	return cm.ClaimIngresss(ingresses)
}

func runSync(conditions *[]metav1.Condition, progressingConditionType, degradedCondType string, observedGeneration int64, syncFn func() ([]metav1.Condition, error)) error {
	progressingConditions, err := syncFn()
	controllerhelpers.SetStatusConditionFromError(conditions, err, degradedCondType, observedGeneration)
	if err != nil {
		return err
	}

	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		progressingConditions,
		metav1.Condition{
			Type:               progressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: observedGeneration,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate progressing conditions %q: %w", progressingConditionType, err)
	}
	apimeta.SetStatusCondition(conditions, progressingCondition)

	return nil
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

	configMapMap, err := scc.getConfigMaps(ctx, sc)
	if err != nil {
		return err
	}

	serviceAccounts, err := scc.getServiceAccounts(ctx, sc)
	if err != nil {
		return fmt.Errorf("can't get serviceaccounts: %w", err)
	}

	roleBindings, err := scc.getRoleBindings(ctx, sc)
	if err != nil {
		return fmt.Errorf("can't get rolebindings: %w", err)
	}

	pdbMap, err := scc.getPDBs(ctx, sc)
	if err != nil {
		return err
	}

	ingressMap, err := scc.getIngresses(ctx, sc)
	if err != nil {
		return fmt.Errorf("can't get ingresses: %w", err)
	}

	status := scc.calculateStatus(sc, statefulSetMap, serviceMap)

	if sc.DeletionTimestamp != nil {
		return scc.updateStatus(ctx, sc, status)
	}

	var errs []error

	err = runSync(
		&status.Conditions,
		serviceAccountControllerProgressingCondition,
		serviceAccountControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncServiceAccounts(ctx, sc, serviceAccounts)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync service accounts: %w", err))
	}

	err = runSync(
		&status.Conditions,
		roleBindingControllerProgressingCondition,
		roleBindingControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncRoleBindings(ctx, sc, roleBindings)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync role bindings: %w", err))
	}

	err = runSync(
		&status.Conditions,
		agentTokenControllerProgressingCondition,
		agentTokenControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncAgentToken(ctx, sc, secretMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync agent token: %w", err))
	}

	if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
		err = runSync(
			&status.Conditions,
			certControllerProgressingCondition,
			certControllerDegradedCondition,
			sc.Generation,
			func() ([]metav1.Condition, error) {
				return scc.syncCerts(ctx, sc, secretMap, configMapMap, serviceMap)
			},
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync certificates: %w", err))
		}
	}

	err = runSync(
		&status.Conditions,
		statefulSetControllerProgressingCondition,
		statefulSetControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncStatefulSets(ctx, key, sc, status, statefulSetMap, serviceMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync stateful sets: %w", err))
	}
	// Ideally, this would be projected in calculateStatus but because we are updating the status based on applied
	// StatefulSets, the rack status can change afterwards. Overtime we should consider adding a status.progressing
	// field (to allow determining cluster status without conditions) and wait for the status to be updated
	// in a single place, on the next resync.
	scc.setStatefulSetsAvailableStatusCondition(sc, status)

	err = runSync(
		&status.Conditions,
		serviceControllerProgressingCondition,
		serviceControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncServices(ctx, sc, status, serviceMap, statefulSetMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync services: %w", err))
	}

	err = runSync(
		&status.Conditions,
		pdbControllerProgressingCondition,
		pdbControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncPodDisruptionBudgets(ctx, sc, pdbMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync pdbs: %w", err))
	}

	err = runSync(
		&status.Conditions,
		ingressControllerProgressingCondition,
		ingressControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncIngresses(ctx, sc, ingressMap, serviceMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ingresses: %w", err))
	}

	// Aggregate conditions.

	availableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1.AvailableCondition),
		metav1.Condition{
			Type:               scyllav1.AvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, availableCondition)

	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1.ProgressingCondition),
		metav1.Condition{
			Type:               scyllav1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)

	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1.DegradedCondition),
		metav1.Condition{
			Type:               scyllav1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = scc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}
