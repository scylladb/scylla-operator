// Copyright (c) 2024 ScyllaDB.

package scyllaclustermigration

import (
	"context"
	"fmt"
	"maps"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scmc *Controller) syncScyllaDBDatacenter(ctx context.Context, sc *scyllav1.ScyllaCluster, sdcMap map[string]*scyllav1alpha1.ScyllaDBDatacenter) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	sdc, err := controllerhelpers.ConvertV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't migrate scyllav1.ScyllaCluster to v1alpha1.ScyllaDBDatacenter: %w", err)
	}

	maps.Copy(sdc.Labels, naming.ClusterLabelsForScyllaCluster(sc))
	sdc.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
	})

	requiredSDCs := []*scyllav1alpha1.ScyllaDBDatacenter{sdc}

	err = controllerhelpers.Prune(ctx, requiredSDCs, sdcMap,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scmc.scyllaClient.ScyllaV1alpha1().ScyllaDBDatacenters(sdc.Namespace).Delete,
		},
		scmc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scylladbdatacenter(s): %w", err)
	}

	var errs []error

	for _, sdc := range requiredSDCs {
		_, changed, err := resourceapply.ApplyScyllaDBDatacenter(ctx, scmc.scyllaClient.ScyllaV1alpha1(), scmc.scyllaDBDatacenterLister, scmc.eventRecorder, sdc, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBDatacenterControllerProgressingCondition, sdc, "apply", sdc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply ScyllaDBDatacenter: %w", err))
			continue
		}
	}
	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, err
	}

	return progressingConditions, nil
}
