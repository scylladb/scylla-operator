// Copyright (C) 2025 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scmc *Controller) syncScyllaDBManagerTasks(ctx context.Context, sc *scyllav1.ScyllaCluster, smts map[string]*scyllav1alpha1.ScyllaDBManagerTask) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredScyllaDBManagerTasks, err := MigrateV1ScyllaClusterToV1Alpha1ScyllaDBManagerTasks(sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't migrate v1.ScyllaCluster to v1alpha1.ScyllaDBManagerTasks: %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredScyllaDBManagerTasks,
		smts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scmc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerTasks(sc.Namespace).Delete,
		},
		scmc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune ScyllaDBManagerTask(s): %w", err)
	}

	var errs []error
	for _, smt := range requiredScyllaDBManagerTasks {
		_, changed, err := resourceapply.ApplyScyllaDBManagerTask(ctx, scmc.scyllaClient.ScyllaV1alpha1(), scmc.scyllaDBManagerTaskLister, scmc.eventRecorder, smt, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBManagerTaskControllerProgressingCondition, smt, "apply", smt.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply ScyllaDBManagerTask: %w", err))
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
