// Copyright (c) 2026 ScyllaDB.

package defaulting

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
)

// SetDefaultsScyllaCluster sets the create-time defaults on a ScyllaCluster.
// It must not fail: under failurePolicy: Fail, a defaulter returning an error would block object creation.
func SetDefaultsScyllaCluster(sc *scyllav1.ScyllaCluster) {
}
