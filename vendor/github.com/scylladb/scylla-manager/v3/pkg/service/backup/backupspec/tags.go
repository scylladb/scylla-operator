// Copyright (C) 2017 ScyllaDB

package backupspec

import (
	"regexp"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
)

var (
	tagDateFormat         = "20060102150405"
	tagRegexp             = regexp.MustCompile("^sm_([0-9]{14})UTC$")
	errInvalidSnapshotTag = errors.New("not a Scylla Manager snapshot tag, expected format is sm_20060102150405UTC")
)

// NewSnapshotTag creates new snapshot tag for the current time.
func NewSnapshotTag() string {
	return SnapshotTagAt(timeutc.Now())
}

// SnapshotTagAt creates new snapshot tag for specified time.
func SnapshotTagAt(t time.Time) string {
	return "sm_" + t.UTC().Format(tagDateFormat) + "UTC"
}

// IsSnapshotTag returns true if provided string has valid snapshot tag format.
func IsSnapshotTag(tag string) bool {
	return tagRegexp.MatchString(tag)
}

// SnapshotTagTime returns time of the provided snapshot tag.
func SnapshotTagTime(tag string) (time.Time, error) {
	m := tagRegexp.FindStringSubmatch(tag)
	if m == nil {
		return time.Time{}, errInvalidSnapshotTag
	}
	return timeutc.Parse(tagDateFormat, m[1])
}
