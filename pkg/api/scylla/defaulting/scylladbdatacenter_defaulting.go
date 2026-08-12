// Copyright (c) 2026 ScyllaDB.

package defaulting

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

// SetDefaultsScyllaDBDatacenter sets the create-time defaults on a ScyllaDBDatacenter.
// It must not fail: under failurePolicy: Fail, a defaulter returning an error would block object creation.
func SetDefaultsScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
}
