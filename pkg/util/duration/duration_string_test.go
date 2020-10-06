// Copyright (C) 2017 ScyllaDB

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package duration

import (
	"testing"
	"time"
)

func TestDurationString(t *testing.T) {
	t.Parallel()

	var table = []struct {
		S string
		D time.Duration
	}{
		{"0s", 0},
		{"1ns", 1 * time.Nanosecond},
		{"1.1µs", 1100 * time.Nanosecond},
		{"2.2ms", 2200 * time.Microsecond},
		{"3.3s", 3300 * time.Millisecond},
		{"4m5s", 4*time.Minute + 5*time.Second},
		{"4m5.001s", 4*time.Minute + 5001*time.Millisecond},
		{"5h6m7.001s", 5*time.Hour + 6*time.Minute + 7001*time.Millisecond},
		{"6d5h4m9.001s", 6*24*time.Hour + 5*time.Hour + 4*time.Minute + 9001*time.Millisecond},
		{"8m0.000000001s", 8*time.Minute + 1*time.Nanosecond},
		{"106751d23h47m16.854775807s", 1<<63 - 1},
		{"-106751d23h47m16.854775808s", -1 << 63},
	}

	for _, test := range table {
		if str := Duration(test.D).String(); str != test.S {
			t.Errorf("Duration(%d).String() = %s, want %s", int64(test.D), str, test.S)
		}
		if test.D > 0 {
			if str := Duration(-test.D).String(); str != "-"+test.S {
				t.Errorf("Duration(%d).String() = %s, want %s", int64(-test.D), str, "-"+test.S)
			}
		}
	}
}
