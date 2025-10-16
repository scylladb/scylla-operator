// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sdcc *Controller) syncScyllaDBDatacenterNodesStatusReports(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	services map[string]*corev1.Service,
	scyllaDBDatacenterNodesStatusReports map[string]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	scyllaDBDatacenterNodesStatusReport, err := makeScyllaDBDatacenterNodesStatusReport(sdc, services, sdcc.podLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		[]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{scyllaDBDatacenterNodesStatusReport},
		scyllaDBDatacenterNodesStatusReports,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.scyllaClient.ScyllaDBDatacenterNodesStatusReports(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune ScyllaDBDatacenterNodesStatusReport(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyScyllaDBDatacenterNodesStatusReport(ctx, sdcc.scyllaClient, sdcc.scyllaDBDatacenterNodesStatusReportLister, sdcc.eventRecorder, scyllaDBDatacenterNodesStatusReport, resourceapply.ApplyOptions{})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBDatacenterNodesStatusReportControllerProgressingCondition, scyllaDBDatacenterNodesStatusReport, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply ScyllaDBDatacenterNodesStatusReport: %w", err)
	}

	return progressingConditions, nil
}
