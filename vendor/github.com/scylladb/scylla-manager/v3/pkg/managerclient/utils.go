// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
)

const rfc822WithSec = "02 Jan 06 15:04:05 MST"

// TaskSplit attempts to split a string into task type and ID.
// It accepts task type without ID, and validates task type against TasksTypes.
func TaskSplit(s string) (taskType string, taskID uuid.UUID, taskName string, err error) {
	if TasksTypes.Has(s) {
		taskType = s
		return
	}

	i := strings.LastIndex(s, "/")
	if i != -1 {
		taskType = s[:i]
	} else {
		taskType = s
	}
	if !TasksTypes.Has(taskType) {
		err = errors.Errorf("unknown task type %s", taskType)
		return
	}

	txt := s[i+1:]
	if id, err := uuid.Parse(txt); err != nil {
		taskName = txt
	} else {
		taskID = id
	}

	return
}

// TaskID creates a string in the form `taskType/taskName` or `taskType/taskID`.
func TaskID(t *Task) string {
	id := t.Name
	if id == "" {
		id = t.ID
	}
	return taskJoin(t.Type, id)
}

// taskJoin creates a new type id string in the form `taskType/taskId`.
func taskJoin(taskType string, taskID interface{}) string {
	return fmt.Sprint(taskType, "/", taskID)
}

func uuidFromLocation(location string) (uuid.UUID, error) {
	l, err := url.Parse(location)
	if err != nil {
		return uuid.Nil, err
	}
	_, id := path.Split(l.Path)

	return uuid.Parse(id)
}

// FormatRepairProgress returns string representation of percentage of
// successful and error repair ranges.
func FormatRepairProgress(total, success, failed int64) string {
	if total == 0 {
		return "-"
	}
	out := fmt.Sprintf("%.f%%", math.Floor(float64(success)*100/float64(total)))
	if failed > 0 {
		out += fmt.Sprintf("/%.f%%", math.Ceil(float64(failed)*100/float64(total)))
	}
	return out
}

// FormatTotalRepairProgress returns string representation of weighted repair progress.
func FormatTotalRepairProgress(successPr, errorPr int64) string {
	if successPr < 0 || errorPr < 0 {
		return "-"
	}
	out := fmt.Sprintf("%d%%", successPr)
	if errorPr > 0 {
		out += fmt.Sprintf("/%d%%", errorPr)
	}
	return out
}

// FormatRepairParallel return string representation of currently used and maximal parallel value.
func FormatRepairParallel(parallel, maxParallel int64) string {
	if maxParallel <= 0 {
		return fmt.Sprint(parallel)
	}
	if parallel > maxParallel {
		parallel = maxParallel
	}
	if parallel == 0 {
		return fmt.Sprintf("%d/%d (max)", maxParallel, maxParallel)
	}
	return fmt.Sprintf("%d/%d", parallel, maxParallel)
}

// FormatRepairIntensity return string representation of currently used and maximal intensity value.
func FormatRepairIntensity(intensity, maxIntensity float64) string {
	if maxIntensity <= 0 {
		return fmt.Sprint(intensity)
	}
	if intensity > maxIntensity {
		intensity = maxIntensity
	}
	if intensity == 0 {
		return fmt.Sprintf("%.0f/%.0f (max)", maxIntensity, maxIntensity)
	}
	return fmt.Sprintf("%.0f/%.0f", intensity, maxIntensity)
}

// FormatUploadProgress calculates percentage of success and failed uploads.
func FormatUploadProgress(size, uploaded, skipped, failed int64) string {
	if size == 0 {
		return "100%"
	}
	transferred := uploaded + skipped
	out := fmt.Sprintf("%.f%%", math.Floor(float64(transferred)*100/float64(size)))
	if failed > 0 {
		out += fmt.Sprintf("/%.f%%", math.Ceil(float64(failed)*100/float64(size)))
	}
	return out
}

// FormatRestoreProgress calculates percentage of success and failed downloads and restores.
func FormatRestoreProgress(size, restored, downloaded, failed int64) string {
	if size == 0 {
		return "100%"
	}
	out := fmt.Sprintf("%.f%%", math.Floor(float64(restored)*100/float64(size)))
	out += fmt.Sprintf(" | %.f%%", math.Floor(float64(downloaded)*100/float64(size)))
	if failed > 0 {
		out += fmt.Sprintf(" / failed: %.f%%", math.Ceil(float64(failed)*100/float64(size)))
	}
	return out
}

// FormatSizeSuffix returns string printing size with a unit.
func FormatSizeSuffix(b int64) string {
	return SizeSuffix(b).String()
}

// FormatTime formats the supplied DateTime in `02 Jan 06 15:04:05 MST` format.
func FormatTime(t strfmt.DateTime) string {
	if isZero(t) {
		return ""
	}
	return time.Time(t).Local().Format(rfc822WithSec) //nolint: gosmopolitan
}

// FormatTimePointer see FormatTime.
func FormatTimePointer(t *strfmt.DateTime) string {
	var tf strfmt.DateTime
	if t != nil {
		tf = *t
	}
	return FormatTime(tf)
}

// FormatDuration creates and formats the duration between
// the supplied DateTime values.
// If t1 is zero it will default to current time.
func FormatDuration(t0, t1 strfmt.DateTime) string {
	if isZero(t0) && isZero(t1) {
		return "0s"
	}
	var d time.Duration
	if isZero(t1) {
		d = timeutc.Now().Sub(time.Time(t0))
	} else {
		d = time.Time(t1).Sub(time.Time(t0))
	}

	return d.Truncate(time.Second).String()
}

// FormatMsDuration returns string representation of duration as number of
// milliseconds.
func FormatMsDuration(d int64) string {
	return (time.Duration(d) * time.Millisecond).Truncate(time.Second).String()
}

func isZero(t strfmt.DateTime) bool {
	return time.Time(t).IsZero()
}

// FormatTables returns tables listing if number of tables is lower than
// threshold. It prints (n tables) or (table_a, table_b, ...).
func FormatTables(threshold int, tables []string, all bool) string {
	var out string
	if len(tables) == 0 || threshold == 0 || (threshold > 0 && len(tables) > threshold) {
		if len(tables) == 1 {
			out = "(1 table)"
		} else {
			out = fmt.Sprintf("(%d tables)", len(tables))
		}
	}
	if out == "" {
		out = "(" + strings.Join(tables, ", ") + ")"
	}
	if all {
		out = "all " + out
	}
	return out
}

// FormatRestoreTables returns tables listing if number of tables is lower than
// threshold. It prints (n tables) or (table_a: size_a, table_b: size_b, ...).
func FormatRestoreTables(threshold int, tables []*models.RestoreTable) string {
	var out string
	if len(tables) == 0 || threshold == 0 || (threshold > 0 && len(tables) > threshold) {
		if len(tables) == 1 {
			out = "(1 table)"
		} else {
			out = fmt.Sprintf("(%d tables)", len(tables))
		}
	}
	if out == "" {
		var tableStr []string
		for _, t := range tables {
			tableStr = append(tableStr, fmt.Sprintf("%s: %s", t.Table, FormatSizeSuffix(t.Size)))
		}

		out = "(" + strings.Join(tableStr, ", ") + ")"
	}
	return out
}

// FormatClusterName writes cluster name and id to the writer.
func FormatClusterName(w io.Writer, c *Cluster) {
	fmt.Fprintf(w, "Cluster: %s (%s)\n", c.Name, c.ID)
}

// FormatIntensity returns text representation of repair intensity.
func FormatIntensity(v float64) string {
	if v == -1 {
		return "max"
	}
	if math.Floor(v) == v {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.2f", v)
}

// FormatRetentionPolicy returns text representation of retention policy.
func FormatRetentionPolicy(retention, retentionDays int64) string {
	var res string
	if retention == 0 && retentionDays == 0 {
		res += "  - Scylla Manager will not purge any backups created by this task\n"
	}
	if retention > 0 {
		res += "  - Last "
		if retention == 1 {
			res += "backup\n"
		} else {
			res += fmt.Sprintf("%d backups\n", retention)
		}
	}
	if retentionDays > 0 {
		res += fmt.Sprintf("  - Backups not older than %d ", retentionDays)
		if retentionDays == 1 {
			res += "day\n"
		} else {
			res += "days\n"
		}
	}
	return res
}
