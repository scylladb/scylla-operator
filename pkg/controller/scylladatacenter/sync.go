// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
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

func (sdc *Controller) getStatefulSets(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*appsv1.StatefulSet, error) {
	// List all StatefulSets to find even those that no longer match our selector.
	// They will be orphaned in ClaimStatefulSets().
	statefulSets, err := sdc.statefulSetLister.StatefulSets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing StatefulSets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewStatefulSetControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealStatefulSetControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimStatefulSets(statefulSets)
}

func (sdc *Controller) getServices(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.Service, error) {
	// List all Services to find even those that no longer match our selector.
	// They will be orphaned in ClaimServices().
	services, err := sdc.serviceLister.Services(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Services.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimServices(services)
}

func (sdc *Controller) getSecrets(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.Secret, error) {
	// List all Secrets to find even those that no longer match our selector.
	// They will be orphaned in ClaimSecrets().
	secrets, err := sdc.secretLister.Secrets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Secrets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewSecretControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealSecretControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimSecrets(secrets)
}

func (sdc *Controller) getConfigMaps(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.ConfigMap, error) {
	// List all ConfigMaps to find even those that no longer match our selector.
	// They will be orphaned in ClaimConfigMaps().
	configMaps, err := sdc.configMapLister.ConfigMaps(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing ConfigMaps.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(configMaps)
}

func (sdc *Controller) getServiceAccounts(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.ServiceAccount, error) {
	// List all ServiceAccounts to find even those that no longer match our selector.
	// They will be orphaned in ClaimServiceAccount().
	serviceAccounts, err := sdc.serviceAccountLister.ServiceAccounts(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing StatefulSets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceAccountControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceAccountControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimServiceAccounts(serviceAccounts)
}

func (sdc *Controller) getRoleBindings(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*rbacv1.RoleBinding, error) {
	// List all RoleBindings to find even those that no longer match our selector.
	// They will be orphaned in ClaimRoleBindings().
	roleBindings, err := sdc.roleBindingLister.RoleBindings(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing RoleBindings.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewRoleBindingControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealRoleBindingControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimRoleBindings(roleBindings)
}

func (sdc *Controller) getPDBs(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*policyv1.PodDisruptionBudget, error) {
	// List all Pdbs to find even those that no longer match our selector.
	// They will be orphaned in ClaimPdbs().
	pdbs, err := sdc.pdbLister.PodDisruptionBudgets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pdbs.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewPodDisruptionBudgetControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealPodDisruptionBudgetControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimPodDisruptionBudgets(pdbs)
}

func (scc *Controller) getIngresses(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*networkingv1.Ingress, error) {
	// List all Ingresses to find even those that no longer match our selector.
	// They will be orphaned in ClaimIngress().
	ingresses, err := scc.ingressLister.Ingresses(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Ingresses.
	canAdoptFunc := func() error {
		fresh, err := scc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaCluster %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewIngressControllerRefManager(
		ctx,
		sd,
		scyllaDatacenterControllerGVK,
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

func (sdc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDatacenter", "ScyllaDatacenter", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDatacenter", "ScyllaDatacenter", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sd, err := sdc.scyllaLister.ScyllaDatacenters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDatacenter has been deleted", "ScyllaDatacenter", klog.KObj(sd))
		return nil
	}
	if err != nil {
		return err
	}

	statefulSetMap, err := sdc.getStatefulSets(ctx, sd)
	if err != nil {
		return err
	}

	serviceMap, err := sdc.getServices(ctx, sd)
	if err != nil {
		return err
	}

	secretMap, err := sdc.getSecrets(ctx, sd)
	if err != nil {
		return err
	}

	configMapMap, err := sdc.getConfigMaps(ctx, sd)
	if err != nil {
		return err
	}

	serviceAccounts, err := sdc.getServiceAccounts(ctx, sd)
	if err != nil {
		return fmt.Errorf("can't get serviceaccounts: %w", err)
	}

	roleBindings, err := sdc.getRoleBindings(ctx, sd)
	if err != nil {
		return fmt.Errorf("can't get rolebindings: %w", err)
	}

	pdbMap, err := sdc.getPDBs(ctx, sd)
	if err != nil {
		return err
	}

	ingressMap, err := sdc.getIngresses(ctx, sd)
	if err != nil {
		return fmt.Errorf("can't get ingresses: %w", err)
	}

	status := sdc.calculateStatus(sd, statefulSetMap, serviceMap)

	if sd.DeletionTimestamp != nil {
		return sdc.updateStatus(ctx, sd, status)
	}

	var errs []error

	err = runSync(
		&status.Conditions,
		serviceAccountControllerProgressingCondition,
		serviceAccountControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncServiceAccounts(ctx, sd, serviceAccounts)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync service accounts: %w", err))
	}

	err = runSync(
		&status.Conditions,
		roleBindingControllerProgressingCondition,
		roleBindingControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncRoleBindings(ctx, sd, roleBindings)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync role bindings: %w", err))
	}

	err = runSync(
		&status.Conditions,
		agentTokenControllerProgressingCondition,
		agentTokenControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncAgentToken(ctx, sd, secretMap)
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
			sd.Generation,
			func() ([]metav1.Condition, error) {
				return sdc.syncCerts(ctx, sd, secretMap, configMapMap, serviceMap)
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
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncStatefulSets(ctx, key, sd, status, statefulSetMap, serviceMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync stateful sets: %w", err))
	}
	// Ideally, this would be projected in calculateStatus but because we are updating the status based on applied
	// StatefulSets, the rack status can change afterwards. Overtime we should consider adding a status.progressing
	// field (to allow determining cluster status without conditions) and wait for the status to be updated
	// in a single place, on the next resync.
	sdc.setStatefulSetsAvailableStatusCondition(sd, status)

	err = runSync(
		&status.Conditions,
		serviceControllerProgressingCondition,
		serviceControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncServices(ctx, sd, status, serviceMap, statefulSetMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync services: %w", err))
	}

	err = runSync(
		&status.Conditions,
		pdbControllerProgressingCondition,
		pdbControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncPodDisruptionBudgets(ctx, sd, pdbMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync pdbs: %w", err))
	}

	err = runSync(
		&status.Conditions,
		ingressControllerProgressingCondition,
		ingressControllerDegradedCondition,
		sd.Generation,
		func() ([]metav1.Condition, error) {
			return sdc.syncIngresses(ctx, sd, ingressMap, serviceMap)
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
			ObservedGeneration: sd.Generation,
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
			ObservedGeneration: sd.Generation,
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
			ObservedGeneration: sd.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = sdc.updateStatus(ctx, sd, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}
