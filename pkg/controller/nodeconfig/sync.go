// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (ncc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("can't split meta namespace cache key: %w", err)
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing NodeConfig", "NodeConfig", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing NodeConfig", "NodeConfig", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	nc, err := ncc.nodeConfigLister.Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("NodeConfig has been deleted", "NodeConfig", klog.KObj(nc))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't get NodeConfig %q: %w", key, err)
	}

	soc, err := ncc.scyllaOperatorConfigLister.Get(naming.SingletonName)
	if err != nil {
		return fmt.Errorf("can't get ScyllaOperatorConfig: %w", err)
	}

	namespaces, err := ncc.getNamespaces()
	if err != nil {
		return fmt.Errorf("get Namespaces: %w", err)
	}

	clusterRoles, err := ncc.getClusterRoles()
	if err != nil {
		return fmt.Errorf("get ClusterRoles: %w", err)
	}

	serviceAccounts, err := ncc.getServiceAccounts()
	if err != nil {
		return fmt.Errorf("get ServiceAccounts: %w", err)
	}

	clusterRoleBindings, err := ncc.getClusterRoleBindings()
	if err != nil {
		return fmt.Errorf("get ClusterRoleBindings: %w", err)
	}

	daemonSets, err := ncc.getDaemonSets(ctx, nc)
	if err != nil {
		return fmt.Errorf("get DaemonSets: %w", err)
	}

	status, err := ncc.calculateStatus(nc, daemonSets, ncc.operatorImage)
	if err != nil {
		return fmt.Errorf("can't calculate status: %w", err)
	}

	if nc.DeletionTimestamp != nil {
		return ncc.updateStatus(ctx, nc, status)
	}

	var errs []error

	err = ncc.syncNamespaces(ctx, namespaces)
	if err != nil {
		errs = append(errs, fmt.Errorf("sync Namespace(s): %w", err))
	}

	err = ncc.syncClusterRoles(ctx, clusterRoles)
	if err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRole(s): %w", err))
	}

	err = ncc.syncServiceAccounts(ctx, serviceAccounts)
	if err != nil {
		errs = append(errs, fmt.Errorf("sync ServiceAccount(s): %w", err))
	}

	err = ncc.syncClusterRoleBindings(ctx, clusterRoleBindings)
	if err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRoleBinding(s): %w", err))
	}

	err = ncc.syncDaemonSet(ctx, nc, soc, status, daemonSets)
	if err != nil {
		errs = append(errs, fmt.Errorf("sync DaemonSet(s): %w", err))
	}

	err = ncc.updateStatus(ctx, nc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) getDaemonSets(ctx context.Context, nc *scyllav1alpha1.NodeConfig) (map[string]*appsv1.DaemonSet, error) {
	dss, err := ncc.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list daemonsets: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: nc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := ncc.scyllaClient.NodeConfigs().Get(ctx, nc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != nc.UID {
			return fmt.Errorf("original NodeConfig %q is gone: got uid %v, wanted %v", nc.Name, fresh.UID, nc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", nc.Name, nc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewDaemonSetControllerRefManager(
		ctx,
		nc,
		nodeConfigControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealDaemonSetControl{
			KubeClient: ncc.kubeClient,
			Recorder:   ncc.eventRecorder,
		},
	)
	return cm.ClaimDaemonSets(dss)
}

func (ncc *Controller) getNamespaces() (map[string]*corev1.Namespace, error) {
	nss, err := ncc.namespaceLister.List(labels.SelectorFromSet(map[string]string{
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

func (ncc *Controller) getClusterRoles() (map[string]*rbacv1.ClusterRole, error) {
	crs, err := ncc.clusterRoleLister.List(labels.SelectorFromSet(map[string]string{
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

func (ncc *Controller) getClusterRoleBindings() (map[string]*rbacv1.ClusterRoleBinding, error) {
	crbs, err := ncc.clusterRoleBindingLister.List(labels.SelectorFromSet(map[string]string{
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

func (ncc *Controller) getServiceAccounts() (map[string]*corev1.ServiceAccount, error) {
	sas, err := ncc.serviceAccountLister.List(labels.SelectorFromSet(map[string]string{
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
