// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

	matchingNodes, err := ncc.getMatchingNodes(nc)
	if err != nil {
		return fmt.Errorf("can't get matching Nodes: %w", err)
	}

	ncSelector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: nc.Name,
	})

	type CT = *scyllav1alpha1.NodeConfig
	var objectErrs []error

	namespaces, err := ncc.getNamespaces()
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	clusterRoles, err := ncc.getClusterRoles()
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	serviceAccounts, err := ncc.getServiceAccounts()
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	clusterRoleBindings, err := ncc.getClusterRoleBindings()
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	daemonSets, err := controllerhelpers.GetObjects[CT, *appsv1.DaemonSet](
		ctx,
		nc,
		nodeConfigControllerGVK,
		ncSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *appsv1.DaemonSet]{
			GetControllerUncachedFunc: ncc.scyllaClient.NodeConfigs().Get,
			ListObjectsFunc:           ncc.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List,
			PatchObjectFunc:           ncc.kubeClient.AppsV1().DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	status, err := ncc.calculateStatus(nc)
	if err != nil {
		return fmt.Errorf("can't calculate status: %w", err)
	}

	if nc.DeletionTimestamp != nil {
		return ncc.updateStatus(ctx, nc, status)
	}

	var errs []error
	err = controllerhelpers.RunSync(
		&status.Conditions,
		namespaceControllerProgressingCondition,
		namespaceControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncNamespaces(ctx, nc, namespaces)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync Namespace(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		clusterRoleControllerProgressingCondition,
		clusterRoleControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncClusterRoles(ctx, nc, clusterRoles)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ClusterRole(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		serviceAccountControllerProgressingCondition,
		serviceAccountControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncServiceAccounts(ctx, nc, serviceAccounts)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ServiceAccount(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		clusterRoleBindingControllerProgressingCondition,
		clusterRoleBindingControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncClusterRoleBindings(ctx, nc, clusterRoleBindings)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ClusterRoleBinding(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		daemonSetControllerProgressingCondition,
		daemonSetControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncDaemonSet(ctx, nc, soc, daemonSets)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync DaemonSet(s): %w", err))
	}

	// Aggregate conditions.
	var aggregationErrs []error

	availableConditions := controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.AvailableCondition)

	// Use a default available node condition for nodes which haven't propagated their conditions to status yet.
	for _, n := range matchingNodes {
		nodeAvailableConditionType := fmt.Sprintf(internalapi.NodeAvailableConditionFormat, controllerhelpers.DNS1123SubdomainToValidStatusConditionReason(n.Name))
		nodeAvailableStatusCondition := apimeta.FindStatusCondition(availableConditions, nodeAvailableConditionType)
		if nodeAvailableStatusCondition == nil || nodeAvailableStatusCondition.ObservedGeneration < nc.Generation {
			availableConditions = append(availableConditions, metav1.Condition{
				Type:               nodeAvailableConditionType,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting available condition of node %q to be set.", naming.ObjRef(n)),
			})
		}
	}
	availableCondition, err := controllerhelpers.AggregateStatusConditions(
		availableConditions,
		metav1.Condition{
			Type:               scyllav1alpha1.AvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate available status conditions: %w", err))
	}

	// Use a default progressing node condition for nodes which haven't propagated their conditions to status yet.
	progressingConditions := controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.ProgressingCondition)
	for _, n := range matchingNodes {
		nodeProgressingConditionType := fmt.Sprintf(internalapi.NodeProgressingConditionFormat, controllerhelpers.DNS1123SubdomainToValidStatusConditionReason(n.Name))
		nodeProgressingStatusCondition := apimeta.FindStatusCondition(progressingConditions, nodeProgressingConditionType)
		if nodeProgressingStatusCondition == nil || nodeProgressingStatusCondition.ObservedGeneration < nc.Generation {
			availableConditions = append(availableConditions, metav1.Condition{
				Type:               nodeProgressingConditionType,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting progressing condition of node %q to be set.", naming.ObjRef(n)),
			})
		}
	}
	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		progressingConditions,
		metav1.Condition{
			Type:               scyllav1alpha1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing status conditions: %w", err))
	}

	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.DegradedCondition),
		metav1.Condition{
			Type:               scyllav1alpha1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded status conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return utilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&status.Conditions, availableCondition)
	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = ncc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
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

func (ncc *Controller) getMatchingNodes(nc *scyllav1alpha1.NodeConfig) ([]*corev1.Node, error) {
	nodes, err := ncc.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list Nodes: %w", err)
	}

	var errs []error
	var matchingNodes []*corev1.Node
	for _, n := range nodes {
		isNodeConfigSelectingNode, err := controllerhelpers.IsNodeConfigSelectingNode(nc, n)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't determine if NodeConfig %q is selecting Node %q: %w", naming.ObjRef(nc), naming.ObjRef(n), err))
		}

		if isNodeConfigSelectingNode {
			matchingNodes = append(matchingNodes, n)
		}
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return matchingNodes, nil
}
