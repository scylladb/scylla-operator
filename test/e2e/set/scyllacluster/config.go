package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "6.0.1"
	updateToScyllaVersion    = "6.0.2"
	upgradeFromScyllaVersion = "6.0.2"
	upgradeToScyllaVersion   = "6.1.0"

	testTimeout = 45 * time.Minute

	multiDatacenterTestTimeout = 3 * time.Hour
)
