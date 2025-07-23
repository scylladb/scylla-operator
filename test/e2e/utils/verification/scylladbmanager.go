// Copyright (C) 2025 ScyllaDB

package verification

import (
	"context"

	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
)

func VerifyScyllaDBManagerRestoreTaskCompleted(eo o.Gomega, ctx context.Context, managerClient *managerclient.Client, clusterID, taskID string) {
	restoreProgress, err := managerClient.RestoreProgress(ctx, clusterID, taskID, "latest")
	eo.Expect(err).NotTo(o.HaveOccurred())

	eo.Expect(restoreProgress.Errors).To(o.BeEmpty())
	eo.Expect(restoreProgress.Run).NotTo(o.BeNil())
	eo.Expect(restoreProgress.Run.Status).To(o.Equal(managerclient.TaskStatusDone), "Restore task %q is not done, status: %q, cause: %q.", taskID, restoreProgress.Run.Status, restoreProgress.Run.Cause)
}

func VerifyScyllaDBManagerRepairTaskCompleted(eo o.Gomega, ctx context.Context, managerClient *managerclient.Client, clusterID, taskID string) {
	repairProgress, err := managerClient.RepairProgress(ctx, clusterID, taskID, "latest")
	o.Expect(err).NotTo(o.HaveOccurred())

	eo.Expect(repairProgress.Run).NotTo(o.BeNil())
	eo.Expect(repairProgress.Run.Status).To(o.Equal(managerclient.TaskStatusDone), "Repair task %q is not done, status: %q, cause: %q.", taskID, repairProgress.Run.Status, repairProgress.Run.Cause)
}

func VerifyScyllaDBManagerBackupTaskCompleted(eo o.Gomega, ctx context.Context, managerClient *managerclient.Client, clusterID, taskID string) {
	backupProgress, err := managerClient.BackupProgress(ctx, clusterID, taskID, "latest")
	eo.Expect(err).NotTo(o.HaveOccurred())

	eo.Expect(backupProgress.Run).NotTo(o.BeNil())
	eo.Expect(backupProgress.Run.Status).To(o.Equal(managerclient.TaskStatusDone), "Backup task %q is not done, status: %q, cause: %q.", taskID, backupProgress.Run.Status, backupProgress.Run.Cause)
}
