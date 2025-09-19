// Copyright (C) 2017 ScyllaDB

package managerclient

import "github.com/scylladb/go-set/strset"

// TaskType enumeration.
const (
	BackupTask         string = "backup"
	RestoreTask        string = "restore"
	One2OneRestoreTask string = "1_1_restore"
	HealthCheckTask    string = "healthcheck"
	RepairTask         string = "repair"
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
	SuspendTask,
	ValidateBackupTask,
)

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
