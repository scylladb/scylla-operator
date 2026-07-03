// Copyright (c) 2026 ScyllaDB.

package unit

const (
	ScyllaDBImageRepository = "scylladb/scylla"
	ScyllaDBImageTag        = "latest"
	ScyllaDBImage           = ScyllaDBImageRepository + ":" + ScyllaDBImageTag

	ScyllaDBOperatorImage = "scylladb/scylla-operator:latest"

	ScyllaDBImageBelowBootstrapSynchronisationThresholdTag = "2025.1.0"
	ScyllaDBImageBelowBootstrapSynchronisationThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageBelowBootstrapSynchronisationThresholdTag
)
