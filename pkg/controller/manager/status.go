// Copyright (C) 2024 ScyllaDB

package manager

import (
	"context"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, managerState *state) *scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()

	status.Repairs = []scyllav1.RepairTaskStatus{}
	for _, rt := range sc.Spec.Repairs {
		managerTaskStatus, ok := managerState.RepairTasks[rt.Name]
		if !ok {
			continue
		}
		status.Repairs = append(status.Repairs, scyllav1.RepairTaskStatus(managerTaskStatus))
	}

	status.Backups = []scyllav1.BackupTaskStatus{}
	for _, bt := range sc.Spec.Backups {
		managerTaskStatus, ok := managerState.BackupTasks[bt.Name]
		if !ok {
			continue
		}
		status.Backups = append(status.Backups, scyllav1.BackupTaskStatus(managerTaskStatus))
	}

	return status
}

func syncStatusWithManagerState(sc *scyllav1.ScyllaCluster, status *scyllav1.ScyllaClusterStatus, managerState *state) {
	for _, rt := range sc.Spec.Repairs {
		managerTaskStatus, ok := managerState.RepairTasks[rt.Name]
		if !ok {
			continue
		}

		repairTaskStatus := scyllav1.RepairTaskStatus(managerTaskStatus)
		_, i, ok := slices.Find(status.Repairs, func(rts scyllav1.RepairTaskStatus) bool {
			return rts.Name == rt.Name
		})

		if ok {
			// Retain the error from client.
			repairTaskStatus.Error = status.Repairs[i].Error
			status.Repairs[i] = repairTaskStatus
			continue
		}

		status.Repairs = append(status.Repairs, repairTaskStatus)
	}

	for _, bt := range sc.Spec.Backups {
		managerTaskStatus, ok := managerState.BackupTasks[bt.Name]
		if !ok {
			continue
		}

		backupTaskStatus := scyllav1.BackupTaskStatus(managerTaskStatus)

		_, i, ok := slices.Find(status.Backups, func(bts scyllav1.BackupTaskStatus) bool {
			return bts.Name == bt.Name
		})

		if ok {
			// Retain the error from client.
			backupTaskStatus.Error = status.Backups[i].Error
			status.Backups[i] = backupTaskStatus
			continue
		}

		status.Backups = append(status.Backups, backupTaskStatus)
	}
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
