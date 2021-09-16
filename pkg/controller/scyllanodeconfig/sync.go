// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
)

func (sncc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("split meta namespace cache key: %w", err)
	}

	snc, err := sncc.scyllaNodeConfigLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) && name == resource.DefaultScyllaNodeConfig().Name {
			snc, err = sncc.scyllaClient.ScyllaNodeConfigs().Create(ctx, resource.DefaultScyllaNodeConfig(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create default ScyllaNodeConfig: %w", err)
			}
			return nil
		} else {
			return fmt.Errorf("get ScyllaNodeConfig %q: %w", name, err)
		}
	}

	scyllaPods, err := sncc.podLister.Pods(corev1.NamespaceAll).List(naming.ScyllaSelector())
	if err != nil {
		return fmt.Errorf("get Scylla Pods: %w", err)
	}

	namespaces, err := sncc.getNamespaces()
	if err != nil {
		return fmt.Errorf("get Namespaces: %w", err)
	}

	clusterRoles, err := sncc.getClusterRoles()
	if err != nil {
		return fmt.Errorf("get ClusterRoles: %w", err)
	}

	serviceAccounts, err := sncc.getServiceAccounts()
	if err != nil {
		return fmt.Errorf("get ServiceAccounts: %w", err)
	}

	clusterRoleBindings, err := sncc.getClusterRoleBindings()
	if err != nil {
		return fmt.Errorf("get ClusterRoleBindings: %w", err)
	}

	daemonSets, err := sncc.getDaemonSets(ctx, snc)
	if err != nil {
		return fmt.Errorf("get DaemonSets: %w", err)
	}

	jobs, err := sncc.getJobs(ctx, snc)
	if err != nil {
		return fmt.Errorf("get Jobs: %w", err)
	}

	configMaps, err := sncc.getConfigMaps(ctx, snc, scyllaPods)
	if err != nil {
		return fmt.Errorf("get ConfigMaps: %w", err)
	}

	status := sncc.calculateStatus(snc, daemonSets)

	if snc.DeletionTimestamp != nil {
		return sncc.updateStatus(ctx, snc, status)
	}

	var errs []error

	if err := sncc.syncNamespaces(ctx, namespaces); err != nil {
		errs = append(errs, fmt.Errorf("sync Namespace(s): %w", err))
	}

	if err := sncc.syncClusterRoles(ctx, clusterRoles); err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRole(s): %w", err))
	}

	if err := sncc.syncServiceAccounts(ctx, serviceAccounts); err != nil {
		errs = append(errs, fmt.Errorf("sync ServiceAccount(s): %w", err))
	}

	if err := sncc.syncClusterRoleBindings(ctx, clusterRoleBindings); err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRoleBinding(s): %w", err))
	}

	if err := sncc.syncDaemonSets(ctx, snc, status, daemonSets); err != nil {
		errs = append(errs, fmt.Errorf("sync DaemonSet(s): %w", err))
	}

	if err := sncc.syncConfigMaps(ctx, snc, scyllaPods, configMaps, jobs); err != nil {
		errs = append(errs, fmt.Errorf("sync ConfigMap(s): %w", err))
	}

	err = sncc.updateStatus(ctx, snc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func (sncc *Controller) getDaemonSets(ctx context.Context, snc *scyllav1alpha1.ScyllaNodeConfig) (map[string]*appsv1.DaemonSet, error) {
	dss, err := sncc.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list daemonsets: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := sncc.scyllaClient.ScyllaNodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original ScyllaNodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewDaemonSetControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealDaemonSetControl{
			KubeClient: sncc.kubeClient,
			Recorder:   sncc.eventRecorder,
		},
	)
	return cm.ClaimDaemonSets(dss)
}

func (sncc *Controller) getNamespaces() (map[string]*corev1.Namespace, error) {
	nss, err := sncc.namespaceLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	nsMap := map[string]*corev1.Namespace{}
	for i := range nss {
		nsMap[nss[i].Name] = nss[i]
	}

	return nsMap, nil
}

func (sncc *Controller) getClusterRoles() (map[string]*rbacv1.ClusterRole, error) {
	crs, err := sncc.clusterRoleLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list clusterroles: %w", err)
	}

	crMap := map[string]*rbacv1.ClusterRole{}
	for i := range crs {
		crMap[crs[i].Name] = crs[i]
	}

	return crMap, nil
}

func (sncc *Controller) getClusterRoleBindings() (map[string]*rbacv1.ClusterRoleBinding, error) {
	crbs, err := sncc.clusterRoleBindingLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list clusterrolebindings: %w", err)
	}

	crbMap := map[string]*rbacv1.ClusterRoleBinding{}
	for i := range crbs {
		crbMap[crbs[i].Name] = crbs[i]
	}

	return crbMap, nil
}

func (sncc *Controller) getServiceAccounts() (map[string]*corev1.ServiceAccount, error) {
	sas, err := sncc.serviceAccountLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list serviceaccounts: %w", err)
	}

	sasMap := map[string]*corev1.ServiceAccount{}
	for i := range sas {
		sasMap[sas[i].Name] = sas[i]
	}

	return sasMap, nil
}

func (sncc *Controller) getJobs(ctx context.Context, snc *scyllav1alpha1.ScyllaNodeConfig) (map[string]*batchv1.Job, error) {
	jobs, err := sncc.jobLister.Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := sncc.scyllaClient.ScyllaNodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original ScyllaNodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewJobControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealJobControl{
			KubeClient: sncc.kubeClient,
			Recorder:   sncc.eventRecorder,
		},
	)
	return cm.ClaimJobs(jobs)
}

func (sncc *Controller) getConfigMaps(ctx context.Context, snc *scyllav1alpha1.ScyllaNodeConfig, scyllaPods []*corev1.Pod) (map[string]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap

	namespaces := map[string]struct{}{}
	for _, scyllaPod := range scyllaPods {
		namespaces[scyllaPod.Namespace] = struct{}{}
	}

	for ns := range namespaces {
		cms, err := sncc.configMapLister.ConfigMaps(ns).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("list configmaps: %w", err)
		}
		configMaps = append(configMaps, cms...)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := sncc.scyllaClient.ScyllaNodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original ScyllaNodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: sncc.kubeClient,
			Recorder:   sncc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(configMaps)
}
