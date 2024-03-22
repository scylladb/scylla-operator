// Copyright (C) 2017 ScyllaDB

package manager

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
)

type RepairTaskSpec v1.RepairTaskSpec
type RepairTaskStatus v1.RepairTaskStatus

func (r *RepairTaskSpec) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		Type:       "repair",
		Enabled:    true,
		Schedule:   new(managerclient.Schedule),
		Properties: make(map[string]interface{}),
	}

	props := t.Properties.(map[string]interface{})

	startDate, err := parseStartDate(r.StartDate)
	if err != nil {
		return nil, errors.Wrap(err, "parse start date")
	}
	t.Schedule.StartDate = &startDate
	t.Schedule.Interval = r.Interval

	if r.NumRetries != nil {
		t.Schedule.NumRetries = *r.NumRetries
	}

	if r.Cron != nil {
		t.Schedule.Cron = *r.Cron
	}
	if r.Timezone != nil {
		t.Schedule.Timezone = *r.Timezone
	}
	if r.Keyspace != nil {
		props["keyspace"] = unescapeFilters(r.Keyspace)
	}
	if r.DC != nil {
		props["dc"] = unescapeFilters(r.DC)
	}
	if r.FailFast {
		t.Schedule.NumRetries = 0
		props["fail_fast"] = true
	}

	intensity, err := strconv.ParseFloat(r.Intensity, 64)
	if err != nil {
		return nil, errors.Wrap(err, "parse intensity")
	}
	props["intensity"] = intensity
	props["parallel"] = r.Parallel
	threshold, err := parseByteCount(r.SmallTableThreshold)
	if err != nil {
		return nil, errors.Wrap(err, "parse small table threshold")
	}
	props["small_table_threshold"] = threshold

	if r.Host != nil {
		props["host"] = *r.Host
	}

	t.Name = r.Name
	t.Properties = props

	return t, nil
}

func (r *RepairTaskSpec) ToStatus() *RepairTaskStatus {
	rts := &RepairTaskStatus{
		TaskStatus: v1.TaskStatus{
			SchedulerTaskStatus: v1.SchedulerTaskStatus{
				StartDate: pointer.Ptr(r.StartDate),
				Interval:  pointer.Ptr(r.Interval),
			},
			Name: r.Name,
		},
		DC:                  r.DC,
		Keyspace:            r.Keyspace,
		FailFast:            pointer.Ptr(r.FailFast),
		Intensity:           pointer.Ptr(r.Intensity),
		Parallel:            pointer.Ptr(r.Parallel),
		SmallTableThreshold: pointer.Ptr(r.SmallTableThreshold),
	}
	if r.NumRetries != nil {
		rts.NumRetries = pointer.Ptr(*r.NumRetries)
	}
	if r.Cron != nil {
		rts.Cron = pointer.Ptr(*r.Cron)
	}
	if r.Timezone != nil {
		rts.Timezone = pointer.Ptr(*r.Timezone)
	}
	if r.Host != nil {
		rts.Host = pointer.Ptr(*r.Host)
	}

	return rts
}

func (r *RepairTaskSpec) GetStartDate() string {
	return r.StartDate
}

func (r *RepairTaskSpec) SetStartDate(sd string) {
	r.StartDate = sd
}

func NewRepairStatusFromManager(t *managerclient.TaskListItem) (*RepairTaskStatus, error) {
	rts := &RepairTaskStatus{
		TaskStatus: v1.TaskStatus{
			SchedulerTaskStatus: v1.SchedulerTaskStatus{
				StartDate:  pointer.Ptr(t.Schedule.StartDate.String()),
				Interval:   pointer.Ptr(t.Schedule.Interval),
				NumRetries: pointer.Ptr(t.Schedule.NumRetries),
				Cron:       pointer.Ptr(t.Schedule.Cron),
				Timezone:   pointer.Ptr(t.Schedule.Timezone),
			},
			Name: t.Name,
			ID:   pointer.Ptr(t.ID),
		},
	}

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, rts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return rts, nil
}

func (r *RepairTaskStatus) GetStartDate() string {
	if r.StartDate != nil {
		return *r.StartDate
	}
	return *new(string)
}

type BackupTaskSpec v1.BackupTaskSpec
type BackupTaskStatus v1.BackupTaskStatus

func (b *BackupTaskSpec) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		Type:       "backup",
		Enabled:    true,
		Schedule:   new(managerclient.Schedule),
		Properties: make(map[string]interface{}),
	}

	props := t.Properties.(map[string]interface{})

	startDate, err := parseStartDate(b.StartDate)
	if err != nil {
		return nil, errors.Wrap(err, "parse start date")
	}
	t.Schedule.StartDate = &startDate
	t.Schedule.Interval = b.Interval

	if b.NumRetries != nil {
		t.Schedule.NumRetries = *b.NumRetries
	}

	if b.Cron != nil {
		t.Schedule.Cron = *b.Cron
	}
	if b.Timezone != nil {
		t.Schedule.Timezone = *b.Timezone
	}

	if b.Keyspace != nil {
		props["keyspace"] = unescapeFilters(b.Keyspace)
	}
	if b.DC != nil {
		props["dc"] = unescapeFilters(b.DC)
	}
	props["retention"] = b.Retention
	if b.RateLimit != nil {
		props["rate_limit"] = b.RateLimit
	}
	if b.SnapshotParallel != nil {
		props["snapshot_parallel"] = b.SnapshotParallel
	}
	if b.UploadParallel != nil {
		props["upload_parallel"] = b.UploadParallel
	}

	props["location"] = b.Location

	t.Name = b.Name
	t.Properties = props

	return t, nil
}

func (b *BackupTaskSpec) ToStatus() *BackupTaskStatus {
	bts := &BackupTaskStatus{
		TaskStatus: v1.TaskStatus{
			SchedulerTaskStatus: v1.SchedulerTaskStatus{
				StartDate: pointer.Ptr(b.StartDate),
				Interval:  pointer.Ptr(b.Interval),
			},
			Name: b.Name,
		},
		DC:               b.DC,
		Keyspace:         b.Keyspace,
		Location:         b.Location,
		RateLimit:        b.RateLimit,
		Retention:        pointer.Ptr(b.Retention),
		SnapshotParallel: b.SnapshotParallel,
		UploadParallel:   b.UploadParallel,
	}
	if b.NumRetries != nil {
		bts.NumRetries = pointer.Ptr(*b.NumRetries)
	}
	if b.Cron != nil {
		bts.Cron = pointer.Ptr(*b.Cron)
	}
	if b.Timezone != nil {
		bts.Timezone = pointer.Ptr(*b.Timezone)
	}

	return bts
}

func (b *BackupTaskSpec) GetStartDate() string {
	return b.StartDate
}

func (b *BackupTaskSpec) SetStartDate(sd string) {
	b.StartDate = sd
}

func NewBackupStatusFromManager(t *managerclient.TaskListItem) (*BackupTaskStatus, error) {
	bts := &BackupTaskStatus{
		TaskStatus: v1.TaskStatus{
			SchedulerTaskStatus: v1.SchedulerTaskStatus{
				StartDate:  pointer.Ptr(t.Schedule.StartDate.String()),
				Interval:   pointer.Ptr(t.Schedule.Interval),
				NumRetries: pointer.Ptr(t.Schedule.NumRetries),
				Cron:       pointer.Ptr(t.Schedule.Cron),
				Timezone:   pointer.Ptr(t.Schedule.Timezone),
			},
			Name: t.Name,
			ID:   pointer.Ptr(t.ID),
		},
	}

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, bts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return bts, nil
}

func (b *BackupTaskStatus) GetStartDate() string {
	if b.StartDate != nil {
		return *b.StartDate
	}
	return *new(string)
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
	now := timeutc.Now()

	if value == "now" {
		return strfmt.DateTime{}, nil
	}

	if strings.HasPrefix(value, "now") {
		d, err := duration.ParseDuration(value[3:])
		if err != nil {
			return strfmt.DateTime{}, err
		}
		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	// No more heuristics, assume the user passed a date formatted string
	t, err := timeutc.Parse(time.RFC3339, value)
	if err != nil {
		return strfmt.DateTime(t), err
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
		return 0, errors.Errorf("invalid byte size string: %q; it must be real number with unit suffix: %s", s, strings.Join(exps, ","))
	}

	v, err := strconv.ParseFloat(parts[byteCountReValueIdx], 64)
	if err != nil {
		return 0, errors.Wrapf(err, "parsing value for byte size string: %s", s)
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
