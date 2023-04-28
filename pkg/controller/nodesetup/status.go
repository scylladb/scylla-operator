// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func convertMetaToNodeConfigConditions(conditions []metav1.Condition) []v1alpha1.NodeConfigCondition {
	converted := make([]v1alpha1.NodeConfigCondition, 0, len(conditions))
	for _, c := range conditions {
		converted = append(converted, v1alpha1.NodeConfigCondition{
			Type:               v1alpha1.NodeConfigConditionType(c.Type),
			Status:             corev1.ConditionStatus(c.Status),
			ObservedGeneration: c.ObservedGeneration,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}

	return converted
}

func (nsc *Controller) updateNodeStatus(ctx context.Context, currentNC *v1alpha1.NodeConfig, conditions []metav1.Condition) error {
	nc := currentNC.DeepCopy()

	for _, cond := range convertMetaToNodeConfigConditions(conditions) {
		controllerhelpers.SetNodeConfigStatusCondition(&nc.Status.Conditions, cond)
	}

	if apiequality.Semantic.DeepEqual(nc.Status, currentNC.Status) {
		return nil
	}

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	_, err := nsc.scyllaClient.NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update node config status %q: %w", nsc.nodeConfigName, err)
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	return nil
}
