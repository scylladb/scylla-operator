// Copyright (c) 2026 ScyllaDB.

package unit

const (
	ScyllaDBImageRepository = "scylladb/scylla"
	ScyllaDBImageTag        = "latest"
	ScyllaDBImage           = ScyllaDBImageRepository + ":" + ScyllaDBImageTag

	ScyllaDBOperatorImage = "scylladb/scylla-operator:latest"

	ScyllaDBNodeExporterImage = "scylladb/scylladb-node-exporter:latest"

	ScyllaDBImageBelowBootstrapSynchronisationThresholdTag = "2025.1.0"
	ScyllaDBImageBelowBootstrapSynchronisationThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageBelowBootstrapSynchronisationThresholdTag

	ScyllaDBImageBelowNodeExporterThresholdTag = "2026.2.0"
	ScyllaDBImageBelowNodeExporterThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageBelowNodeExporterThresholdTag

	ScyllaDBImageAboveNodeExporterThresholdTag = "2026.3.0"
	ScyllaDBImageAboveNodeExporterThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageAboveNodeExporterThresholdTag

	ScyllaDBImageBelowParallelBootstrapThresholdTag = "2026.1.0"
	ScyllaDBImageBelowParallelBootstrapThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageBelowParallelBootstrapThresholdTag

	ScyllaDBImageAtParallelBootstrapThresholdTag = "2026.2.0"
	ScyllaDBImageAtParallelBootstrapThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageAtParallelBootstrapThresholdTag

	ScyllaDBImageAboveParallelBootstrapThresholdTag = "2026.3.0"
	ScyllaDBImageAboveParallelBootstrapThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImageAboveParallelBootstrapThresholdTag

	// Per semver precedence a pre-release version is lower than the same version without one, so a pre-release of the
	// threshold is below it.
	ScyllaDBImagePreReleaseOfParallelBootstrapThresholdTag = "2026.2.0-rc0"
	ScyllaDBImagePreReleaseOfParallelBootstrapThreshold    = ScyllaDBImageRepository + ":" + ScyllaDBImagePreReleaseOfParallelBootstrapThresholdTag
)
