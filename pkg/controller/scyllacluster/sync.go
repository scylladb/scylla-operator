package scyllacluster

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
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

	scSelector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	type CT = *scyllav1.ScyllaCluster
	var objectErrs []error

	statefulSetMap, err := controllerhelpers.GetObjects[CT, *appsv1.StatefulSet](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *appsv1.StatefulSet]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.statefulSetLister.StatefulSets(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.AppsV1().StatefulSets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	serviceMap, err := controllerhelpers.GetObjects[CT, *corev1.Service](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Service]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.serviceLister.Services(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().Services(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	secretMap, err := controllerhelpers.GetObjects[CT, *corev1.Secret](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Secret]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.secretLister.Secrets(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().Secrets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	configMapMap, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.configMapLister.ConfigMaps(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().ConfigMaps(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	serviceAccounts, err := controllerhelpers.GetObjects[CT, *corev1.ServiceAccount](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ServiceAccount]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.serviceAccountLister.ServiceAccounts(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().ServiceAccounts(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	roleBindings, err := controllerhelpers.GetObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *rbacv1.RoleBinding]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.roleBindingLister.RoleBindings(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.RbacV1().RoleBindings(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	pdbMap, err := controllerhelpers.GetObjects[CT, *policyv1.PodDisruptionBudget](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *policyv1.PodDisruptionBudget]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.pdbLister.PodDisruptionBudgets(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.PolicyV1().PodDisruptionBudgets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	ingressMap, err := controllerhelpers.GetObjects[CT, *networkingv1.Ingress](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *networkingv1.Ingress]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.ingressLister.Ingresses(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.NetworkingV1().Ingresses(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
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
