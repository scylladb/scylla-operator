// Copyright (C) 2017 ScyllaDB

package table

import (
	"github.com/scylladb/gocqlx/v2/table"
)

// SchedulerTaskUpdate is subset of SchedulerTask that contain task specification, and can be used for updates.
var SchedulerTaskUpdate = table.New(table.Metadata{
	Name: "scheduler_task",
	Columns: []string{
		"cluster_id",
		"enabled",
		"id",
		"name",
		"properties",
		"sched",
		"tags",
		"type",
	},
	PartKey: []string{
		"cluster_id",
	},
	SortKey: []string{
		"type",
		"id",
	},
})
