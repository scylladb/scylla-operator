// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (ncc *Controller) calculateStatus(nc *scyllav1alpha1.NodeConfig, matchingNodes []*corev1.Node) *scyllav1alpha1.NodeConfigStatus {
	status := nc.Status.DeepCopy()
	status.ObservedGeneration = nc.Generation

	statusConditions := nc.Status.Conditions.ToMetaV1Conditions()

	unknownNodeControllerConditionFuncs := []func(*corev1.Node) metav1.Condition{
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeSetupAvailableConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Available condition of node setup for node %q to be set.", naming.ObjRef(node)),
			}
		},
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeSetupProgressingConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Progressing condition of node setup for node %q to be set.", naming.ObjRef(node)),
			}
		},
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeSetupDegradedConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Degraded condition of node setup for node %q to be set.", naming.ObjRef(node)),
			}
		},
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeTuneAvailableConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Available condition of node tune for node %q to be set.", naming.ObjRef(node)),
			}
		},
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeTuneProgressingConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Progressing condition of node tune for node %q to be set.", naming.ObjRef(node)),
			}
		},
		func(node *corev1.Node) metav1.Condition {
			return metav1.Condition{
				Type:               fmt.Sprintf(internalapi.NodeTuneDegradedConditionFormat, node.Name),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: nc.Generation,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            fmt.Sprintf("Awaiting Degraded condition of node tune for node %q to be set.", naming.ObjRef(node)),
			}
		},
	}

	unknownNodeControllerConditions := makeUnknownNodeControllerConditions(matchingNodes, statusConditions, unknownNodeControllerConditionFuncs)
	for _, c := range unknownNodeControllerConditions {
		apimeta.SetStatusCondition(&statusConditions, c)
	}

	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(statusConditions)

	return status
}

func makeUnknownNodeControllerConditions(nodes []*corev1.Node, conditions []metav1.Condition, unknownNodeControllerConditionFuncs []func(*corev1.Node) metav1.Condition) []metav1.Condition {
	var unknownNodeControllerConditions []metav1.Condition

	for _, n := range nodes {
		for _, f := range unknownNodeControllerConditionFuncs {
			unknownNodeControllerCondition := f(n)
			existingNodeControllerCondition := apimeta.FindStatusCondition(conditions, unknownNodeControllerCondition.Type)
			if existingNodeControllerCondition == nil || existingNodeControllerCondition.ObservedGeneration < unknownNodeControllerCondition.ObservedGeneration {
				unknownNodeControllerConditions = append(unknownNodeControllerConditions, unknownNodeControllerCondition)
			}
		}
	}

	return unknownNodeControllerConditions
}

func (ncc *Controller) updateStatus(ctx context.Context, currentNodeConfig *scyllav1alpha1.NodeConfig, status *scyllav1alpha1.NodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(&currentNodeConfig.Status, status) {
		return nil
	}

	nc := currentNodeConfig.DeepCopy()
	nc.Status = *status

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(nc))

	_, err := ncc.scyllaClient.NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(nc))

	return nil
}
