package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "4.4.4"
	updateToScyllaVersion    = "4.4.5"
	upgradeFromScyllaVersion = "4.3.6"
	upgradeToScyllaVersion   = "4.4.5"

	testTimout = 15 * time.Minute

	memberRolloutTimeout = 3 * time.Minute
	baseRolloutTimout    = 30 * time.Second

	baseManagerSyncTimeout = 3 * time.Minute
	managerTaskSyncTimeout = 30 * time.Second
)
