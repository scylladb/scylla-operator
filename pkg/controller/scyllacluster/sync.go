// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"slices"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scmc *Controller) sync(ctx context.Context, key string) error {
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

	sc, err := scmc.scyllaClusterLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	type CT = *scyllav1.ScyllaCluster
	var releaseErrs []error

	scSelector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	err = controllerhelpers.ReleaseObjects[CT, *appsv1.StatefulSet](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *appsv1.StatefulSet]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.statefulSetLister.StatefulSets(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.AppsV1().StatefulSets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *corev1.Service](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Service]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.serviceLister.Services(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.CoreV1().Services(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *corev1.Secret](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Secret]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.secretLister.Secrets(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.CoreV1().Secrets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *corev1.ConfigMap](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.configMapLister.ConfigMaps(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.CoreV1().ConfigMaps(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *corev1.ServiceAccount](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ServiceAccount]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.serviceAccountLister.ServiceAccounts(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.CoreV1().ServiceAccounts(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *rbacv1.RoleBinding]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.roleBindingLister.RoleBindings(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.RbacV1().RoleBindings(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *policyv1.PodDisruptionBudget](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *policyv1.PodDisruptionBudget]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.pdbLister.PodDisruptionBudgets(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.PolicyV1().PodDisruptionBudgets(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *networkingv1.Ingress](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *networkingv1.Ingress]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.ingressLister.Ingresses(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.NetworkingV1().Ingresses(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	err = controllerhelpers.ReleaseObjects[CT, *batchv1.Job](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *batchv1.Job]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.jobLister.Jobs(sc.Namespace).List,
			PatchObjectFunc:           scmc.kubeClient.BatchV1().Jobs(sc.Namespace).Patch,
		},
	)
	if err != nil {
		releaseErrs = append(releaseErrs, err)
	}

	releaseErr := apimachineryutilerrors.NewAggregate(releaseErrs)
	if releaseErr != nil {
		return releaseErr
	}

	var objectErrs []error

	scyllaDBDatacenterMap, err := controllerhelpers.GetCustomResourceObjects[CT, *scyllav1alpha1.ScyllaDBDatacenter](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *scyllav1alpha1.ScyllaDBDatacenter]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.scyllaDBDatacenterLister.ScyllaDBDatacenters(sc.Namespace).List,
			PatchObjectFunc:           scmc.scyllaClient.ScyllaV1alpha1().ScyllaDBDatacenters(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	scyllaDBManagerTaskMap, err := controllerhelpers.GetCustomResourceObjects[CT, *scyllav1alpha1.ScyllaDBManagerTask](
		ctx,
		sc,
		scyllaClusterControllerGVK,
		scSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *scyllav1alpha1.ScyllaDBManagerTask]{
			GetControllerUncachedFunc: scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Get,
			ListObjectsFunc:           scmc.scyllaDBManagerTaskLister.ScyllaDBManagerTasks(sc.Namespace).List,
			PatchObjectFunc:           scmc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerTasks(sc.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get ScyllaDBManagerTasks: %w", err))
	}

	// List objects matching our cluster selector and owned either by ScyllaCluster or already migrated ScyllaDBDatacenter
	configMaps, err := scmc.configMapLister.ConfigMaps(sc.Namespace).List(labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)))
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't list ConfigMaps: %w", err))
	}

	services, err := scmc.serviceLister.Services(sc.Namespace).List(labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)))
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't list Services: %w", err))
	}

	allowedOwnerUIDs := []types.UID{
		sc.UID,
	}

	if sdc, ok := scyllaDBDatacenterMap[sc.Name]; ok {
		allowedOwnerUIDs = append(allowedOwnerUIDs, sdc.UID)
	}

	configMaps = oslices.Filter(configMaps, isOwnedByAnyFunc[*corev1.ConfigMap](allowedOwnerUIDs))
	services = oslices.Filter(services, isOwnedByAnyFunc[*corev1.Service](allowedOwnerUIDs))

	// ScyllaDBManagerClusterRegistrations are not owned by ScyllaCluster or ScyllaDBDatacenter, so we list all.
	scyllaDBManagerClusterRegistrations, err := scmc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(sc.Namespace).List(labels.Everything())
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't list ScyllaDBManagerClusterRegistrations: %w", err))
	}
	scyllaDBManagerClusterRegistrations, err = filterScyllaDBManagerClusterRegistrations(sc, scyllaDBDatacenterMap, scyllaDBManagerClusterRegistrations)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't filter ScyllaDBManagerClusterRegistrations: %w", err))
	}

	objectErr := apimachineryutilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	status := scmc.calculateStatus(sc, scyllaDBDatacenterMap, configMaps, services, scyllaDBManagerClusterRegistrations, scyllaDBManagerTaskMap)

	if sc.DeletionTimestamp != nil {
		return scmc.updateStatus(ctx, sc, status)
	}

	// Some SC spec changes don't bump SDC's generation (e.g. spec fields translated to annotations),
	// and users can also directly bump SDC's generation without touching SC.
	// Because of that, we shift SDC conditions' ObservedGeneration into SC's generation space.
	var sdcConditions []metav1.Condition
	if sdc, ok := scyllaDBDatacenterMap[sc.Name]; ok {
		sdcConditions = offsetSDCConditionsToSCGenerationSpace(sc.Generation, sdc.Generation, sdc.Status.Conditions)
	}

	var errs []error
	var conditions []metav1.Condition

	err = controllerhelpers.RunSync(
		&conditions,
		scyllaDBDatacenterControllerProgressingCondition,
		scyllaDBDatacenterControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scmc.syncScyllaDBDatacenter(ctx, sc, scyllaDBDatacenterMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ScyllaDBDatacenter: %w", err))
	}

	err = controllerhelpers.RunSync(
		&conditions,
		scyllaDBManagerTaskControllerProgressingCondition,
		scyllaDBManagerTaskControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scmc.syncScyllaDBManagerTasks(ctx, sc, scyllaDBManagerTaskMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ScyllaDBManagerTasks: %w", err))
	}

	var aggregationErrs []error

	// Aggregate the conditions from two independent sources:
	// - Controller partial conditions produced by RunSync above (names end with the condition suffix).
	// - The matching aggregate condition reported by the ScyllaDBDatacenter, or an Unknown condition
	//   if the SDC has not yet reported a matching condition for the current generation.

	progressingConditionInputs := append(
		controllerhelpers.FindStatusConditionsWithSuffix(conditions, scyllav1alpha1.ProgressingCondition),
		findStatusConditionOrUnknown(sdcConditions, scyllav1alpha1.ProgressingCondition, sc.Generation),
	)
	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		progressingConditionInputs,
		metav1.Condition{
			Type:               scyllav1alpha1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: sc.Generation,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing conditions: %w", err))
	}

	degradedConditionInputs := append(
		controllerhelpers.FindStatusConditionsWithSuffix(conditions, scyllav1alpha1.DegradedCondition),
		findStatusConditionOrUnknown(sdcConditions, scyllav1alpha1.DegradedCondition, sc.Generation),
	)
	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		degradedConditionInputs,
		metav1.Condition{
			Type:               scyllav1alpha1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: sc.Generation,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return apimachineryutilerrors.NewAggregate(errs)
	}

	// Merge the raw SDC conditions into the final set first, then overwrite Progressing and Degraded with the aggregated values computed above.
	// Available is intentionally left as reported by the SDC — no controller partial affects it.
	conditions = append(conditions, sdcConditions...)
	apimeta.SetStatusCondition(&conditions, progressingCondition)
	apimeta.SetStatusCondition(&conditions, degradedCondition)

	status.Conditions = conditions

	err = scmc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return apimachineryutilerrors.NewAggregate(errs)
}

func filterScyllaDBManagerClusterRegistrations(
	sc *scyllav1.ScyllaCluster,
	scyllaDBDatacenterMap map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	scyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration,
) ([]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	var smcrs []*scyllav1alpha1.ScyllaDBManagerClusterRegistration

	sdc, ok := scyllaDBDatacenterMap[sc.Name]
	if !ok {
		return smcrs, nil
	}

	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		return smcrs, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	return oslices.Filter(scyllaDBManagerClusterRegistrations, func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) bool {
		return smcr.Name == smcrName
	}), nil
}

// offsetSDCConditionsToSCGenerationSpace shifts the ObservedGeneration of each SDC condition
// into SC's generation space. The offset is computed as scGeneration - sdcGeneration and applied to each
// condition's ObservedGeneration. The result is clamped to 0 to satisfy the CRD's minimum: 0
// constraint — a condition whose shifted ObservedGeneration would be negative is from a
// generation too old to be relevant and won't match scGeneration.
func offsetSDCConditionsToSCGenerationSpace(scGeneration, sdcGeneration int64, conditions []metav1.Condition) []metav1.Condition {
	result := slices.Clone(conditions)
	generationOffset := scGeneration - sdcGeneration
	for i := range result {
		result[i].ObservedGeneration = max(0, result[i].ObservedGeneration+generationOffset)
	}
	return result
}

// findStatusConditionOrUnknown returns the condition of the given type whose ObservedGeneration matches generation,
// or a synthetic Unknown condition when no such condition exists.
func findStatusConditionOrUnknown(conditions []metav1.Condition, conditionType string, generation int64) metav1.Condition {
	i := slices.IndexFunc(conditions, func(c metav1.Condition) bool {
		return c.Type == conditionType && c.ObservedGeneration == generation
	})
	if i < 0 {
		return metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: generation,
			Reason:             internalapi.AwaitingConditionReason,
			Message:            "",
		}
	}

	return conditions[i]
}
