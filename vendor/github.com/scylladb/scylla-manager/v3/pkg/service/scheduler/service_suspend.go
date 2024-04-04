// Copyright (C) 2022 ScyllaDB

package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/b16set"
	"github.com/scylladb/gocqlx/v2/qb"
	"github.com/scylladb/scylla-manager/v3/pkg/schema/table"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
	"github.com/scylladb/scylla-manager/v3/pkg/store"
	"github.com/scylladb/scylla-manager/v3/pkg/util/duration"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

type suspendInfo struct {
	ClusterID    uuid.UUID   `json:"-"`
	StartedAt    time.Time   `json:"started_at"`
	PendingTasks []uuid.UUID `json:"pending_tasks"`
	RunningTask  []uuid.UUID `json:"running_tasks"`
}

var _ store.Entry = &suspendInfo{}

func (v *suspendInfo) Key() (clusterID uuid.UUID, key string) {
	return v.ClusterID, "scheduler_suspended"
}

func (v *suspendInfo) MarshalBinary() (data []byte, err error) {
	return json.Marshal(v)
}

func (v *suspendInfo) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, v)
}

// SuspendProperties specify properties of Suspend task.
type SuspendProperties struct {
	Resume     bool              `json:"resume"`
	Duration   duration.Duration `json:"duration"`
	StartTasks bool              `json:"start_tasks"`
}

// GetSuspendProperties unmarshals suspend properties and validates them.
func GetSuspendProperties(data []byte) (SuspendProperties, error) {
	properties := SuspendProperties{}
	if err := json.Unmarshal(data, &properties); err != nil {
		return properties, err
	}

	if properties.StartTasks {
		if properties.Duration == 0 {
			return properties, errors.New("can't use startTasks without a duration")
		}
	}

	return properties, nil
}

func (s *Service) initSuspended() error {
	var clusters []uuid.UUID
	if err := qb.Select(table.SchedulerTask.Name()).Distinct("cluster_id").Query(s.session).SelectRelease(&clusters); err != nil {
		return errors.Wrap(err, "list clusters")
	}

	for _, c := range clusters {
		si := &suspendInfo{ClusterID: c}
		if err := s.drawer.Get(si); err != nil {
			if !errors.Is(err, service.ErrNotFound) {
				return err
			}
		} else {
			s.suspended.Add(c.Bytes16())
			s.metrics.Suspend(c)
		}
	}

	return nil
}

// IsSuspended returns true iff cluster is suspended.
func (s *Service) IsSuspended(ctx context.Context, clusterID uuid.UUID) bool {
	s.logger.Debug(ctx, "IsSuspended", "clusterID", clusterID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isSuspendedLocked(clusterID)
}

func (s *Service) isSuspendedLocked(clusterID uuid.UUID) bool {
	return s.suspended.Has(clusterID.Bytes16())
}

// Suspend stops scheduler for a given cluster.
// Running tasks will be stopped.
// Scheduled task executions will be canceled.
// Scheduler can be later resumed, see `Resume` function.
func (s *Service) Suspend(ctx context.Context, clusterID uuid.UUID) error {
	wait, err := s.suspend(ctx, clusterID, SuspendProperties{})
	if wait != nil {
		wait()
	}
	return err
}

func (s *Service) suspend(ctx context.Context, clusterID uuid.UUID, p SuspendProperties) (func(), error) {
	if p.Duration > 0 {
		s.logger.Info(ctx, "Suspending cluster", "cluster_id", clusterID, "target", p)
	} else {
		s.logger.Info(ctx, "Suspending cluster", "cluster_id", clusterID)
	}

	si := &suspendInfo{
		ClusterID: clusterID,
		StartedAt: timeutc.Now(),
	}

	s.mu.Lock()
	if s.isSuspendedLocked(clusterID) {
		s.logger.Info(ctx, "Cluster already suspended", "cluster_id", clusterID)
		s.mu.Unlock()
		return nil, nil // nolint: nilnil
	}
	s.suspended.Add(clusterID.Bytes16())
	s.metrics.Suspend(clusterID)
	l := s.resetSchedulerLocked(si)
	s.mu.Unlock()

	if err := s.forEachClusterHealthCheckTask(clusterID, func(t *Task) error {
		s.schedule(ctx, t, false)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "schedule")
	}

	if p.Duration > 0 {
		rt, err := newResumeTask(si, p)
		if err != nil {
			return nil, errors.Wrap(err, "new resume task")
		}
		if err := table.SchedulerTask.InsertQuery(s.session).BindStruct(rt).ExecRelease(); err != nil {
			return nil, errors.Wrap(err, "put task")
		}
		s.schedule(ctx, rt, false)
	}

	if err := s.drawer.Put(si); err != nil {
		return nil, errors.Wrap(err, "save canceled tasks")
	}

	var wait func()
	if l != nil {
		wait = l.Wait
	}
	return wait, nil
}

// resetSchedulerLocked closes the current scheduler, records the information on running tasks, and creates a new empty scheduler.
// It returns the old closed scheduler.
func (s *Service) resetSchedulerLocked(si *suspendInfo) *Scheduler {
	cid := si.ClusterID
	l := s.scheduler[cid]
	if l != nil {
		si.RunningTask, si.PendingTasks = l.Close()
	}
	s.scheduler[cid] = s.newScheduler(cid)
	return l
}

// ResumeTaskID is a special task ID reserved for scheduled resume of suspended cluster.
// It can be reused for different suspend tasks at different times.
// Note that a suspended cluster cannot be suspended.
var ResumeTaskID = uuid.MustParse("805E43B0-2C0A-481E-BAB8-9C2418940D67")

func newResumeTask(si *suspendInfo, p SuspendProperties) (*Task, error) {
	p.Resume = true

	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return &Task{
		ClusterID: si.ClusterID,
		Type:      SuspendTask,
		ID:        ResumeTaskID,
		Name:      "resume",
		Enabled:   true,
		Sched: Schedule{
			StartDate:  si.StartedAt.Add(p.Duration.Duration()),
			NumRetries: 3,
			RetryWait:  duration.Duration(5 * time.Second),
		},
		Status:     StatusNew,
		Properties: b,
	}, nil
}

func newDisabledResumeTask(clusterID uuid.UUID) *Task {
	return &Task{
		ClusterID: clusterID,
		Type:      SuspendTask,
		ID:        ResumeTaskID,
		Name:      "resume",
	}
}

// Resume resumes scheduler for a suspended cluster.
func (s *Service) Resume(ctx context.Context, clusterID uuid.UUID, startTasks bool) error {
	s.logger.Info(ctx, "Resuming cluster", "cluster_id", clusterID)

	s.mu.Lock()
	if !s.suspended.Has(clusterID.Bytes16()) {
		s.mu.Unlock()
		s.logger.Info(ctx, "Cluster not suspended", "cluster_id", clusterID)
		return nil
	}
	si := &suspendInfo{ClusterID: clusterID}
	if err := s.drawer.Get(si); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			s.logger.Error(ctx, "Expected canceled tasks got none")
		} else {
			s.mu.Unlock()
			return errors.Wrap(err, "get canceled tasks")
		}
	}
	if err := s.drawer.Delete(si); err != nil {
		s.logger.Error(ctx, "Failed to delete canceled tasks", "error", err)
	}
	s.suspended.Remove(clusterID.Bytes16())
	s.metrics.Resume(clusterID)
	s.mu.Unlock()

	running := b16set.New()
	if startTasks {
		for _, u := range si.RunningTask {
			running.Add(u.Bytes16())
		}
	}
	if err := s.forEachClusterTask(clusterID, func(t *Task) error {
		r := running.Has(t.ID.Bytes16())
		if needsOneShotRun(t) {
			r = true
		}
		if t.Type == SuspendTask {
			r = false
		}
		s.schedule(ctx, t, r)
		return nil
	}); err != nil {
		return errors.Wrap(err, "schedule")
	}

	if err := s.PutTask(ctx, newDisabledResumeTask(clusterID)); err != nil {
		return errors.Wrap(err, "disable resume task")
	}

	return nil
}

func (s *Service) forEachClusterHealthCheckTask(clusterID uuid.UUID, f func(t *Task) error) error {
	q := qb.Select(table.SchedulerTask.Name()).
		Where(qb.Eq("cluster_id"), qb.Eq("type")).
		Query(s.session).
		Bind(clusterID, HealthCheckTask)
	defer q.Release()

	return forEachTaskWithQuery(q, f)
}

func (s *Service) forEachClusterTask(clusterID uuid.UUID, f func(t *Task) error) error {
	q := qb.Select(table.SchedulerTask.Name()).Where(qb.Eq("cluster_id")).Query(s.session).Bind(clusterID)
	defer q.Release()
	return forEachTaskWithQuery(q, f)
}

type suspendRunner struct {
	service *Service
}

func (s suspendRunner) Run(ctx context.Context, clusterID, _, _ uuid.UUID, properties json.RawMessage) error {
	p, err := GetSuspendProperties(properties)
	if err != nil {
		return service.ErrValidate(err)
	}

	if p.Resume {
		err = s.service.Resume(ctx, clusterID, p.StartTasks)
	} else {
		// Suspend close scheduler while running for this reason we need to
		// - detach from the context
		// - ignore wait for tasks completion
		ctx = log.CopyTraceID(context.Background(), ctx)
		_, err = s.service.suspend(ctx, clusterID, p)
	}

	return err
}
