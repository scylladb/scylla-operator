// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

var deprecatedConditionTypeFormats = []string{
	deprecatedRaidControllerNodeSetupProgressingConditionFormat,
	deprecatedRaidControllerNodeSetupDegradedConditionFormat,
	deprecatedFilesystemControllerNodeSetupProgressingConditionFormat,
	deprecatedFilesystemControllerNodeSetupDegradedConditionFormat,
	deprecatedMountControllerNodeSetupProgressingConditionFormat,
	deprecatedMountControllerNodeSetupDegradedConditionFormat,
	deprecatedLoopDeviceControllerNodeSetupProgressingConditionFormat,
	deprecatedLoopDeviceControllerNodeSetupDegradedConditionFormat,
}

func (nsc *Controller) calculateStatus(nc *scyllav1alpha1.NodeConfig) *scyllav1alpha1.NodeConfigStatus {
	status := nc.Status.DeepCopy()

	deprecatedConditionTypes := sets.New(slices.ConvertSlice(deprecatedConditionTypeFormats, func(conditionTypeFormat string) string {
		return fmt.Sprintf(conditionTypeFormat, nsc.nodeName)
	})...)

	statusConditions := status.Conditions.ToMetaV1Conditions()
	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(slices.FilterOut(statusConditions, func(c metav1.Condition) bool {
		return deprecatedConditionTypes.Has(c.Type)
	}))

	return status
}

func (nsc *Controller) updateStatus(ctx context.Context, currentNC *scyllav1alpha1.NodeConfig, status *scyllav1alpha1.NodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(currentNC.Status, status) {
		return nil
	}

	nc := currentNC.DeepCopy()
	nc.Status = *status

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	_, err := nsc.scyllaClient.NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update node config status %q: %w", nsc.nodeConfigName, err)
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	return nil
}
