// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// EachDay is a special weekday marker that matches any day.
const EachDay = time.Weekday(7)

// WeekdayTime specifies weekday and time in that day.
// The time must be less than 24h.
type WeekdayTime struct {
	Weekday time.Weekday
	Time    time.Duration
}

func (i WeekdayTime) MarshalText() (text []byte, err error) {
	day := weekday[i.Weekday]
	if day != "" {
		day += "-"
	}
	var hh, mm int
	m := int(i.Time.Minutes())
	hh = m / 60
	mm = m % 60
	return []byte(fmt.Sprintf("%s%d:%02d", day, hh, mm)), nil
}

var (
	weekdayTimeRegexp = regexp.MustCompile("(?i)^((Mon|Tue|Wed|Thu|Fri|Sat|Sun)-)?([0-9]{1,2}):([0-9]{2})$")
	weekday           = map[time.Weekday]string{
		time.Monday:    "Mon",
		time.Tuesday:   "Tue",
		time.Wednesday: "Wed",
		time.Thursday:  "Thu",
		time.Friday:    "Fri",
		time.Saturday:  "Sat",
		time.Sunday:    "Sun",
	}
	weekdayRev = map[string]time.Weekday{
		"":    EachDay,
		"mon": time.Monday,
		"tue": time.Tuesday,
		"wed": time.Wednesday,
		"thu": time.Thursday,
		"fri": time.Friday,
		"sat": time.Saturday,
		"sun": time.Sunday,
	}
)

func (i *WeekdayTime) UnmarshalText(text []byte) error {
	m := weekdayTimeRegexp.FindSubmatch(text)
	if len(m) == 0 {
		return errors.New("invalid format")
	}
	var wdt WeekdayTime

	w, ok := weekdayRev[strings.ToLower(string(m[2]))]
	if !ok {
		return errors.Errorf("unknown day of week %q", string(m[2]))
	}
	wdt.Weekday = w

	hh, _ := strconv.Atoi(string(m[3])) // nolint: errcheck
	if hh >= 24 {
		return errors.Errorf("invalid hour %d", hh)
	}
	mm, _ := strconv.Atoi(string(m[4])) // nolint: errcheck
	if mm >= 60 {
		return errors.Errorf("invalid minute %d", mm)
	}
	wdt.Time = time.Duration(hh*60+mm) * time.Minute

	*i = wdt
	return nil
}

const day = 24 * time.Hour

// Next returns the closest time after now that matches the weekday and time.
// It is Location aware, the same time in different locations will have
// different results.
func (i WeekdayTime) Next(now time.Time) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	w := i.Weekday - t.Weekday()
	if w < 0 || w == 0 && now.Sub(t) > i.Time {
		w += 7
	}

	d := t.Add(time.Duration(w) * day)

	return time.Date(d.Year(), d.Month(), d.Day(), int(i.Time.Hours()), int(i.Time.Minutes())%60,
		int(i.Time.Seconds())%60, int(i.Time.Nanoseconds())%1e9, t.Location())
}

type slot struct {
	Begin WeekdayTime
	End   WeekdayTime
	pos   int
}

func (s slot) validate() error {
	if s.Begin.Weekday == EachDay || s.End.Weekday == EachDay {
		if s.Begin.Weekday != s.End.Weekday {
			return errors.New("begin and end must be each day")
		}
	}
	if s.Begin.Time >= 24*time.Hour {
		return errors.New("time must be less than 24h")
	}
	if s.End.Time >= 24*time.Hour {
		return errors.New("time must be less than 24h")
	}
	if s.Begin == s.End {
		return errors.New("equal")
	}
	return nil
}

func (s slot) expand() []slot {
	if s.Begin.Weekday != EachDay {
		return []slot{s}
	}
	w := make([]slot, 7)
	for i := 0; i < 7; i++ {
		w[i].Begin.Weekday = time.Weekday(i)
		w[i].Begin.Time = s.Begin.Time
		w[i].End.Weekday = time.Weekday(i)
		w[i].End.Time = s.End.Time
		if s.Begin.Time > s.End.Time {
			w[i].End.Weekday++
			w[i].End.Weekday %= 7
		}
	}
	return w
}

// Window specifies repeatable time windows when scheduler can run a function.
// When window ends the scheduler schedules a continuation in a next window.
type Window []slot

func NewWindow(wdt ...WeekdayTime) (Window, error) {
	if len(wdt) == 0 {
		return nil, errors.New("empty")
	}
	if len(wdt)%2 != 0 {
		return nil, errors.New("number of points must be even")
	}

	l := len(wdt) / 2
	w := make(Window, 0, l)
	for i := 0; i < l; i++ {
		j := 2 * i
		s := slot{
			Begin: wdt[j],
			End:   wdt[j+1],
			pos:   j,
		}
		if err := s.validate(); err != nil {
			return nil, errors.Wrapf(err, "[%d,%d]", j, j+1)
		}
		w = append(w, s.expand()...)
	}

	index := func(i int) int64 {
		return w[i].Begin.Next(time.Time{}).UnixNano()
	}
	sort.Slice(w, func(i, j int) bool {
		return index(i) < index(j)
	})

	return joinSlots(w), nil
}

func joinSlots(w Window) Window {
	out := make(Window, 0, len(w))
	out = append(out, w[0])

	cur := func() int {
		return len(out) - 1
	}

	// Keep rolling time and join overlapping slots.
	t := time.Time{}
	for i := 1; i < len(w); i++ {
		b := w[i].Begin.Next(t)
		e := out[cur()].End.Next(t)
		if b.After(e) {
			out = append(out, w[i])
			t = b
		} else { // nolint: gocritic
			if ew := w[i].End.Next(t); ew.After(e) {
				out[cur()].End = w[i].End
			}
		}
	}
	// Wrap around and see if the last element can ingest the first.
	if len(out) > 1 {
		b := out[0].Begin.Next(t)
		e := out[cur()].End.Next(t)
		if !b.After(e) {
			if ew := out[0].End.Next(t); ew.After(e) {
				out[cur()].End = out[0].End
				out = out[1:]
			}
		}
	}

	return out
}

// Next returns the closest open slot begin and end time given now value.
// The end time is always > now, begin may be before now in case now is
// inside an open slot.
func (w Window) Next(now time.Time) (begin, end time.Time) {
	if w == nil {
		return now, time.Time{}
	}

	// To find the smallest value of w[i].End.Time(now) over i we use
	// binary search on a helper function that compares n-th value with 0-th
	// value. The calculated values are sorted but may be shifted.
	//
	// Ex 1, now=W
	//
	// M | T | W | T | F | S | S
	// --+---+---+---+---+---+--
	// 5 | 6 | 0 | 1 | 2 | 3 | 4 (values)
	// 0 | 0 | 1 | 1 | 1 | 1 | 1 (indicator)
	//
	// Ex 2, now=S
	// --+---+---+---+---+---+--
	// 1 | 2 | 3 | 4 | 5 | 6 | 0 (values)
	// 0 | 0 | 0 | 0 | 0 | 0 | 1 (indicator)

	u0 := w[0].End.Next(now).Unix()
	i := sort.Search(len(w), func(i int) bool {
		u := w[i].End.Next(now).Unix()
		return u < u0
	})
	if i == len(w) {
		i = 0
	}

	begin = w[i].Begin.Next(now)
	end = w[i].End.Next(now)

	if begin.After(end) {
		begin = now
	}
	return // nolint: nakedret
}
