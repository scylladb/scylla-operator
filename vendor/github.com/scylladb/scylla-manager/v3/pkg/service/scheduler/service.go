// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/b16set"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/qb"
	"github.com/scylladb/scylla-manager/v3/pkg/metrics"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler/trigger"
	"github.com/scylladb/scylla-manager/v3/pkg/schema/table"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
	"github.com/scylladb/scylla-manager/v3/pkg/store"
	"github.com/scylladb/scylla-manager/v3/pkg/util/jsonutil"
	"github.com/scylladb/scylla-manager/v3/pkg/util/pointer"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

type (
	// Key is unique identifier of a task in scheduler.
	Key = uuid.UUID

	// Properties are defined task parameters.
	// They are JSON encoded.
	Properties = json.RawMessage

	// Scheduler instantiated with Key type.
	Scheduler = scheduler.Scheduler[Key]

	// RunContext instantiated with Key type.
	RunContext = scheduler.RunContext[Key]
)

// PropertiesDecorator modifies task properties before running.
type PropertiesDecorator func(ctx context.Context, clusterID, taskID uuid.UUID, properties json.RawMessage) (json.RawMessage, error)

type Service struct {
	session gocqlx.Session
	metrics metrics.SchedulerMetrics
	drawer  store.Store
	logger  log.Logger

	decorators map[TaskType]PropertiesDecorator
	runners    map[TaskType]Runner
	runs       map[uuid.UUID]Run
	resolver   resolver
	scheduler  map[uuid.UUID]*Scheduler
	suspended  *b16set.Set
	noContinue map[uuid.UUID]time.Time
	closed     bool
	mu         sync.Mutex
}

func NewService(session gocqlx.Session, metrics metrics.SchedulerMetrics, drawer store.Store, logger log.Logger) (*Service, error) {
	s := &Service{
		session: session,
		metrics: metrics,
		drawer:  drawer,
		logger:  logger,

		decorators: make(map[TaskType]PropertiesDecorator),
		runners:    make(map[TaskType]Runner),
		runs:       make(map[uuid.UUID]Run),
		resolver:   newResolver(),
		scheduler:  make(map[uuid.UUID]*Scheduler),
		suspended:  b16set.New(),
		noContinue: make(map[uuid.UUID]time.Time),
	}
	s.runners[SuspendTask] = suspendRunner{service: s}

	if err := s.initSuspended(); err != nil {
		return nil, errors.Wrap(err, "init suspended")
	}

	return s, nil
}

// SetPropertiesDecorator sets optional decorator of properties for a given
// task type.
func (s *Service) SetPropertiesDecorator(tp TaskType, d PropertiesDecorator) {
	s.mu.Lock()
	s.decorators[tp] = d
	s.mu.Unlock()
}

// PropertiesDecorator returns the PropertiesDecorator for a task type.
func (s *Service) PropertiesDecorator(tp TaskType) PropertiesDecorator {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.decorators[tp]
}

// SetRunner assigns runner for a given task type.
// All runners need to be registered prior to running the service.
// The registration is separated from constructor to loosen coupling between services.
func (s *Service) SetRunner(tp TaskType, r Runner) {
	s.mu.Lock()
	s.runners[tp] = r
	s.mu.Unlock()
}

func (s *Service) mustRunner(tp TaskType) Runner {
	s.mu.Lock()
	r, ok := s.runners[tp]
	s.mu.Unlock()

	if !ok {
		panic("no runner")
	}
	return r
}

// LoadTasks should be called on start it loads and schedules task from database.
func (s *Service) LoadTasks(ctx context.Context) error {
	s.logger.Info(ctx, "Loading tasks from database")

	endTime := now()
	err := s.forEachTask(func(t *Task) error {
		s.initMetrics(t)
		r, err := s.markRunningAsAborted(t, endTime)
		if err != nil {
			return errors.Wrap(err, "fix last run status")
		}
		if needsOneShotRun(t) {
			r = true
		}
		s.schedule(ctx, t, r)
		return nil
	})
	if err != nil {
		s.logger.Info(ctx, "Failed to load task from database")
	} else {
		s.logger.Info(ctx, "All tasks scheduled")
	}
	return err
}

func (s *Service) forEachTask(f func(t *Task) error) error {
	q := qb.Select(table.SchedulerTask.Name()).Query(s.session)
	defer q.Release()
	return forEachTaskWithQuery(q, f)
}

func (s *Service) markRunningAsAborted(t *Task, endTime time.Time) (bool, error) {
	r, err := s.getLastRun(t)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	if r.Status == StatusAborted {
		return true, nil
	}
	if r.Status == StatusRunning {
		r.Status = StatusAborted
		r.Cause = scheduler.ErrStoppedScheduler.Error()
		r.EndTime = &endTime
		return true, s.putRunAndUpdateTask(r)
	}

	return false, nil
}

func (s *Service) getLastRun(t *Task) (*Run, error) {
	q := s.getLastRunQuery(t, 1)
	var run Run
	return &run, q.GetRelease(&run)
}

func (s *Service) getLastRunQuery(t *Task, n int, columns ...string) *gocqlx.Queryx {
	return table.SchedulerTaskRun.SelectBuilder(columns...).
		Limit(uint(n)).
		Query(s.session).
		BindMap(qb.M{
			"cluster_id": t.ClusterID,
			"type":       t.Type,
			"task_id":    t.ID,
		})
}

// GetLastRuns returns n last runs of a task.
func (s *Service) GetLastRuns(ctx context.Context, t *Task, n int) ([]*Run, error) {
	s.logger.Debug(ctx, "GetLastRuns", "task", t, "n", n)
	q := s.getLastRunQuery(t, n)
	var runs []*Run
	return runs, q.SelectRelease(&runs)
}

// GetNthLastRun returns the n-th last task run, 0 is the last run, 1 is one run before that.
func (s *Service) GetNthLastRun(ctx context.Context, t *Task, n int) (*Run, error) {
	s.logger.Debug(ctx, "GetNthLastRun", "task", t, "n", n)

	if n < 0 {
		return nil, errors.New("index out of bounds")
	}
	if n == 0 {
		return s.getLastRun(t)
	}

	runID, err := s.nthRunID(t, n)
	if err != nil {
		return nil, err
	}
	return s.GetRun(ctx, t, runID)
}

func (s *Service) nthRunID(t *Task, n int) (uuid.UUID, error) {
	q := s.getLastRunQuery(t, n+1, "id")
	defer q.Release()

	var (
		id uuid.UUID
		i  int
	)
	iter := q.Iter()
	for iter.Scan(&id) {
		if i == n {
			return id, iter.Close()
		}
		i++
	}
	if err := iter.Close(); err != nil {
		return uuid.Nil, err
	}

	return uuid.Nil, service.ErrNotFound
}

// GetRun returns a run based on ID. If nothing was found ErrNotFound is returned.
func (s *Service) GetRun(ctx context.Context, t *Task, runID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetRun", "task", t, "run_id", runID)

	if err := t.Validate(); err != nil {
		return nil, err
	}

	r := &Run{
		ClusterID: t.ClusterID,
		Type:      t.Type,
		TaskID:    t.ID,
		ID:        runID,
	}
	q := table.SchedulerTaskRun.GetQuery(s.session).BindStruct(r)
	return r, q.GetRelease(r)
}

// PutTask upserts a task.
func (s *Service) PutTask(ctx context.Context, t *Task) error {
	create := false

	if t != nil && t.ID == uuid.Nil {
		id, err := uuid.NewRandom()
		if err != nil {
			return errors.Wrap(err, "couldn't generate random UUID for task")
		}
		t.ID = id
		t.Status = StatusNew
		create = true
	}
	s.logger.Info(ctx, "PutTask", "task", t, "schedule", t.Sched, "properties", t.Properties, "create", create)

	if err := t.Validate(); err != nil {
		return err
	}
	if err := s.shouldPutTask(create, t); err != nil {
		return err
	}

	if create { // nolint: nestif
		// Force run if there is no start date and cron.
		// Note that tasks with '--start-date now' have StartDate set to zero value.
		run := false
		if t.Sched.StartDate.IsZero() {
			t.Sched.StartDate = now()
			if t.Sched.Cron.IsZero() {
				run = true
			}
		} else if t.Sched.StartDate.Before(now()) && t.Sched.Interval != 0 {
			return errors.New("start date of scheduled task cannot be in the past")
		}

		if err := table.SchedulerTask.InsertQuery(s.session).BindStruct(t).ExecRelease(); err != nil {
			return err
		}
		s.initMetrics(t)

		s.schedule(ctx, t, run)
	} else {
		if err := table.SchedulerTaskUpdate.InsertQuery(s.session).BindStruct(t).ExecRelease(); err != nil {
			return err
		}
		s.schedule(ctx, t, false)
	}

	return nil
}

func (s *Service) shouldPutTask(create bool, t *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if create && s.isSuspendedLocked(t.ClusterID) {
		return service.ErrValidate(errors.New("cluster is suspended, scheduling tasks is not allowed"))
	}

	if t.Name != "" {
		ti := newTaskInfoFromTask(t)
		if s.resolver.FillTaskID(&ti) && ti.TaskID != t.ID {
			return errors.Errorf("task name %s is already used", t.Name)
		}
	}

	return nil
}

func (s *Service) initMetrics(t *Task) {
	s.metrics.Init(t.ClusterID, t.Type.String(), t.ID, *(*[]string)(unsafe.Pointer(&allStatuses))...)
}

func (s *Service) schedule(ctx context.Context, t *Task, run bool) {
	s.mu.Lock()
	if s.isSuspendedLocked(t.ClusterID) && t.Type != HealthCheckTask && t.Type != SuspendTask {
		s.mu.Unlock()
		return
	}

	s.resolver.Put(newTaskInfoFromTask(t))
	l, lok := s.scheduler[t.ClusterID]
	if !lok {
		l = s.newScheduler(t.ClusterID)
		s.scheduler[t.ClusterID] = l
	}
	s.mu.Unlock()

	if t.Enabled {
		d := details(t)
		if run {
			d.Trigger = trigger.NewMulti(trigger.NewOnce(), d.Trigger)
		}
		l.Schedule(ctx, t.ID, d)
	} else {
		l.Unschedule(ctx, t.ID)
	}
}

func (s *Service) newScheduler(clusterID uuid.UUID) *Scheduler {
	l := scheduler.NewScheduler[Key](now, s.run, newSchedulerListener(s.findTaskByID, s.logger.Named(clusterID.String()[0:8])))
	go l.Start(context.Background())
	return l
}

const noContinueThreshold = 500 * time.Millisecond

func (s *Service) run(ctx RunContext) (runErr error) {
	s.mu.Lock()
	ti, ok := s.resolver.FindByID(ctx.Key)
	c, cok := s.noContinue[ti.TaskID]
	if cok {
		delete(s.noContinue, ti.TaskID)
	}
	d := s.decorators[ti.TaskType]
	r := newRunFromTaskInfo(ti)
	s.runs[ti.TaskID] = *r
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.runs, ti.TaskID)
		s.mu.Unlock()
	}()

	runCtx := log.WithTraceID(ctx)
	logger := s.logger.Named(ti.ClusterID.String()[0:8])

	if ti.TaskType != HealthCheckTask {
		logger.Info(runCtx, "Run started",
			"task", ti,
			"retry", ctx.Retry,
		)
		defer func() {
			if r.Status == StatusError {
				logger.Info(runCtx, "Run ended with ERROR",
					"task", ti,
					"status", r.Status,
					"cause", r.Cause,
					"duration", r.EndTime.Sub(r.StartTime),
				)
			} else {
				logger.Info(runCtx, "Run ended",
					"task", ti,
					"status", r.Status,
					"duration", r.EndTime.Sub(r.StartTime),
				)
			}
		}()
	}

	if !ok {
		return service.ErrNotFound
	}
	if err := s.putRunAndUpdateTask(r); err != nil {
		return errors.Wrap(err, "put run")
	}
	s.metrics.BeginRun(ti.ClusterID, ti.TaskType.String(), ti.TaskID)

	defer func() {
		r.Status, r.Cause = statusAndCauseFromCtxAndErr(runCtx, runErr)
		if r.Status == StatusStopped && s.isClosed() {
			r.Status = StatusAborted
		}
		r.EndTime = pointer.TimePtr(now())

		if ti.TaskType == HealthCheckTask {
			if r.Status != StatusDone {
				r.ID = uuid.NewTime()
			}
		}
		if err := s.putRunAndUpdateTask(r); err != nil {
			logger.Error(runCtx, "Cannot update the run", "task", ti, "run", r, "error", err)
		}
		s.metrics.EndRun(ti.ClusterID, ti.TaskType.String(), ti.TaskID, r.Status.String(), r.StartTime.Unix())
	}()

	if ctx.Properties.(Properties) == nil {
		ctx.Properties = json.RawMessage("{}")
	}
	if ctx.Retry == 0 && now().Sub(c) < noContinueThreshold {
		ctx.Properties = jsonutil.Set(ctx.Properties.(Properties), "continue", false)
	}
	if d != nil {
		p, err := d(runCtx, ti.ClusterID, ti.TaskID, ctx.Properties.(Properties))
		if err != nil {
			return errors.Wrap(err, "decorate properties")
		}
		ctx.Properties = p
	}
	return s.mustRunner(ti.TaskType).Run(runCtx, ti.ClusterID, ti.TaskID, r.ID, ctx.Properties.(Properties))
}

func (s *Service) putRunAndUpdateTask(r *Run) error {
	if err := s.putRun(r); err != nil {
		return err
	}
	return s.updateTaskWithRun(r)
}

func (s *Service) putRun(r *Run) error {
	return table.SchedulerTaskRun.InsertQuery(s.session).BindStruct(r).ExecRelease()
}

func (s *Service) updateTaskWithRun(r *Run) error {
	t := Task{
		ClusterID: r.ClusterID,
		Type:      r.Type,
		ID:        r.TaskID,
		Status:    r.Status,
	}
	b := table.SchedulerTask.UpdateBuilder("status")

	var u *gocqlx.Queryx
	switch r.Status {
	case StatusDone:
		q := table.SchedulerTask.GetQuery(s.session, "success_count").BindStruct(&t)
		if err := q.GetRelease(&t); err != nil {
			return err
		}

		u = b.Set("success_count", "last_success").Query(s.session)
		t.SuccessCount++
		t.LastSuccess = r.EndTime
	case StatusError:
		q := table.SchedulerTask.GetQuery(s.session, "error_count").BindStruct(&t)
		if err := q.GetRelease(&t); err != nil {
			return err
		}

		u = b.Set("error_count", "last_error").Query(s.session)
		t.ErrorCount++
		t.LastError = r.EndTime
	default:
		u = b.Query(s.session)
	}

	return u.BindStruct(&t).ExecRelease()
}

func statusAndCauseFromCtxAndErr(ctx context.Context, err error) (taskStatus Status, taskError string) {
	switch {
	case err == nil:
		return StatusDone, ""
	case errors.Is(context.Cause(ctx), scheduler.ErrStoppedTask):
		return StatusStopped, scheduler.ErrStoppedTask.Error()
	case errors.Is(context.Cause(ctx), scheduler.ErrOutOfWindowTask):
		return StatusWaiting, scheduler.ErrOutOfWindowTask.Error()
	case errors.Is(context.Cause(ctx), scheduler.ErrStoppedScheduler):
		return StatusStopped, scheduler.ErrStoppedScheduler.Error()
	default:
		return StatusError, err.Error()
	}
}

// GetTaskByID returns a task based on ID and type. If nothing was found
// scylla-manager.ErrNotFound is returned.
func (s *Service) GetTaskByID(ctx context.Context, clusterID uuid.UUID, tp TaskType, id uuid.UUID) (*Task, error) {
	s.logger.Debug(ctx, "GetTaskByID", "cluster_id", clusterID, "id", id)
	t := &Task{
		ClusterID: clusterID,
		Type:      tp,
		ID:        id,
	}
	q := table.SchedulerTask.GetQuery(s.session).BindStruct(t)
	return t, q.GetRelease(t)
}

func (s *Service) findTaskByID(key Key) (taskInfo, bool) {
	s.mu.Lock()
	ti, ok := s.resolver.FindByID(key)
	s.mu.Unlock()
	return ti, ok
}

// DeleteTask removes and stops task based on ID.
func (s *Service) DeleteTask(ctx context.Context, t *Task) error {
	s.logger.Debug(ctx, "DeleteTask", "task", t)

	t.Deleted = true
	t.Enabled = false

	// Remove the deleted task's name so that new tasks can use it
	t.Name = ""

	q := table.SchedulerTask.UpdateQuery(s.session, "deleted", "enabled", "name").BindStruct(t)

	if err := q.ExecRelease(); err != nil {
		return err
	}

	s.mu.Lock()
	l, lok := s.scheduler[t.ClusterID]
	s.resolver.Remove(t.ID)
	s.mu.Unlock()
	if lok {
		l.Unschedule(ctx, t.ID)
	}

	s.logger.Info(ctx, "Task deleted",
		"cluster_id", t.ClusterID,
		"task_type", t.Type,
		"task_id", t.ID,
	)
	return nil
}

// StartTask starts execution of a task immediately.
func (s *Service) StartTask(ctx context.Context, t *Task) error {
	return s.startTask(ctx, t, false)
}

// StartTaskNoContinue starts execution of a task immediately and adds the
// "no_continue" flag to properties of the next run.
// The possible retries would not have the flag enabled.
func (s *Service) StartTaskNoContinue(ctx context.Context, t *Task) error {
	return s.startTask(ctx, t, true)
}

func (s *Service) startTask(ctx context.Context, t *Task, noContinue bool) error {
	s.logger.Debug(ctx, "StartTask", "task", t, "no_continue", noContinue)

	s.mu.Lock()
	if s.isSuspendedLocked(t.ClusterID) {
		s.mu.Unlock()
		return service.ErrValidate(errors.New("cluster is suspended"))
	}
	l, lok := s.scheduler[t.ClusterID]
	if !lok {
		l = s.newScheduler(t.ClusterID)
		s.scheduler[t.ClusterID] = l
	}
	if noContinue {
		s.noContinue[t.ID] = now()
	}
	s.mu.Unlock()

	// For regular tasks trigger will be enough but for one shot or disabled
	// tasks we need to reschedule them to run once.
	if !l.Trigger(ctx, t.ID) {
		d := details(t)
		d.Trigger = trigger.NewOnce()
		l.Schedule(ctx, t.ID, d)
	}
	return nil
}

// StopTask stops task execution of immediately, task is rescheduled according
// to its run interval.
func (s *Service) StopTask(ctx context.Context, t *Task) error {
	s.logger.Debug(ctx, "StopTask", "task", t)

	s.mu.Lock()
	l, lok := s.scheduler[t.ClusterID]
	r, rok := s.runs[t.ID]
	s.mu.Unlock()

	if !lok || !rok {
		return nil
	}

	r.Status = StatusStopping
	if err := s.updateRunStatus(&r); err != nil {
		return err
	}
	l.Stop(ctx, t.ID)

	return nil
}

func (s *Service) updateRunStatus(r *Run) error {
	// Only update if running as there is a race between manually stopping
	// a run and the run returning normally.
	return table.SchedulerTaskRun.
		UpdateBuilder("status").
		If(qb.EqNamed("status", "from_status")).
		Query(s.session).
		BindStructMap(r, qb.M{"from_status": StatusRunning}).
		ExecRelease()
}

func (s *Service) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Close cancels all tasks and waits for them to terminate.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	v := make([]*Scheduler, 0, len(s.scheduler))
	for _, l := range s.scheduler {
		v = append(v, l)
		l.Close()
	}
	s.mu.Unlock()

	for _, l := range v {
		l.Wait()
	}
}

func forEachTaskWithQuery(q *gocqlx.Queryx, f func(t *Task) error) error {
	var t Task
	iter := q.Iter()
	for iter.StructScan(&t) {
		if err := f(&t); err != nil {
			iter.Close()
			return err
		}
		t = Task{}
	}
	return iter.Close()
}

func needsOneShotRun(t *Task) bool {
	return t.Sched.Cron.IsZero() &&
		t.Sched.Interval == 0 &&
		t.Sched.Window != nil &&
		t.SuccessCount+t.ErrorCount == 0
}
