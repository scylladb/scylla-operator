package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "4.5.0"
	updateToScyllaVersion    = "4.5.1"
	upgradeFromScyllaVersion = "4.4.6"
	upgradeToScyllaVersion   = "4.5.1"

	testTimout = 15 * time.Minute

	memberRolloutTimeout = 3 * time.Minute
	baseRolloutTimout    = 30 * time.Second

	baseManagerSyncTimeout = 3 * time.Minute
	managerTaskSyncTimeout = 30 * time.Second
)
