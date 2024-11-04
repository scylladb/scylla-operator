// Copyright (C) 2024 ScyllaDB

package manager

import (
	"context"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, state *managerClusterState) *scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()

	status.ManagerID = nil
	status.Backups = []scyllav1.BackupTaskStatus{}
	status.Repairs = []scyllav1.RepairTaskStatus{}

	if state.Cluster == nil {
		return status
	}

	ownerUIDLabelValue, hasOwnerUIDLabel := state.Cluster.Labels[naming.OwnerUIDLabel]
	if !hasOwnerUIDLabel {
		klog.Warningf("Cluster %q is missing an owner UID label.", state.Cluster.Name)
	}

	if ownerUIDLabelValue != string(sc.UID) {
		// Cluster is not owned by ScyllaCluster, do not propagate its status.
		return status
	}

	status.ManagerID = pointer.Ptr(state.Cluster.ID)

	repairTaskClientErrorMap := map[string]string{}
	for _, rts := range status.Repairs {
		if rts.Error != nil {
			repairTaskClientErrorMap[rts.Name] = *rts.Error
		}
	}

	for _, rt := range sc.Spec.Repairs {
		repairTaskStatus := scyllav1.RepairTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: rt.Name,
			},
		}

		managerTaskStatus, isInManagerState := state.RepairTasks[rt.Name]
		if isInManagerState {
			repairTaskStatus = managerTaskStatus
		} else {
			// Retain the error from client.
			err, hasClientError := repairTaskClientErrorMap[rt.Name]
			if !hasClientError {
				continue
			}

			repairTaskStatus.Error = &err
		}

		status.Repairs = append(status.Repairs, repairTaskStatus)
	}

	backupTaskClientErrorMap := map[string]string{}
	for _, bts := range status.Backups {
		if bts.Error != nil {
			backupTaskClientErrorMap[bts.Name] = *bts.Error
		}
	}

	for _, bt := range sc.Spec.Backups {
		backupTaskStatus := scyllav1.BackupTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: bt.Name,
			},
		}

		managerTaskStatus, isInManagerState := state.BackupTasks[bt.Name]
		if isInManagerState {
			backupTaskStatus = managerTaskStatus
		} else {
			// Retain the error from client.
			err, hasClientError := backupTaskClientErrorMap[bt.Name]
			if !hasClientError {
				continue
			}

			backupTaskStatus.Error = &err
		}

		status.Backups = append(status.Backups, backupTaskStatus)
	}

	return status
}

func (c *Controller) updateStatus(ctx context.Context, currentSC *scyllav1.ScyllaCluster, status *scyllav1.ScyllaClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaCluster", klog.KObj(sc))

	_, err := c.scyllaClient.ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaCluster", klog.KObj(sc))

	return nil
}
