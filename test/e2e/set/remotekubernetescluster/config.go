package remotekubernetescluster

import "time"

const (
	testSetupTimeout    = 1 * time.Minute
	testTeardownTimeout = 1 * time.Minute
	testTimeout         = 3 * time.Minute
)
