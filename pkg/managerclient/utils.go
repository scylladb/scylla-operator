// Copyright (C) 2017 ScyllaDB

package mermaidclient

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	"github.com/scylladb/scylla-operator/pkg/util/timeutc"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
)

const (
	nowSafety     = 30 * time.Second
	rfc822WithSec = "02 Jan 06 15:04:05 MST"
)

// TaskSplit attempts to split a string into type and id.
func TaskSplit(s string) (taskType string, taskID uuid.UUID, err error) {
	i := strings.LastIndex(s, "/")
	if i != -1 {
		taskType = s[:i]
	}
	taskID, err = uuid.Parse(s[i+1:])
	return
}

// TaskJoin creates a new type id string in the form `taskType/taskId`.
func TaskJoin(taskType string, taskID interface{}) string {
	return fmt.Sprint(taskType, "/", taskID)
}

// ParseStartDate parses the supplied string as a strfmt.DateTime.
func ParseStartDate(value string) (strfmt.DateTime, error) {
	now := timeutc.Now()

	if value == "now" {
		return strfmt.DateTime(now.Add(nowSafety)), nil
	}

	if strings.HasPrefix(value, "now") {
		d, err := duration.ParseDuration(value[3:])
		if err != nil {
			return strfmt.DateTime{}, err
		}
		if d < 0 {
			return strfmt.DateTime(time.Time{}), errors.New("start date cannot be in the past")
		}
		if d.Duration() < nowSafety {
			return strfmt.DateTime(time.Time{}), errors.Errorf("start date must be at least in %s", nowSafety)
		}
		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	// No more heuristics, assume the user passed a date formatted string
	t, err := timeutc.Parse(time.RFC3339, value)
	if err != nil {
		return strfmt.DateTime(t), err
	}
	if t.Before(now) {
		return strfmt.DateTime(time.Time{}), errors.New("start date cannot be in the past")
	}
	if t.Before(now.Add(nowSafety)) {
		return strfmt.DateTime(time.Time{}), errors.Errorf("start date must be at least in %s", nowSafety)
	}
	return strfmt.DateTime(t), nil
}

// ParseDate parses the supplied string as a strfmt.DateTime.
func ParseDate(value string) (strfmt.DateTime, error) {
	now := timeutc.Now()

	if value == "now" {
		return strfmt.DateTime(now), nil
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

func uuidFromLocation(location string) (uuid.UUID, error) {
	l, err := url.Parse(location)
	if err != nil {
		return uuid.Nil, err
	}
	_, id := path.Split(l.Path)

	return uuid.Parse(id)
}

// FormatProgress creates complete vs. failed representation.
func FormatProgress(complete, failed float32) string {
	if failed == 0 {
		return FormatPercent(complete)
	}
	return FormatPercent(complete) + " / " + FormatPercent(failed)
}

// FormatPercent simply creates a percent representation of the supplied value.
func FormatPercent(p float32) string {
	return fmt.Sprintf("%0.2f%%", p)
}

// FormatUploadProgress calculates percentage of success and failed uploads
func FormatUploadProgress(size, uploaded, skipped, failed int64) string {
	if size == 0 {
		return "100%"
	}
	transferred := uploaded + skipped
	out := fmt.Sprintf("%d%%",
		transferred*100/size,
	)
	if failed > 0 {
		out += fmt.Sprintf("/%d%%", failed*100/size)
	}
	return out
}

// ByteCountBinary returns string representation of the byte count with proper
// unit appended if withUnit is true.
func ByteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// One decimal by default, two decimals for GiB and three for more than
	// that.
	format := "%.0f%ciB"
	if exp == 2 {
		format = "%.2f%ciB"
	} else if exp > 2 {
		format = "%.3f%ciB"
	}
	return fmt.Sprintf(format, float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatTime formats the supplied DateTime in `02 Jan 06 15:04:05 MST` format.
func FormatTime(t strfmt.DateTime) string {
	if isZero(t) {
		return ""
	}
	return time.Time(t).Local().Format(rfc822WithSec)
}

// FormatDuration creates and formats the duration between
// the supplied DateTime values.
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

	return fmt.Sprint(d.Truncate(time.Second))
}

func isZero(t strfmt.DateTime) bool {
	return time.Time(t).IsZero()
}

// FormatError formats messages created by using multierror with
// errors wrapped with host IP so that each host error is in it's own line.
// It also adds "failed to" prefix if needed.
func FormatError(msg string) string {
	const prefix = " "

	// Fairly relaxed IPv4 and IPv6 heuristic pattern, a proper pattern can
	// be very complex
	const ipRegex = `([0-9A-Fa-f]{1,4}:){7}[0-9A-Fa-f]{1,4}|(\d{1,3}\.){3}\d{1,3}`

	// Move host errors to newline
	r := regexp.MustCompile(`(^|: |; )(` + ipRegex + `): `)
	msg = r.ReplaceAllString(msg, "\n"+prefix+"${2}: ")

	// Add "failed to" prefix if needed
	if !strings.HasPrefix(msg, "failed to") && !strings.HasPrefix(msg, "\n") {
		msg = "failed to " + msg
	}

	return msg
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

// FormatClusterName writes cluster name and id to the writer.
func FormatClusterName(w io.Writer, c *Cluster) {
	fmt.Fprintf(w, "Cluster: %s (%s)\n", c.Name, c.ID)
}

// StringByteCount returns string representation of the byte count with proper
// unit.
func StringByteCount(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// No decimals by default, two decimals for GiB and three for more than
	// that.
	format := "%.0f%ciB"
	if exp == 2 {
		format = "%.2f%ciB"
	} else if exp > 2 {
		format = "%.3f%ciB"
	}
	return fmt.Sprintf(format, float64(b)/float64(div), "KMGTPE"[exp])
}

var (
	byteCountRe          = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)(B|[KMGTPE]iB)`)
	byteCountReValueIdx  = 1
	byteCountReSuffixIdx = 2
)

// ParseByteCount returns byte count parsed from input string.
// This is opposite of StringByteCount function.
func ParseByteCount(s string) (int64, error) {
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
