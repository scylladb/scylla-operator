// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import "time"

const (
	baseRolloutTimout  = 30 * time.Second
	imagePullTimeout   = 4 * time.Minute
	joinClusterTimeout = 3 * time.Minute

	// memberRolloutTimeout is the maximum amount of time it takes to start a scylla pod and become ready.
	// It includes the time to pull the images, copy the necessary files (sidecar), join the cluster and similar.
	memberRolloutTimeout = 30*time.Second + imagePullTimeout + joinClusterTimeout
)
