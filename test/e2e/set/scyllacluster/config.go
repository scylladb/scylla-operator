package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "5.4.0"
	updateToScyllaVersion    = "5.4.3"
	upgradeFromScyllaVersion = "5.2.15"
	upgradeToScyllaVersion   = "5.4.3"

	testTimeout = 45 * time.Minute

	multiDatacenterTestTimeout = 3 * time.Hour
)
