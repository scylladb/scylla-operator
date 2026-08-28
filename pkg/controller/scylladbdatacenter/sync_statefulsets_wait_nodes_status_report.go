package scylladbdatacenter

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

// waitForNodesStatusReportController waits for the ScyllaDBDatacenterNodesStatusReport controller to settle before
// making any changes. This ensures that the status report is up to date, which lowers the chance of a new node being
// bootstrapped while the cluster is unhealthy.
func (sdcc *Controller) waitForNodesStatusReportController(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	isProgressing := apimeta.IsStatusConditionTrue(sc.status.Conditions, scyllaDBDatacenterNodesStatusReportControllerProgressingCondition)
	isDegraded := apimeta.IsStatusConditionTrue(sc.status.Conditions, scyllaDBDatacenterNodesStatusReportControllerDegradedCondition)
	if !isProgressing && !isDegraded {
		return proceed(), nil
	}

	klog.V(4).InfoS("Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle", "ScyllaDBDatacenter", klog.KObj(sc.sdc), "Progressing", isProgressing, "Degraded", isDegraded)
	return blockWith(newStatefulSetProgressingCondition(
		sc.sdc,
		reasonWaitingForScyllaDBDatacenterNodesStatusReportController,
		"Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle.",
	)), nil
}
