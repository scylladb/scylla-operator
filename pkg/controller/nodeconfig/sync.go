// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncc *Controller) sync(ctx context.Context, key types.NamespacedName) error {
	namespace, name := key.Namespace, key.Name

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

	roles, err := ncc.getRoles()
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

	roleBindings, err := ncc.getRoleBindings()
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

	objectErr := apimachineryutilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	matchingNodes, err := ncc.getMatchingNodes(nc)
	if err != nil {
		return fmt.Errorf("can't get matching Nodes: %w", err)
	}

	status := ncc.calculateStatus(nc, matchingNodes)

	if nc.DeletionTimestamp != nil {
		return ncc.updateStatus(ctx, nc, status)
	}

	statusConditions := status.Conditions.ToMetaV1Conditions()

	var errs []error
	err = controllerhelpers.RunSync(
		&statusConditions,
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
		&statusConditions,
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
		&statusConditions,
		roleControllerProgressingCondition,
		roleControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncRoles(ctx, nc, roles)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync Role(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&statusConditions,
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
		&statusConditions,
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
		&statusConditions,
		roleBindingControllerProgressingCondition,
		roleBindingControllerDegradedCondition,
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncc.syncRoleBindings(ctx, nc, roleBindings)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync RoleBinding(s): %w", err))
	}

	err = controllerhelpers.RunSync(
		&statusConditions,
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

	availableConditions := controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, scyllav1alpha1.AvailableCondition)

	controllerNodeAvailableConditionTypeFuncs := []func(*corev1.Node) string{
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeSetupAvailableConditionFormat, n.Name)
		},
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeTuneAvailableConditionFormat, n.Name)
		},
	}
	newNodeAvailableConditionFunc := func(generation int64, node *corev1.Node) metav1.Condition {
		return metav1.Condition{
			Type:               fmt.Sprintf(internalapi.NodeAvailableConditionFormat, node.Name),
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		}
	}
	nodeAvailableConditions, err := aggregateNodeStatusConditions(
		nc.Generation,
		availableConditions,
		matchingNodes,
		controllerNodeAvailableConditionTypeFuncs,
		newNodeAvailableConditionFunc,
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate node available conditions: %w", err))
	} else {
		availableConditions = append(availableConditions, nodeAvailableConditions...)
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

	progressingConditions := controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, scyllav1alpha1.ProgressingCondition)

	controllerNodeProgressingConditionTypeFuncs := []func(*corev1.Node) string{
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeSetupProgressingConditionFormat, n.Name)
		},
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeTuneProgressingConditionFormat, n.Name)
		},
	}
	newNodeProgressingConditionFunc := func(generation int64, node *corev1.Node) metav1.Condition {
		return metav1.Condition{
			Type:               fmt.Sprintf(internalapi.NodeProgressingConditionFormat, node.Name),
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		}
	}
	nodeProgressingConditions, err := aggregateNodeStatusConditions(
		nc.Generation,
		progressingConditions,
		matchingNodes,
		controllerNodeProgressingConditionTypeFuncs,
		newNodeProgressingConditionFunc,
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate node progressing conditions: %w", err))
	} else {
		progressingConditions = append(progressingConditions, nodeProgressingConditions...)
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

	degradedConditions := controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, scyllav1alpha1.DegradedCondition)

	controllerNodeDegradedConditionTypeFuncs := []func(*corev1.Node) string{
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeSetupDegradedConditionFormat, n.Name)
		},
		func(n *corev1.Node) string {
			return fmt.Sprintf(internalapi.NodeTuneDegradedConditionFormat, n.Name)
		},
	}
	newNodeDegradedConditionFunc := func(generation int64, node *corev1.Node) metav1.Condition {
		return metav1.Condition{
			Type:               fmt.Sprintf(internalapi.NodeDegradedConditionFormat, node.Name),
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		}
	}
	nodeDegradedConditions, err := aggregateNodeStatusConditions(
		nc.Generation,
		degradedConditions,
		matchingNodes,
		controllerNodeDegradedConditionTypeFuncs,
		newNodeDegradedConditionFunc,
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate node degraded conditions: %w", err))
	} else {
		degradedConditions = append(degradedConditions, nodeDegradedConditions...)
	}

	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		degradedConditions,
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
		return apimachineryutilerrors.NewAggregate(errs)
	}

	for _, c := range nodeAvailableConditions {
		apimeta.SetStatusCondition(&statusConditions, c)
	}
	apimeta.SetStatusCondition(&statusConditions, availableCondition)

	for _, c := range nodeProgressingConditions {
		apimeta.SetStatusCondition(&statusConditions, c)
	}
	apimeta.SetStatusCondition(&statusConditions, progressingCondition)

	for _, c := range nodeDegradedConditions {
		apimeta.SetStatusCondition(&statusConditions, c)
	}
	apimeta.SetStatusCondition(&statusConditions, degradedCondition)

	// TODO(rzetelskik): remove NodeConfigReconciledConditionType in next API version.
	reconciledCondition := getNodeConfigReconciledCondition(statusConditions, nc.Generation)
	apimeta.SetStatusCondition(&statusConditions, reconciledCondition)

	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(statusConditions)
	err = ncc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return apimachineryutilerrors.NewAggregate(errs)
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

func (ncc *Controller) getRoles() (map[string]*rbacv1.Role, error) {
	roles, err := ncc.roleLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	res := map[string]*rbacv1.Role{}
	for i := range roles {
		res[roles[i].Name] = roles[i]
	}

	return res, nil
}

func (ncc *Controller) getRoleBindings() (map[string]*rbacv1.RoleBinding, error) {
	roleBindings, err := ncc.roleBindingLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list rolebindings: %w", err)
	}

	res := map[string]*rbacv1.RoleBinding{}
	for i := range roleBindings {
		res[roleBindings[i].Name] = roleBindings[i]
	}

	return res, nil
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

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return matchingNodes, nil
}

func getNodeConfigReconciledCondition(conditions []metav1.Condition, generation int64) metav1.Condition {
	reconciled := helpers.IsStatusConditionPresentAndTrue(conditions, scyllav1alpha1.AvailableCondition, generation) &&
		helpers.IsStatusConditionPresentAndFalse(conditions, scyllav1alpha1.ProgressingCondition, generation) &&
		helpers.IsStatusConditionPresentAndFalse(conditions, scyllav1alpha1.DegradedCondition, generation)

	reconciledCondition := metav1.Condition{
		Type:               string(scyllav1alpha1.NodeConfigReconciledConditionType),
		ObservedGeneration: generation,
	}

	if reconciled {
		reconciledCondition.Status = metav1.ConditionTrue
		reconciledCondition.Reason = "FullyReconciledAndUp"
		reconciledCondition.Message = "All operands are reconciled and available."
	} else {
		reconciledCondition.Status = metav1.ConditionFalse
		reconciledCondition.Reason = "NotReconciledYet"
		reconciledCondition.Message = "Not all operands are reconciled and available yet."
	}

	return reconciledCondition
}

func aggregateNodeStatusConditions(generation int64, conditions []metav1.Condition, nodes []*corev1.Node, controllerNodeConditionTypeFuncs []func(*corev1.Node) string, nodeAggregateConditionFunc func(int64, *corev1.Node) metav1.Condition) ([]metav1.Condition, error) {
	var nodeAggregateConditions []metav1.Condition

	var errs []error
	for _, n := range nodes {
		var controllerNodeConditions []metav1.Condition
		for _, f := range controllerNodeConditionTypeFuncs {
			controllerNodeConditionType := f(n)
			controllerNodeCondition := apimeta.FindStatusCondition(conditions, controllerNodeConditionType)
			if controllerNodeCondition == nil || controllerNodeCondition.ObservedGeneration < generation {
				errs = append(errs, fmt.Errorf("controller node condition missing in generation %q: %q", generation, controllerNodeConditionType))
				continue
			}

			controllerNodeConditions = append(controllerNodeConditions, *controllerNodeCondition)
		}

		nodeAggregateCondition, err := controllerhelpers.AggregateStatusConditions(controllerNodeConditions, nodeAggregateConditionFunc(generation, n))
		if err != nil {
			errs = append(errs, fmt.Errorf("can't aggregate status conditions for node %q: %w", naming.ObjRef(n), err))
			continue
		}

		nodeAggregateConditions = append(nodeAggregateConditions, nodeAggregateCondition)
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return nodeAggregateConditions, nil
}
