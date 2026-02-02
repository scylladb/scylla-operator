// Copyright (C) 2026 ScyllaDB

package managerclient

import "github.com/scylladb/go-set/strset"

// TaskType enumeration.
const (
	BackupTask         string = "backup"
	RestoreTask        string = "restore"
	One2OneRestoreTask string = "1_1_restore"
	HealthCheckTask    string = "healthcheck"
	RepairTask         string = "repair"
	TabletRepairTask   string = "tablet_repair"
	SuspendTask        string = "suspend"
	ValidateBackupTask string = "validate_backup"
)

// TasksTypes is a set of all known task types.
var TasksTypes = strset.New(
	BackupTask,
	RestoreTask,
	One2OneRestoreTask,
	HealthCheckTask,
	RepairTask,
	TabletRepairTask,
	SuspendTask,
	ValidateBackupTask,
)

// TasksResilientToTopologyChanges is a slice of tasks that
// can be running while nodes are added/removed from the cluster.
var TasksResilientToTopologyChanges = []string{
	TabletRepairTask,
}

// Status enumeration.
const (
	TaskStatusNew      string = "NEW"
	TaskStatusRunning  string = "RUNNING"
	TaskStatusStopping string = "STOPPING"
	TaskStatusStopped  string = "STOPPED"
	TaskStatusWaiting  string = "WAITING"
	TaskStatusDone     string = "DONE"
	TaskStatusError    string = "ERROR"
	TaskStatusAborted  string = "ABORTED"
)

// TaskStatusDescription contains human friendly descriptions of task statuses.
var TaskStatusDescription = map[string]string{
	TaskStatusNew:      "Task was created and awaits its first execution according to schedule",
	TaskStatusRunning:  "Task is currently running",
	TaskStatusStopping: "Task is being stopped by the user. It will be stopped shortly",
	TaskStatusStopped: "Task was stopped by the user or cluster suspend. " +
		"In case it was stopped, next execution is planned according to schedule. " +
		"In case of cluster suspend, next execution is planned according to cluster resume parameters",
	TaskStatusWaiting: "Task was stopped because it reached the end of maintenance window. " +
		"Next execution is planned at the beginning of next window",
	TaskStatusDone: "Task finished with success. " +
		"Next execution is planned according to schedule",
	TaskStatusError: "Task finished with error. " +
		"In case it hasn't used all retries, next execution is planned according to retry and backoff settings. " +
		"Otherwise, next execution is planned according to schedule",
	TaskStatusAborted: "Task was interrupted by ScyllaDB Manager shutdown. " +
		"Next execution is planned according to schedule",
}
