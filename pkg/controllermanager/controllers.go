// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1listers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/globalscylladbmanager"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagerclusterregistration"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagertask"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/klog/v2"
)

// registerControllers wires every controller of the operator binary. The controllers not migrated to
// controller-runtime yet get their informers from the manager's cache through the Informer bridge, so there is
// one informer per kind in the process regardless of which world a controller lives in.
func (m *Manager) registerControllers(ctx context.Context) error {
	o := m.options
	c := m.mgr.GetCache()
	errs := &errorCollector{}

	pods := informerFor(ctx, c, errs, &corev1.Pod{}, corev1listers.NewPodLister)
	services := informerFor(ctx, c, errs, &corev1.Service{}, corev1listers.NewServiceLister)
	secrets := informerFor(ctx, c, errs, &corev1.Secret{}, corev1listers.NewSecretLister)
	configMaps := informerFor(ctx, c, errs, &corev1.ConfigMap{}, corev1listers.NewConfigMapLister)
	serviceAccounts := informerFor(ctx, c, errs, &corev1.ServiceAccount{}, corev1listers.NewServiceAccountLister)
	namespaces := informerFor(ctx, c, errs, &corev1.Namespace{}, corev1listers.NewNamespaceLister)
	nodes := informerFor(ctx, c, errs, &corev1.Node{}, corev1listers.NewNodeLister)
	persistentVolumes := informerFor(ctx, c, errs, &corev1.PersistentVolume{}, corev1listers.NewPersistentVolumeLister)
	persistentVolumeClaims := informerFor(ctx, c, errs, &corev1.PersistentVolumeClaim{}, corev1listers.NewPersistentVolumeClaimLister)
	roles := informerFor(ctx, c, errs, &rbacv1.Role{}, rbacv1listers.NewRoleLister)
	roleBindings := informerFor(ctx, c, errs, &rbacv1.RoleBinding{}, rbacv1listers.NewRoleBindingLister)
	clusterRoles := informerFor(ctx, c, errs, &rbacv1.ClusterRole{}, rbacv1listers.NewClusterRoleLister)
	clusterRoleBindings := informerFor(ctx, c, errs, &rbacv1.ClusterRoleBinding{}, rbacv1listers.NewClusterRoleBindingLister)
	statefulSets := informerFor(ctx, c, errs, &appsv1.StatefulSet{}, appsv1listers.NewStatefulSetLister)
	deployments := informerFor(ctx, c, errs, &appsv1.Deployment{}, appsv1listers.NewDeploymentLister)
	daemonSets := informerFor(ctx, c, errs, &appsv1.DaemonSet{}, appsv1listers.NewDaemonSetLister)
	podDisruptionBudgets := informerFor(ctx, c, errs, &policyv1.PodDisruptionBudget{}, policyv1listers.NewPodDisruptionBudgetLister)
	ingresses := informerFor(ctx, c, errs, &networkingv1.Ingress{}, networkingv1listers.NewIngressLister)
	jobs := informerFor(ctx, c, errs, &batchv1.Job{}, batchv1listers.NewJobLister)

	scyllaClusters := informerFor(ctx, c, errs, &scyllav1.ScyllaCluster{}, scyllav1listers.NewScyllaClusterLister)
	scyllaDBDatacenters := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBDatacenter{}, scyllav1alpha1listers.NewScyllaDBDatacenterLister)
	scyllaDBDatacenterNodesStatusReports := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}, scyllav1alpha1listers.NewScyllaDBDatacenterNodesStatusReportLister)
	scyllaDBMonitorings := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBMonitoring{}, scyllav1alpha1listers.NewScyllaDBMonitoringLister)
	scyllaDBManagerClusterRegistrations := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBManagerClusterRegistration{}, scyllav1alpha1listers.NewScyllaDBManagerClusterRegistrationLister)
	scyllaDBManagerTasks := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaDBManagerTask{}, scyllav1alpha1listers.NewScyllaDBManagerTaskLister)
	nodeConfigs := informerFor(ctx, c, errs, &scyllav1alpha1.NodeConfig{}, scyllav1alpha1listers.NewNodeConfigLister)
	// The cache filters ScyllaOperatorConfigs to the singleton; see the manager's cache options.
	scyllaOperatorConfigs := informerFor(ctx, c, errs, &scyllav1alpha1.ScyllaOperatorConfig{}, scyllav1alpha1listers.NewScyllaOperatorConfigLister)

	err := errs.Err()
	if err != nil {
		return fmt.Errorf("can't get informers: %w", err)
	}

	sdcc, err := scylladbdatacenter.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		pods,
		services,
		secrets,
		configMaps,
		serviceAccounts,
		roleBindings,
		statefulSets,
		podDisruptionBudgets,
		ingresses,
		jobs,
		scyllaDBDatacenters,
		scyllaDBDatacenterNodesStatusReports,
		scyllaOperatorConfigs,
		o.OperatorImage,
		o.CQLSIngressPort,
		o.KeyGenerator,
	)
	if err != nil {
		return fmt.Errorf("can't create scylladbdatacenter controller: %w", err)
	}
	m.addRunnable(sdcc.Run, o.ConcurrentSyncs)

	scc, err := scyllacluster.NewController(
		o.KubeClient,
		o.ScyllaClient,
		services,
		secrets,
		configMaps,
		serviceAccounts,
		roleBindings,
		statefulSets,
		podDisruptionBudgets,
		ingresses,
		jobs,
		scyllaClusters,
		scyllaDBDatacenters,
		scyllaDBManagerClusterRegistrations,
		scyllaDBManagerTasks,
	)
	if err != nil {
		return fmt.Errorf("can't create scyllacluster controller: %w", err)
	}
	m.addRunnable(scc.Run, o.ConcurrentSyncs)

	opc, err := orphanedpv.NewController(
		o.KubeClient,
		persistentVolumes,
		persistentVolumeClaims,
		nodes,
		scyllaDBDatacenters,
	)
	if err != nil {
		return fmt.Errorf("can't create orphanpv controller: %w", err)
	}
	m.addRunnable(opc.Run, o.ConcurrentSyncs)

	ncc, err := nodeconfig.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		nodeConfigs,
		scyllaOperatorConfigs,
		clusterRoles,
		clusterRoleBindings,
		roles,
		roleBindings,
		daemonSets,
		namespaces,
		nodes,
		serviceAccounts,
		configMaps,
		o.OperatorImage,
	)
	if err != nil {
		return fmt.Errorf("can't create nodeconfig controller: %w", err)
	}
	m.addRunnable(ncc.Run, o.ConcurrentSyncs)

	ncpc, err := nodeconfigpod.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		pods,
		configMaps,
		nodes,
		nodeConfigs,
	)
	if err != nil {
		return fmt.Errorf("can't create nodeconfigpod controller: %w", err)
	}
	m.addRunnable(ncpc.Run, o.ConcurrentSyncs)

	socc, err := scyllaoperatorconfig.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		scyllaOperatorConfigs,
		o.ClusterDomainGetter,
	)
	if err != nil {
		return fmt.Errorf("can't create scyllaoperatorconfig controller: %w", err)
	}
	m.addRunnable(socc.Run, o.ConcurrentSyncs)

	// The ScyllaDBMonitoring controller watches Prometheus Operator kinds, so it can only run where their CRDs are.
	monitoringCRDsInstalled, err := helpers.IsAPIGroupVersionAvailable(o.KubeClient.Discovery(), "monitoring.coreos.com/v1")
	if err != nil {
		return fmt.Errorf("can't check if monitoring API group version is available: %w", err)
	}
	if !monitoringCRDsInstalled {
		klog.InfoS("Prometheus Operator CRDs (monitoring.coreos.com) are not installed in the cluster. " +
			"ScyllaDBMonitoring controller will not be started. " +
			"To enable monitoring, install Prometheus Operator and restart the ScyllaDB Operator.")
	} else {
		prometheuses := informerFor(ctx, c, errs, &monitoringv1.Prometheus{}, monitoringv1listers.NewPrometheusLister)
		prometheusRules := informerFor(ctx, c, errs, &monitoringv1.PrometheusRule{}, monitoringv1listers.NewPrometheusRuleLister)
		serviceMonitors := informerFor(ctx, c, errs, &monitoringv1.ServiceMonitor{}, monitoringv1listers.NewServiceMonitorLister)
		err = errs.Err()
		if err != nil {
			return fmt.Errorf("can't get monitoring informers: %w", err)
		}

		mc, err := scylladbmonitoring.NewController(
			o.KubeClient,
			o.ScyllaClient.ScyllaV1alpha1(),
			o.MonitoringClient.MonitoringV1(),
			scyllaOperatorConfigs,
			configMaps,
			secrets,
			services,
			serviceAccounts,
			roleBindings,
			podDisruptionBudgets,
			deployments,
			ingresses,
			scyllaDBMonitorings,
			prometheuses,
			prometheusRules,
			serviceMonitors,
			o.KeyGenerator,
		)
		if err != nil {
			return fmt.Errorf("can't create scylladbmonitoring controller: %w", err)
		}
		m.addRunnable(mc.Run, o.ConcurrentSyncs)
	}

	gsmc, err := globalscylladbmanager.NewController(
		o.KubeClient,
		o.ScyllaClient,
		scyllaDBManagerClusterRegistrations,
		scyllaDBDatacenters,
		namespaces,
	)
	if err != nil {
		return fmt.Errorf("can't create global ScyllaDB Manager controller: %w", err)
	}
	m.runnables = append(m.runnables, gsmc.Run)

	smcrc, err := scylladbmanagerclusterregistration.NewController(
		o.KubeClient,
		o.ScyllaClient,
		scyllaDBManagerClusterRegistrations,
		scyllaDBDatacenters,
		secrets,
		namespaces,
	)
	if err != nil {
		return fmt.Errorf("can't create ScyllaDBManagerClusterRegistration controller: %w", err)
	}
	m.addRunnable(smcrc.Run, o.ConcurrentSyncs)

	smtc, err := scylladbmanagertask.NewController(
		o.KubeClient,
		o.ScyllaClient.ScyllaV1alpha1(),
		scyllaDBManagerTasks,
		scyllaDBManagerClusterRegistrations,
	)
	if err != nil {
		return fmt.Errorf("can't create ScyllaDBManagerTask controller: %w", err)
	}
	m.addRunnable(smtc.Run, o.ConcurrentSyncs)

	return nil
}

func (m *Manager) addRunnable(run func(ctx context.Context, workers int), workers int) {
	m.runnables = append(m.runnables, func(ctx context.Context) {
		run(ctx, workers)
	})
}
