// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"time"

	"github.com/scylladb/gocqlx/v2/qb"
	"github.com/scylladb/scylla-manager/v3/pkg/schema/table"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// TaskListItem decorates Task with information about task runs and scheduled
// activations.
type TaskListItem struct {
	Task
	Suspended      bool       `json:"suspended"`
	NextActivation *time.Time `json:"next_activation"`
	Retry          int        `json:"retry"`
}

// ListFilter specifies filtering parameters to ListTasks.
type ListFilter struct {
	TaskType []TaskType
	Status   []Status
	Disabled bool
	Deleted  bool
	Short    bool
	TaskID   uuid.UUID
}

// ListTasks returns cluster tasks given the filtering criteria.
func (s *Service) ListTasks(ctx context.Context, clusterID uuid.UUID, filter ListFilter) ([]*TaskListItem, error) {
	s.logger.Debug(ctx, "ListTasks", "filter", filter)

	b := qb.Select(table.SchedulerTask.Name())
	b.Where(qb.Eq("cluster_id"))
	m := qb.M{
		"cluster_id": clusterID,
	}
	if len(filter.TaskType) > 0 {
		b.Where(qb.Eq("type"))
	}
	if !filter.Disabled {
		b.Where(qb.EqLit("enabled", "true"))
		b.AllowFiltering()
	}
	if !filter.Deleted {
		b.Where(qb.EqLit("deleted", "false"))
		b.AllowFiltering()
	}
	if filter.Short {
		m := table.SchedulerTask.Metadata()
		var cols []string
		cols = append(cols, m.PartKey...)
		cols = append(cols, m.SortKey...)
		cols = append(cols, "name")
		b.Columns(cols...)
	}
	if len(filter.Status) > 0 {
		b.Where(qb.In("status"))
		b.AllowFiltering()
		m["status"] = filter.Status
	}
	if filter.TaskID != uuid.Nil {
		b.Where(qb.EqLit("id", filter.TaskID.String()))
	}

	q := b.Query(s.session)
	defer q.Release()

	// This is workaround for the following error using IN keyword
	// Cannot restrict clustering columns by IN relations when a collection is selected by the query
	var tasks []*TaskListItem
	if len(filter.TaskType) == 0 {
		q.BindMap(m)
		if err := q.Select(&tasks); err != nil {
			return nil, err
		}
	} else {
		for _, tt := range filter.TaskType {
			m["type"] = tt
			q.BindMap(m)
			if err := q.Select(&tasks); err != nil {
				return nil, err
			}
		}
	}

	if filter.Short {
		return tasks, nil
	}

	s.decorateTaskListItems(clusterID, tasks)

	return tasks, nil
}

func (s *Service) decorateTaskListItems(clusterID uuid.UUID, tasks []*TaskListItem) {
	s.mu.Lock()
	l, lok := s.scheduler[clusterID]
	suspended := s.isSuspendedLocked(clusterID)
	s.mu.Unlock()

	if !lok {
		return
	}

	keys := make([]uuid.UUID, len(tasks))
	for i := range tasks {
		keys[i] = tasks[i].ID
	}
	a := l.Activations(keys...)
	for i, t := range tasks {
		if a[i].IsZero() {
			t.Suspended = suspended
		} else {
			t.NextActivation = &a[i].Time
		}
		t.Retry = int(a[i].Retry)
	}
}
