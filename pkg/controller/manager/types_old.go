// Copyright (C) 2017 ScyllaDB

package manager

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
)

type taskSpecInterface interface {
	GetTaskSpec() *scyllav1.TaskSpec
	ToManager() (*managerclient.Task, error)
	GetObjectHash() (string, error)
}

type RepairTaskSpec scyllav1.RepairTaskSpec

var _ taskSpecInterface = &RepairTaskSpec{}

func (r *RepairTaskSpec) GetTaskSpec() *scyllav1.TaskSpec {
	return &r.TaskSpec
}

func (r *RepairTaskSpec) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		Name:    r.Name,
		Type:    managerclient.RepairTask,
		Enabled: true,
	}

	schedule, err := schedulerTaskSpecToManager(&r.SchedulerTaskSpec)
	if err != nil {
		return nil, err
	}
	t.Schedule = schedule

	intensity, err := strconv.ParseFloat(r.Intensity, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse intensity: %w", err)
	}

	smallTableThreshold, err := parseByteCount(r.SmallTableThreshold)
	if err != nil {
		return nil, fmt.Errorf("can't parse small table threshold: %w", err)
	}

	props := map[string]interface{}{
		"intensity":             intensity,
		"parallel":              r.Parallel,
		"small_table_threshold": smallTableThreshold,
	}

	if r.DC != nil {
		props["dc"] = unescapeFilters(r.DC)
	}

	if r.FailFast {
		t.Schedule.NumRetries = 0
		props["fail_fast"] = true
	}

	if r.Keyspace != nil {
		props["keyspace"] = unescapeFilters(r.Keyspace)
	}

	if r.Host != nil {
		props["host"] = *r.Host
	}

	t.Properties = props

	return t, nil
}

func (r *RepairTaskSpec) GetObjectHash() (string, error) {
	return hashutil.HashObjects(r)
}

func (r *RepairTaskSpec) ToStatus() *scyllav1.RepairTaskStatus {
	rts := &scyllav1.RepairTaskStatus{
		DC:                  r.DC,
		Keyspace:            r.Keyspace,
		FailFast:            pointer.Ptr(r.FailFast),
		Intensity:           pointer.Ptr(r.Intensity),
		Parallel:            pointer.Ptr(r.Parallel),
		SmallTableThreshold: pointer.Ptr(r.SmallTableThreshold),
	}

	rts.TaskStatus = taskSpecToStatus(&r.TaskSpec)

	if r.Host != nil {
		rts.Host = pointer.Ptr(*r.Host)
	}

	return rts
}

func NewRepairStatusFromManager(t *managerclient.TaskListItem) (*scyllav1.RepairTaskStatus, error) {
	rts := &scyllav1.RepairTaskStatus{}

	rts.TaskStatus = newTaskStatusFromManager(t)

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, rts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return rts, nil
}

type BackupTaskSpec scyllav1.BackupTaskSpec

var _ taskSpecInterface = &BackupTaskSpec{}

func (b *BackupTaskSpec) GetTaskSpec() *scyllav1.TaskSpec {
	return &b.TaskSpec
}

func (b *BackupTaskSpec) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		Name:    b.Name,
		Type:    managerclient.BackupTask,
		Enabled: true,
	}

	schedule, err := schedulerTaskSpecToManager(&b.SchedulerTaskSpec)
	if err != nil {
		return nil, err
	}
	t.Schedule = schedule

	props := map[string]interface{}{
		"location":  b.Location,
		"retention": b.Retention,
	}

	if b.DC != nil {
		props["dc"] = unescapeFilters(b.DC)
	}

	if b.Keyspace != nil {
		props["keyspace"] = unescapeFilters(b.Keyspace)
	}

	if b.RateLimit != nil {
		props["rate_limit"] = b.RateLimit
	}

	if b.SnapshotParallel != nil {
		props["snapshot_parallel"] = b.SnapshotParallel
	}

	if b.UploadParallel != nil {
		props["upload_parallel"] = b.UploadParallel
	}

	t.Properties = props

	return t, nil
}

func (b *BackupTaskSpec) GetObjectHash() (string, error) {
	return hashutil.HashObjects(b)
}

func (b *BackupTaskSpec) ToStatus() *scyllav1.BackupTaskStatus {
	bts := &scyllav1.BackupTaskStatus{
		DC:               b.DC,
		Keyspace:         b.Keyspace,
		Location:         b.Location,
		RateLimit:        b.RateLimit,
		Retention:        pointer.Ptr(b.Retention),
		SnapshotParallel: b.SnapshotParallel,
		UploadParallel:   b.UploadParallel,
	}

	bts.TaskStatus = taskSpecToStatus(&b.TaskSpec)

	return bts
}

func NewBackupStatusFromManager(t *managerclient.TaskListItem) (*scyllav1.BackupTaskStatus, error) {
	bts := &scyllav1.BackupTaskStatus{}

	bts.TaskStatus = newTaskStatusFromManager(t)

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, bts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return bts, nil
}

func schedulerTaskSpecToManager(schedulerTaskSpec *scyllav1.SchedulerTaskSpec) (*managerclient.Schedule, error) {
	schedule := &managerclient.Schedule{}

	if schedulerTaskSpec.StartDate != nil {
		startDate, err := parseStartDate(*schedulerTaskSpec.StartDate)
		if err != nil {
			return nil, fmt.Errorf("can't parse start date: %w", err)
		}
		schedule.StartDate = &startDate
	}

	if schedulerTaskSpec.Interval != nil {
		schedule.Interval = *schedulerTaskSpec.Interval
	}

	if schedulerTaskSpec.NumRetries != nil {
		schedule.NumRetries = *schedulerTaskSpec.NumRetries
	}

	if schedulerTaskSpec.Cron != nil {
		schedule.Cron = *schedulerTaskSpec.Cron
	}

	if schedulerTaskSpec.Timezone != nil {
		schedule.Timezone = *schedulerTaskSpec.Timezone
	}

	return schedule, nil
}

func taskSpecToStatus(taskSpec *scyllav1.TaskSpec) scyllav1.TaskStatus {
	return scyllav1.TaskStatus{
		SchedulerTaskStatus: schedulerTaskSpecToStatus(&taskSpec.SchedulerTaskSpec),
		Name:                taskSpec.Name,
	}
}

func schedulerTaskSpecToStatus(schedulerTaskSpec *scyllav1.SchedulerTaskSpec) scyllav1.SchedulerTaskStatus {
	schedulerTaskStatus := scyllav1.SchedulerTaskStatus{}

	if schedulerTaskSpec.StartDate != nil {
		schedulerTaskStatus.StartDate = pointer.Ptr(*schedulerTaskSpec.StartDate)
	}

	if schedulerTaskSpec.Interval != nil {
		schedulerTaskStatus.Interval = pointer.Ptr(*schedulerTaskSpec.Interval)
	}

	if schedulerTaskSpec.NumRetries != nil {
		schedulerTaskStatus.NumRetries = pointer.Ptr(*schedulerTaskSpec.NumRetries)
	}

	if schedulerTaskSpec.Cron != nil {
		schedulerTaskStatus.Cron = pointer.Ptr(*schedulerTaskSpec.Cron)
	}

	if schedulerTaskSpec.Timezone != nil {
		schedulerTaskStatus.Timezone = pointer.Ptr(*schedulerTaskSpec.Timezone)
	}

	return schedulerTaskStatus
}

func newTaskStatusFromManager(t *managerclient.TaskListItem) scyllav1.TaskStatus {
	taskStatus := scyllav1.TaskStatus{
		SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{},
		Name:                t.Name,
		Labels:              maps.Clone(t.Labels),
		ID:                  pointer.Ptr(t.ID),
	}

	taskStatus.SchedulerTaskStatus = newSchedulerTaskStatusFromManager(t.Schedule)

	return taskStatus
}

func newSchedulerTaskStatusFromManager(schedule *managerclient.Schedule) scyllav1.SchedulerTaskStatus {
	return scyllav1.SchedulerTaskStatus{
		StartDate:  pointer.Ptr(schedule.StartDate.String()),
		Interval:   pointer.Ptr(schedule.Interval),
		NumRetries: pointer.Ptr(schedule.NumRetries),
		Cron:       pointer.Ptr(schedule.Cron),
		Timezone:   pointer.Ptr(schedule.Timezone),
	}
}

// accommodate for escaping of bash expansions, we can safely remove '\'
// as it's not a valid char in keyspace or table name
func unescapeFilters(strs []string) []string {
	for i := range strs {
		strs[i] = strings.ReplaceAll(strs[i], "\\", "")
	}
	return strs
}

// parseStartDate parses the supplied string as a strfmt.DateTime.
func parseStartDate(value string) (strfmt.DateTime, error) {
	if strings.HasPrefix(value, "now") {
		now := timeutc.Now()
		if value == "now" {
			return strfmt.DateTime{}, nil
		}

		d, err := duration.ParseDuration(value[3:])
		if err != nil {
			return strfmt.DateTime{}, err
		}
		if d == 0 {
			return strfmt.DateTime{}, nil
		}

		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	// No more heuristics, assume the user passed a date formatted string
	t, err := timeutc.Parse(time.RFC3339, value)
	if err != nil {
		return strfmt.DateTime{}, err
	}

	return strfmt.DateTime(t), nil
}

var (
	byteCountRe          = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)(B|[KMGTPE]iB)`)
	byteCountReValueIdx  = 1
	byteCountReSuffixIdx = 2
)

// parseByteCount returns byte count parsed from input string.
// This is opposite of StringByteCount function.
func parseByteCount(s string) (int64, error) {
	const unit = 1024
	var exps = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	parts := byteCountRe.FindStringSubmatch(s)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", s, strings.Join(exps, ","))
	}

	v, err := strconv.ParseFloat(parts[byteCountReValueIdx], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing value for byte size string %q: %w", s, err)
	}

	pow := 0
	for i, e := range exps {
		if e == parts[byteCountReSuffixIdx] {
			pow = i
		}
	}

	mul := math.Pow(unit, float64(pow))

	return int64(v * mul), nil
}
