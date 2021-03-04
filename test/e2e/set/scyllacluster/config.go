package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "4.2.0"
	updateToScyllaVersion    = "4.2.1"
	upgradeFromScyllaVersion = "4.1.0"
	upgradeToScyllaVersion   = "4.2.0"

	testTimout = 15 * time.Minute

	memberRolloutTimeout = 3 * time.Minute
	baseRolloutTimout    = 30 * time.Second
)
