// Copyright (C) 2017 ScyllaDB

package timeutc

import "time"

// Now returns current time in UTC.
func Now() time.Time {
	return time.Now().UTC()
}

// Parse calls time.Parse and returns value in UTC.
func Parse(layout, value string) (time.Time, error) {
	t, err := time.Parse(layout, value)
	return t.UTC(), err
}

// Since returns the time elapsed since t.
func Since(t time.Time) time.Duration {
	return Now().Sub(t.UTC())
}

// TodayMidnight returns local midnight time in UTC.
func TodayMidnight() time.Time {
	t := time.Now().AddDate(0, 0, 1)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).UTC()
}

// MustParse call time.Parse and returns value in UTC.
// It panics on time.Parse error.
func MustParse(layout, value string) time.Time {
	t, err := Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return t
}
