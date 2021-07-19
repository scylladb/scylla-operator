package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "4.4.1"
	updateToScyllaVersion    = "4.4.2"
	upgradeFromScyllaVersion = "4.3.2"
	upgradeToScyllaVersion   = "4.4.2"

	testTimout = 15 * time.Minute

	memberRolloutTimeout = 3 * time.Minute
	baseRolloutTimout    = 30 * time.Second

	baseManagerSyncTimeout = 3 * time.Minute
	managerTaskSyncTimeout = 30 * time.Second
)
