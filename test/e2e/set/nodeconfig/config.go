// Copyright (C) 2021 ScyllaDB

package nodeconfig

import "time"

const (
	testTimeout              = 15 * time.Minute
	nodeConfigRolloutTimeout = 5 * time.Minute
	apiCallTimeout           = 5 * time.Second
)
