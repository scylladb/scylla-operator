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
)
