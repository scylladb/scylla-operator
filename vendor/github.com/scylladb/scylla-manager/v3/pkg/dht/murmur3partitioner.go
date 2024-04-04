// Copyright (C) 2017 ScyllaDB

package dht

import (
	"math"
)

// Full token range.
const (
	Murmur3MinToken = int64(math.MinInt64)
	Murmur3MaxToken = int64(math.MaxInt64)
)
