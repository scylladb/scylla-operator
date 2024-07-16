package scyllacluster

import (
	"time"
)

const (
	updateFromScyllaVersion  = "2024.3.0-dev-0.20240711.1c6306b753c2"
	updateToScyllaVersion    = "2024.3.0-dev-0.20240716.296a9d43e273"
	upgradeFromScyllaVersion = "2024.3.0-dev-0.20240711.1c6306b753c2"
	upgradeToScyllaVersion   = "2024.3.0-dev-0.20240716.296a9d43e273"

	testTimeout = 45 * time.Minute

	multiDatacenterTestTimeout = 3 * time.Hour
)
