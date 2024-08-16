package scylladbmonitoring

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func getLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return labels.Set{
		naming.ScyllaDBMonitoringNameLabel: sm.Name,
	}
}

func getSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getLabels(sm))
}

func (smc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("can't split meta namespace cache key %q: %w", key, err)
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBMonitoring", "ScyllaDBMonitoring", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBMonitoring", "ScyllaDBMonitoring", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sm, err := smc.scylladbMonitoringLister.ScyllaDBMonitorings(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDBMonitoring has been deleted", "ScyllaDBMonitoring", klog.KObj(sm))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't get object %q from cache: %w", naming.ManualRef(namespace, name), err)
	}

	soc, err := smc.scyllaOperatorConfigLister.Get(naming.SingletonName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("can't get scyllaoperatorconfig %q from cache: %w", naming.SingletonName, err)
		}

		klog.V(4).InfoS("Waiting for ScyllaOperatorConfig to be available", "Name", naming.SingletonName)
		return nil
	}

	smSelector := getSelector(sm)

	type CT = *scyllav1alpha1.ScyllaDBMonitoring
	var objectErrs []error

	configMaps, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.configMapLister.ConfigMaps(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get config maps: %w", err))
	}

	secrets, err := controllerhelpers.GetObjects[CT, *corev1.Secret](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Secret]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.secretLister.Secrets(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().Secrets(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get secrets: %w", err))
	}

	services, err := controllerhelpers.GetObjects[CT, *corev1.Service](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Service]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceLister.Services(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().Services(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get services: %w", err))
	}

	serviceAccounts, err := controllerhelpers.GetObjects[CT, *corev1.ServiceAccount](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ServiceAccount]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceAccountLister.ServiceAccounts(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get service accounts: %w", err))
	}

	roleBindings, err := controllerhelpers.GetObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *rbacv1.RoleBinding]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.roleBindingLister.RoleBindings(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get role bindings: %w", err))
	}

	deployments, err := controllerhelpers.GetObjects[CT, *appsv1.Deployment](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *appsv1.Deployment]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.deploymentLister.Deployments(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.AppsV1().Deployments(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get deployments: %w", err))
	}

	ingresses, err := controllerhelpers.GetObjects[CT, *networkingv1.Ingress](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *networkingv1.Ingress]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.ingressLister.Ingresses(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get ingresses: %w", err))
	}

	prometheuses, err := controllerhelpers.GetObjects[CT, *monitoringv1.Prometheus](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.Prometheus]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.prometheusLister.Prometheuses(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.Prometheuses(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get prometheuses: %w", err))
	}

	prometheusRules, err := controllerhelpers.GetObjects[CT, *monitoringv1.PrometheusRule](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.PrometheusRule]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.prometheusRuleLister.PrometheusRules(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.PrometheusRules(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get prometheus rules: %w", err))
	}

	serviceMonitors, err := controllerhelpers.GetObjects[CT, *monitoringv1.ServiceMonitor](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.ServiceMonitor]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceMonitorLister.ServiceMonitors(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.ServiceMonitors(sm.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get service monitors: %w", err))
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	prometheusSelector := getPrometheusSelector(sm)
	grafanaSelector := getGrafanaSelector(sm)

	status := smc.calculateStatus(sm)

	if sm.DeletionTimestamp != nil {
		return smc.updateStatus(ctx, sm, status)
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		prometheusControllerProgressingCondition,
		prometheusControllerDegradedCondition,
		sm.Generation,
		func() ([]metav1.Condition, error) {
			return smc.syncPrometheus(
				ctx,
				sm,
				soc,
				controllerhelpers.FilterObjectMapByLabel(configMaps, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(secrets, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(services, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(serviceAccounts, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(roleBindings, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(ingresses, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(prometheuses, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(prometheusRules, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(serviceMonitors, prometheusSelector),
			)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync prometheus: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		grafanaControllerProgressingCondition,
		grafanaControllerDegradedCondition,
		sm.Generation,
		func() ([]metav1.Condition, error) {
			return smc.syncGrafana(
				ctx,
				sm,
				soc,
				controllerhelpers.FilterObjectMapByLabel(configMaps, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(secrets, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(services, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(serviceAccounts, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(deployments, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(ingresses, grafanaSelector),
			)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync grafana: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, sm.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = smc.updateStatus(ctx, sm, status)
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
