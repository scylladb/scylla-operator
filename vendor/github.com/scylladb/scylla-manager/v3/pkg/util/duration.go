// Copyright (C) 2017 ScyllaDB

package util

import "time"

// EpsilonRange returns start and end of range 5% close to provided value.
func EpsilonRange(d time.Duration) (a, b time.Duration) {
	e := time.Duration(float64(d) * 1.05)
	return d - e, d + e
}
