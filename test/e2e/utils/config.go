// Copyright (C) 2021 ScyllaDB

package utils

import "time"

const (
	// SyncTimeout is the maximum time the sync loop can last. In normal circumstances this is in the order
	// of seconds but there are special cases we need to account for. Like when the sync loop generates new keys
	// and signs certificates it can take several seconds. When the CI creates multiple clusters in parallel on
	// a constrained CPU, one cert can easily take over 30s.
	SyncTimeout        = 2 * time.Minute
	imagePullTimeout   = 4 * time.Minute
	joinClusterTimeout = 3 * time.Minute
	cleanupJobTimeout  = 1 * time.Minute

	// ScyllaDBTerminationTimeout is the amount of time that a ScyllaDB Pod needs to terminate gracefully.
	// In addition to process termination, this needs to account for kubelet sending signals, state propagation
	// and generally being busy in the CI.
	ScyllaDBTerminationTimeout = 5 * time.Minute

	// memberRolloutTimeout is the maximum amount of time it takes to start a scylla pod and become ready.
	// It includes the time to pull the images, copy the necessary files (sidecar), join the cluster and similar.
	memberRolloutTimeout = 30*time.Second + imagePullTimeout + joinClusterTimeout

	// multiDatacenterJoinClusterBuffer accounts for inter-datacenter latencies affecting RPC calls during repair procedure running on bootstrap.
	// The value should be enough to cover for up to 15ms of median inter-datacenter latency.
	// Ref: https://github.com/scylladb/scylladb/issues/19131
	multiDatacenterJoinClusterBuffer    = 15 * time.Minute
	multiDatacenterMemberRolloutTimeout = memberRolloutTimeout + multiDatacenterJoinClusterBuffer

	ScyllaDBManagerTaskNumRetries = 3
	ScyllaDBManagerTaskRetryWait  = 30 * time.Second

	// ScyllaDBManagerClusterSyncTimeout is the maximum amount of time it should take for a ScyllaDB Manager Agent to register/update a cluster with ScyllaDB Manager.
	ScyllaDBManagerClusterSyncTimeout = 3 * time.Minute
	// ScyllaDBManagerTaskSyncTimeout is the maximum amount of time it should take for a ScyllaDB Manager Agent to register/update a task with ScyllaDB Manager.
	ScyllaDBManagerTaskSyncTimeout                      = 3 * time.Minute
	ScyllaDBManagerTaskCompletionTimeout                = 10 * time.Minute
	ScyllaDBManagerMultiDatacenterTaskCompletionTimeout = 15 * time.Minute
)
