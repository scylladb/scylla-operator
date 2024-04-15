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
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
)

type RepairTask scyllav1.RepairTaskStatus

func (r *RepairTask) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		ID:      r.ID,
		Name:    r.Name,
		Type:    "repair",
		Enabled: true,
	}

	schedule, err := schedulerTaskSpecToManager(&r.SchedulerTaskSpec)
	if err != nil {
		return nil, err
	}
	t.Schedule = schedule

	props := make(map[string]interface{})
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
		return nil, fmt.Errorf("can't parse intensity: %w", err)
	}
	props["intensity"] = intensity

	props["parallel"] = r.Parallel

	threshold, err := parseByteCount(r.SmallTableThreshold)
	if err != nil {
		return nil, fmt.Errorf("can't parse small table threshold: %w", err)
	}

	props["small_table_threshold"] = threshold

	if r.Host != nil {
		props["host"] = *r.Host
	}

	t.Properties = props

	return t, nil
}

func (r *RepairTask) FromManager(t *managerclient.TaskListItem) error {
	r.ID = t.ID
	r.Name = t.Name
	r.Interval = pointer.Ptr(t.Schedule.Interval)
	r.StartDate = pointer.Ptr(t.Schedule.StartDate.String())
	r.NumRetries = pointer.Ptr(t.Schedule.NumRetries)

	props := t.Properties.(map[string]interface{})
	if err := mapstructure.Decode(props, r); err != nil {
		return fmt.Errorf("can't decode properties: %w", err)
	}

	return nil
}

func (r *RepairTask) GetStartDateOrEmpty() string {
	if r.StartDate != nil {
		return *r.StartDate
	}

	return ""
}

func (r *RepairTask) SetStartDate(sd string) {
	r.StartDate = &sd
}

type BackupTask scyllav1.BackupTaskStatus

func (b *BackupTask) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		ID:      b.ID,
		Name:    b.Name,
		Type:    "backup",
		Enabled: true,
	}

	schedule, err := schedulerTaskSpecToManager(&b.SchedulerTaskSpec)
	if err != nil {
		return nil, err
	}
	t.Schedule = schedule

	props := make(map[string]interface{})

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

	t.Properties = props

	return t, nil
}

func (b *BackupTask) FromManager(t *managerclient.TaskListItem) error {
	b.ID = t.ID
	b.Name = t.Name
	b.Interval = pointer.Ptr(t.Schedule.Interval)
	b.StartDate = pointer.Ptr(t.Schedule.StartDate.String())
	b.NumRetries = pointer.Ptr(t.Schedule.NumRetries)

	props := t.Properties.(map[string]interface{})
	if err := mapstructure.Decode(props, b); err != nil {
		return fmt.Errorf("can't decode properties: %w", err)
	}

	return nil
}

func (b *BackupTask) GetStartDateOrEmpty() string {
	if b.StartDate != nil {
		return *b.StartDate
	}

	return ""
}

func (b *BackupTask) SetStartDate(sd string) {
	b.StartDate = &sd
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

	return schedule, nil
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
