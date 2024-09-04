// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"maps"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (scmc *Controller) syncScyllaDBDatacenter(ctx context.Context, sc *scyllav1.ScyllaCluster, sdcs map[string]*scyllav1alpha1.ScyllaDBDatacenter) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	sdc, upgradeContext, err := migrateV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't migrate scyllav1.ScyllaCluster to v1alpha1.ScyllaDBDatacenter: %w", err)
	}

	existingSDc, ok := sdcs[sc.Name]
	// Non-nil upgrade context means Operator was updated during upgrade of ScyllaCluster.
	// Before we create the migrated ScyllaDBDatacenter, we have to make sure the ongoing upgrade state is migrated to new ConfigMap which holds it.
	// Once ScyllaDBDatacenter is created, it becomes an authoritative upgrade state.
	if upgradeContext != nil && !ok {
		// Make sure it doesn't exist
		existingSDc, err = scmc.scyllaClient.ScyllaV1alpha1().ScyllaDBDatacenters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return progressingConditions, fmt.Errorf("can't get v1alpha1.ScyllaDBDatacenter %q: %w", naming.ManualRef(sc.Namespace, sc.Name), err)
		}
		if existingSDc == nil {
			ucCM, err := scylladbdatacenter.MakeUpgradeContextConfigMap(sdc, upgradeContext)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't make upgradeContext configmap: %w", err)
			}

			// Clear OwnerReferences we don't want to neither control it here, nor allow garbage collector to remove it before it's adopted by the ScyllaDBDatacenter controller.
			ucCM.OwnerReferences = []metav1.OwnerReference{}
			_, changed, err := resourceapply.ApplyConfigMap(ctx, scmc.kubeClient.CoreV1(), scmc.configMapLister, scmc.eventRecorder, ucCM, resourceapply.ApplyOptions{
				AllowMissingControllerRef: true,
			})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBDatacenterControllerProgressingCondition, sdc, "apply", sdc.Generation)
			}
		}
	}

	maps.Copy(sdc.Labels, naming.ClusterLabelsForScyllaCluster(sc))
	sdc.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
	})

	_, changed, err := resourceapply.ApplyScyllaDBDatacenter(ctx, scmc.scyllaClient.ScyllaV1alpha1(), scmc.scyllaDBDatacenterLister, scmc.eventRecorder, sdc, resourceapply.ApplyOptions{})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBDatacenterControllerProgressingCondition, sdc, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply ScyllaDBDatacenter: %w", err)
	}

	return progressingConditions, nil
}
