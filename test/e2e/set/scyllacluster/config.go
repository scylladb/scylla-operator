package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "6.0.0"
	updateToScyllaVersion    = "6.0.1"
	upgradeFromScyllaVersion = "5.4.7"
	upgradeToScyllaVersion   = "6.0.1"

	testTimeout = 45 * time.Minute

	multiDatacenterTestTimeout = 3 * time.Hour
)
