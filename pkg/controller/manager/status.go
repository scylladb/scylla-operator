// Copyright (C) 2024 ScyllaDB

package manager

import (
	"context"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, managerState *state) *scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()

	repairTaskClientErrorMap := map[string]string{}
	for _, rts := range status.Repairs {
		if rts.Error != nil {
			repairTaskClientErrorMap[rts.Name] = *rts.Error
		}
	}

	status.Repairs = []scyllav1.RepairTaskStatus{}
	for _, rt := range sc.Spec.Repairs {
		repairTaskStatus := scyllav1.RepairTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: rt.Name,
			},
		}

		managerTaskStatus, isInManagerState := managerState.RepairTasks[rt.Name]
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

	status.Backups = []scyllav1.BackupTaskStatus{}
	for _, bt := range sc.Spec.Backups {
		backupTaskStatus := scyllav1.BackupTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: bt.Name,
			},
		}

		managerTaskStatus, isInManagerState := managerState.BackupTasks[bt.Name]
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
