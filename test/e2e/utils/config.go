// Copyright (C) 2021 ScyllaDB

package utils

import "time"

const (
	memberRolloutTimeout = 3 * time.Minute
	baseRolloutTimout    = 30 * time.Second

	baseManagerSyncTimeout = 3 * time.Minute
	managerTaskSyncTimeout = 30 * time.Second

	nodeConfigRolloutTimeout = 3 * time.Minute
)
