// Copyright (c) 2024 ScyllaDB.

package scyllaclustermigration

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	klog.V(4).InfoS("Started migrating ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished migrating ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
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
	var objectErrs []error

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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
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
		objectErrs = append(objectErrs, err)
	}

	sdcMap, err := controllerhelpers.GetObjects[CT, *scyllav1alpha1.ScyllaDBDatacenter](
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

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	sdc, err := scmc.scyllaDBDatacenterLister.ScyllaDBDatacenters(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	status := scmc.calculateStatus(sc, sdc)

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
			return scmc.syncScyllaDBDatacenter(ctx, sc, sdcMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ingresses: %w", err))
	}

	err = scmc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}
