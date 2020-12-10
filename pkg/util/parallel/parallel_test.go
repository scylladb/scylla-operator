// Copyright (C) 2017 ScyllaDB

package parallel

import (
	"errors"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/util/timeutc"
	"go.uber.org/atomic"
)

func TestRun(t *testing.T) {
	t.Parallel()

	const (
		n    = 50
		wait = 5 * time.Millisecond
	)

	table := []struct {
		Name     string
		Limit    int
		Duration time.Duration
	}{
		// This test is flaky under race
		//{
		//	Name:     "No limit",
		//	Duration: wait,
		//},
		{
			Name:     "One by one",
			Limit:    1,
			Duration: n * wait,
		},
		{
			Name:     "Five by five",
			Limit:    5,
			Duration: n / 5 * wait,
		},
	}

	for i := range table {
		test := table[i]

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			active := atomic.NewInt32(0)
			f := func(i int) error {
				v := active.Inc()
				if test.Limit != NoLimit {
					if v > int32(test.Limit) {
						t.Errorf("limit exeded, got %d", v)
					}
				}
				time.Sleep(wait)
				active.Dec()
				return nil
			}

			start := timeutc.Now()
			if err := Run(n, test.Limit, f); err != nil {
				t.Error("Run() error", err)
			}
			d := timeutc.Since(start)
			if a, b := epsilonRange(test.Duration); d < a || d > b {
				t.Errorf("Run() not within expected time margin %v got %v", test.Duration, d)
			}
		})
	}
}

func TestIsErrAbort(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		if ok, err := isErrAbort(Abort(nil)); !ok || err != nil {
			t.Errorf("isErrAbort() = (%v, %v), expected (%v, %v))", ok, err, true, nil)
		}
	})

	t.Run("not nil", func(t *testing.T) {
		t.Parallel()

		err := errors.New("too")

		if ok, inner := isErrAbort(Abort(err)); !ok || inner != err {
			t.Errorf("isErrAbort() = (%v, %v), expected (%v, %v))", ok, inner, true, err)
		}
	})
}

func TestAbort(t *testing.T) {
	t.Parallel()

	called := atomic.NewInt32(0)
	f := func(i int) error {
		called.Inc()
		return Abort(errors.New("boo"))
	}

	if err := Run(10, 1, f); err == nil {
		t.Error("Run() expected error")
	}

	if c := called.Load(); c != 1 {
		t.Errorf("Called %d times expected 1", c)
	}
}

// EpsilonRange returns start and end of range 5% close to provided value.
func epsilonRange(d time.Duration) (a, b time.Duration) {
	e := time.Duration(float64(d) * 1.05)
	return d - e, d + e
}
