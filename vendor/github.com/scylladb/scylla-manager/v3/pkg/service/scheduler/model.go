// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler/trigger"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
	"github.com/scylladb/scylla-manager/v3/pkg/util/duration"
	"github.com/scylladb/scylla-manager/v3/pkg/util/retry"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"go.uber.org/multierr"
)

// TaskType specifies the type of Task.
type TaskType string

// TaskType enumeration.
const (
	UnknownTask        TaskType = "unknown"
	BackupTask         TaskType = "backup"
	RestoreTask        TaskType = "restore"
	HealthCheckTask    TaskType = "healthcheck"
	RepairTask         TaskType = "repair"
	SuspendTask        TaskType = "suspend"
	ValidateBackupTask TaskType = "validate_backup"

	mockTask TaskType = "mock"
)

func (t TaskType) String() string {
	return string(t)
}

func (t TaskType) MarshalText() (text []byte, err error) {
	return []byte(t.String()), nil
}

func (t *TaskType) UnmarshalText(text []byte) error {
	switch TaskType(text) {
	case UnknownTask:
		*t = UnknownTask
	case BackupTask:
		*t = BackupTask
	case RestoreTask:
		*t = RestoreTask
	case HealthCheckTask:
		*t = HealthCheckTask
	case RepairTask:
		*t = RepairTask
	case SuspendTask:
		*t = SuspendTask
	case ValidateBackupTask:
		*t = ValidateBackupTask
	case mockTask:
		*t = mockTask
	default:
		return fmt.Errorf("unrecognized TaskType %q", text)
	}
	return nil
}

// Cron implements a trigger based on cron expression.
// It supports the extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every <time.Duration>.
type Cron struct {
	CronSpecification
	inner scheduler.Trigger
}

// CronSpecification combines specification for cron together with the start dates
// that defines the moment when the cron is being started.
type CronSpecification struct {
	Spec      string    `json:"spec"`
	StartDate time.Time `json:"start_date"`
}

func NewCron(spec string, startDate time.Time) (Cron, error) {
	t, err := trigger.NewCron(spec)
	if err != nil {
		return Cron{}, err
	}

	return Cron{
		CronSpecification: CronSpecification{
			Spec:      spec,
			StartDate: startDate,
		},
		inner: t,
	}, nil
}

func NewCronEvery(d time.Duration, startDate time.Time) Cron {
	c, _ := NewCron("@every "+d.String(), startDate) // nolint: errcheck
	return c
}

// MustCron calls NewCron and panics on error.
func MustCron(spec string, startDate time.Time) Cron {
	c, err := NewCron(spec, startDate)
	if err != nil {
		panic(err)
	}
	return c
}

// Next implements scheduler.Trigger.
func (c Cron) Next(now time.Time) time.Time {
	if c.inner == nil {
		return time.Time{}
	}
	if c.StartDate.After(now) {
		return c.inner.Next(c.StartDate)
	}
	return c.inner.Next(now)
}

func (c Cron) MarshalText() (text []byte, err error) {
	bytes, err := json.Marshal(c.CronSpecification)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot json marshal {%v}", c.CronSpecification)
	}
	return bytes, nil
}

func (c *Cron) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	var cronSpec CronSpecification
	err := json.Unmarshal(text, &cronSpec)
	if err != nil {
		// fallback to the < 3.2.6 approach where cron was not coupled with start date
		cronSpec = CronSpecification{
			Spec: string(text),
		}
	}

	if cronSpec.Spec == "" {
		return nil
	}
	v, err2 := NewCron(cronSpec.Spec, cronSpec.StartDate)
	if err2 != nil {
		return errors.Wrap(multierr.Combine(err, err2), "cron")
	}

	*c = v
	return nil
}

func (c Cron) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return nil, errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return c.MarshalText()
}

func (c *Cron) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return c.UnmarshalText(data)
}

func (c Cron) IsZero() bool {
	return c.inner == nil
}

// WeekdayTime adds CQL capabilities to scheduler.WeekdayTime.
type WeekdayTime struct {
	scheduler.WeekdayTime
}

func (w WeekdayTime) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return nil, errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return w.MarshalText()
}

func (w *WeekdayTime) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return w.UnmarshalText(data)
}

// Window adds JSON validation to scheduler.Window.
type Window []WeekdayTime

func (w *Window) UnmarshalJSON(data []byte) error {
	var wdt []scheduler.WeekdayTime
	if err := json.Unmarshal(data, &wdt); err != nil {
		return errors.Wrap(err, "window")
	}
	if len(wdt) == 0 {
		return nil
	}

	if _, err := scheduler.NewWindow(wdt...); err != nil {
		return errors.Wrap(err, "window")
	}

	s := make([]WeekdayTime, len(wdt))
	for i := range wdt {
		s[i] = WeekdayTime{wdt[i]}
	}
	*w = s

	return nil
}

// Window returns this window as scheduler.Window.
func (w Window) Window() scheduler.Window {
	if len(w) == 0 {
		return nil
	}
	wdt := make([]scheduler.WeekdayTime, len(w))
	for i := range w {
		wdt[i] = w[i].WeekdayTime
	}
	sw, _ := scheduler.NewWindow(wdt...) // nolint: errcheck
	return sw
}

// location adds CQL capabilities and validation to time.Location.
type location struct {
	*time.Location
}

func (l location) MarshalText() (text []byte, err error) {
	if l.Location == nil {
		return nil, nil
	}
	return []byte(l.Location.String()), nil
}

func (l *location) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}
	t, err := time.LoadLocation(string(text))
	if err != nil {
		return err
	}
	l.Location = t
	return nil
}

func (l location) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return nil, errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return l.MarshalText()
}

func (l *location) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return l.UnmarshalText(data)
}

// Timezone adds JSON validation to time.Location.
type Timezone struct {
	location
}

func NewTimezone(tz *time.Location) Timezone {
	return Timezone{location{tz}}
}

func (tz *Timezone) UnmarshalJSON(data []byte) error {
	return errors.Wrap(json.Unmarshal(data, &tz.location), "timezone")
}

// Location returns this timezone as time.Location pointer.
func (tz Timezone) Location() *time.Location {
	return tz.location.Location
}

// Schedule specify task schedule.
type Schedule struct {
	gocqlx.UDT `json:"-"`

	Cron      Cron      `json:"cron"`
	Window    Window    `json:"window"`
	Timezone  Timezone  `json:"timezone"`
	StartDate time.Time `json:"start_date"`
	// Deprecated: use cron instead
	Interval   duration.Duration `json:"interval" db:"interval_seconds"`
	NumRetries int               `json:"num_retries"`
	RetryWait  duration.Duration `json:"retry_wait"`
}

func (s Schedule) trigger() scheduler.Trigger {
	if !s.Cron.IsZero() {
		return s.Cron
	}
	return trigger.NewLegacy(s.StartDate, s.Interval.Duration())
}

func (s Schedule) backoff() retry.Backoff {
	if s.NumRetries == 0 {
		return nil
	}
	w := s.RetryWait
	if w == 0 {
		w = duration.Duration(10 * time.Minute)
	}

	b := retry.NewExponentialBackoff(w.Duration(), 0, 3*time.Hour, 2, 0)
	b = retry.WithMaxRetries(b, uint64(s.NumRetries))
	return b
}

// Task specify task type, properties and schedule.
type Task struct {
	ClusterID  uuid.UUID       `json:"cluster_id"`
	Type       TaskType        `json:"type"`
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Tags       []string        `json:"tags,omitempty"`
	Enabled    bool            `json:"enabled,omitempty"`
	Deleted    bool            `json:"deleted,omitempty"`
	Sched      Schedule        `json:"schedule,omitempty"`
	Properties json.RawMessage `json:"properties,omitempty"`

	Status       Status     `json:"status"`
	SuccessCount int        `json:"success_count"`
	ErrorCount   int        `json:"error_count"`
	LastSuccess  *time.Time `json:"last_success"`
	LastError    *time.Time `json:"last_error"`
}

func (t *Task) String() string {
	return fmt.Sprintf("%s/%s", t.Type, t.ID)
}

func (t *Task) Validate() error {
	if t == nil {
		return service.ErrNilPtr
	}

	var errs error
	if t.ID == uuid.Nil {
		errs = multierr.Append(errs, errors.New("missing ID"))
	}
	if t.ClusterID == uuid.Nil {
		errs = multierr.Append(errs, errors.New("missing ClusterID"))
	}
	if _, e := uuid.Parse(t.Name); e == nil {
		errs = multierr.Append(errs, errors.New("name cannot be an UUID"))
	}
	switch t.Type {
	case "", UnknownTask:
		errs = multierr.Append(errs, errors.New("no TaskType specified"))
	default:
		var tp TaskType
		errs = multierr.Append(errs, tp.UnmarshalText([]byte(t.Type)))
	}

	return service.ErrValidate(errors.Wrap(errs, "invalid task"))
}

// Status specifies the status of a Task.
type Status string

// Status enumeration.
const (
	StatusNew      Status = "NEW"
	StatusRunning  Status = "RUNNING"
	StatusStopping Status = "STOPPING"
	StatusStopped  Status = "STOPPED"
	StatusWaiting  Status = "WAITING"
	StatusDone     Status = "DONE"
	StatusError    Status = "ERROR"
	StatusAborted  Status = "ABORTED"
)

var allStatuses = []Status{
	StatusNew,
	StatusRunning,
	StatusStopping,
	StatusStopped,
	StatusWaiting,
	StatusDone,
	StatusError,
	StatusAborted,
}

func (s Status) String() string {
	return string(s)
}

func (s Status) MarshalText() (text []byte, err error) {
	return []byte(s.String()), nil
}

func (s *Status) UnmarshalText(text []byte) error {
	switch Status(text) {
	case StatusNew:
		*s = StatusNew
	case StatusRunning:
		*s = StatusRunning
	case StatusStopping:
		*s = StatusStopping
	case StatusStopped:
		*s = StatusStopped
	case StatusWaiting:
		*s = StatusWaiting
	case StatusDone:
		*s = StatusDone
	case StatusError:
		*s = StatusError
	case StatusAborted:
		*s = StatusAborted
	default:
		return fmt.Errorf("unrecognized Status %q", text)
	}
	return nil
}

var healthCheckActiveRunID = uuid.NewFromTime(time.Unix(0, 0))

// Run describes a running instance of a Task.
type Run struct {
	ClusterID uuid.UUID  `json:"cluster_id"`
	Type      TaskType   `json:"type"`
	TaskID    uuid.UUID  `json:"task_id"`
	ID        uuid.UUID  `json:"id"`
	Status    Status     `json:"status"`
	Cause     string     `json:"cause,omitempty"`
	Owner     string     `json:"owner"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

func newRunFromTaskInfo(ti taskInfo) *Run {
	var id uuid.UUID
	if ti.TaskType == HealthCheckTask {
		id = healthCheckActiveRunID
	} else {
		id = uuid.NewTime()
	}

	return &Run{
		ClusterID: ti.ClusterID,
		Type:      ti.TaskType,
		TaskID:    ti.TaskID,
		ID:        id,
		StartTime: now(),
		Status:    StatusRunning,
	}
}
