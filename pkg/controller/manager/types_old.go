// Copyright (C) 2017 ScyllaDB

package manager

import (
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

type RepairTask v1.RepairTaskStatus

func (r RepairTask) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		ID:         r.ID,
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

	if _, err := duration.ParseDuration(r.Interval); err != nil {
		return nil, errors.Wrap(err, "parse interval")
	}
	t.Schedule.Interval = r.Interval

	if r.NumRetries != nil {
		t.Schedule.NumRetries = int64(*r.NumRetries)
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

func (r *RepairTask) FromManager(t *managerclient.TaskListItem) error {
	r.ID = t.ID
	r.Name = t.Name
	r.Interval = t.Schedule.Interval
	r.StartDate = t.Schedule.StartDate.String()
	r.NumRetries = pointer.Ptr(t.Schedule.NumRetries)

	props := t.Properties.(map[string]interface{})
	if err := mapstructure.Decode(props, r); err != nil {
		return errors.Wrap(err, "decode properties")
	}

	return nil
}

func (r *RepairTask) GetStartDate() string {
	return r.StartDate
}

func (r *RepairTask) SetStartDate(sd string) {
	r.StartDate = sd
}

type BackupTask v1.BackupTaskStatus

func (b BackupTask) ToManager() (*managerclient.Task, error) {
	t := &managerclient.Task{
		ID:         b.ID,
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

	if _, err := duration.ParseDuration(b.Interval); err != nil {
		return nil, errors.Wrap(err, "parse interval")
	}
	t.Schedule.Interval = b.Interval

	if b.NumRetries != nil {
		t.Schedule.NumRetries = int64(*b.NumRetries)
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

func (b *BackupTask) FromManager(t *managerclient.TaskListItem) error {
	b.ID = t.ID
	b.Name = t.Name
	b.Interval = t.Schedule.Interval
	b.StartDate = t.Schedule.StartDate.String()
	b.NumRetries = pointer.Ptr(t.Schedule.NumRetries)

	props := t.Properties.(map[string]interface{})
	if err := mapstructure.Decode(props, b); err != nil {
		return errors.Wrap(err, "decode properties")
	}

	return nil
}

func (b *BackupTask) GetStartDate() string {
	return b.StartDate
}

func (b *BackupTask) SetStartDate(sd string) {
	b.StartDate = sd
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
