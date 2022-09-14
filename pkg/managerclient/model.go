// Copyright (C) 2017 ScyllaDB

package mermaidclient

import (
	"io"

	"github.com/scylladb/scylla-operator/pkg/mermaidclient/internal/models"
)

// ErrorResponse is returned in case of an error.
type ErrorResponse = models.ErrorResponse

// TableRenderer is the interface that components need to implement
// if they can render themselves as tables.
type TableRenderer interface {
	Render(io.Writer) error
}

// Cluster is cluster.Cluster representation.
type Cluster = models.Cluster

// ClusterSlice is []*cluster.Cluster representation.
type ClusterSlice []*models.Cluster

// ClusterStatus contains cluster status info.
type ClusterStatus models.ClusterStatus

// Task is a scheduler.Task representation.
type Task = models.Task

func makeTaskUpdate(t *Task) *models.TaskUpdate {
	return &models.TaskUpdate{
		Type:       t.Type,
		Enabled:    t.Enabled,
		Name:       t.Name,
		Schedule:   t.Schedule,
		Tags:       t.Tags,
		Properties: t.Properties,
	}
}

// RepairTarget is a representing results of dry running repair task.
type RepairTarget struct {
	models.RepairTarget
	ShowTables int
}

// BackupTarget is a representing results of dry running backup task.
type BackupTarget struct {
	models.BackupTarget
	ShowTables int
}

// ExtendedTask is a representation of scheduler.Task with additional fields
// from scheduler.Run.
type ExtendedTask = models.ExtendedTask

// ExtendedTaskSlice is a representation of a slice of scheduler.Task with
// additional fields from scheduler.Run.
type ExtendedTaskSlice = []*models.ExtendedTask

// ExtendedTasks is a representation of []*scheduler.Task with additional
// fields from scheduler.Run.
type ExtendedTasks struct {
	ExtendedTaskSlice
	All bool
}

// Schedule is a scheduler.Schedule representation.
type Schedule = models.Schedule

// TaskRun is a scheduler.TaskRun representation.
type TaskRun = models.TaskRun

// TaskRunSlice is a []*scheduler.TaskRun representation.
type TaskRunSlice []*TaskRun

// RepairProgress contains shard progress info.
type RepairProgress struct {
	*models.TaskRunRepairProgress
}

// RepairUnitProgress contains unit progress info.
type RepairUnitProgress = models.RepairProgressHostsItems0

// BackupProgress contains shard progress info.
type BackupProgress = *models.TaskRunBackupProgress

// BackupListItems is a []backup.ListItem representation.
type BackupListItems = []*models.BackupListItem
