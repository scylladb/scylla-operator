// Copyright (C) 2017 ScyllaDB

package timeutc

import (
	"testing"
	"time"
)

func TestTodayMidnight(t *testing.T) {
	l := TodayMidnight().In(time.Local)
	if l.Hour() != 0 {
		t.Error("invalid hour", l)
	}
	if l.Minute() != 0 {
		t.Error("invalid minute", l)
	}
	if l.Second() != 0 {
		t.Error("invalid second", l)
	}
	if l.Nanosecond() != 0 {
		t.Error("invalid sub second", l)
	}
}
