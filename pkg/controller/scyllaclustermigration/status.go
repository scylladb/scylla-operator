// Copyright (c) 2024 ScyllaDB.

package scyllaclustermigration

import (
	"context"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (scmc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1.ScyllaCluster, status scyllav1.ScyllaClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = status

	klog.V(2).InfoS("Updating status", "ScyllaCluster", klog.KObj(sc))

	_, err := scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaCluster", klog.KObj(sc))

	return nil
}

func (scmc *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, sdc *scyllav1alpha1.ScyllaDBDatacenter) scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()
	if sdc == nil {
		return *status
	}

	convertedStatus := controllerhelpers.ConvertV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc)

	return scyllav1.ScyllaClusterStatus{
		ObservedGeneration: convertedStatus.ObservedGeneration,
		Racks:              convertedStatus.Racks,
		Members:            convertedStatus.Members,
		ReadyMembers:       convertedStatus.ReadyMembers,
		AvailableMembers:   convertedStatus.AvailableMembers,
		RackCount:          convertedStatus.RackCount,
		Upgrade:            convertedStatus.Upgrade,
		Conditions:         convertedStatus.Conditions,
		ManagerID:          status.ManagerID,
		Repairs:            status.Repairs,
		Backups:            status.Backups,
	}
}
