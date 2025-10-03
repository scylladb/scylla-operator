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

func (sdcc *Controller) syncScyllaDBStatusReports(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	services map[string]*corev1.Service,
	scyllaDBStatusReports map[string]*scyllav1alpha1.ScyllaDBStatusReport,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	scyllaDBStatusReport, err := makeScyllaDBStatusReport(sdc, services, sdcc.podLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		[]*scyllav1alpha1.ScyllaDBStatusReport{scyllaDBStatusReport},
		scyllaDBStatusReports,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.scyllaClient.ScyllaDBStatusReports(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune ScyllaDBStatusReport(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyScyllaDBStatusReport(ctx, sdcc.scyllaClient, sdcc.scyllaDBStatusReportLister, sdcc.eventRecorder, scyllaDBStatusReport, resourceapply.ApplyOptions{})
	if changed {
		// FIXME: is this necessary? This resource is not a "dependency".
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBStatusReportControllerProgressingCondition, scyllaDBStatusReport, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply ScyllaDBStatusReport: %w", err)
	}

	return progressingConditions, nil
}
