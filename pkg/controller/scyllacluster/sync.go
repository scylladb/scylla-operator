// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (scmc *Controller) sync(ctx context.Context, key types.NamespacedName) error {
	namespace, name := key.Namespace, key.Name

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

	objectErr := apimachineryutilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	status := scmc.calculateStatus(sc, scyllaDBDatacenterMap, configMaps, services)

	if sc.DeletionTimestamp != nil {
		return scmc.updateStatus(ctx, sc, status)
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		scyllaDBDatacenterControllerProgressingCondition,
		scyllaDBDatacenterControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scmc.syncScyllaDBDatacenter(ctx, sc, scyllaDBDatacenterMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync scylladbdatacenter: %w", err))
	}

	err = scmc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return apimachineryutilerrors.NewAggregate(errs)
}
