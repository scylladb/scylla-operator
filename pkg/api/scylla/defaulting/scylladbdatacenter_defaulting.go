// Copyright (c) 2026 ScyllaDB.

package defaulting

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/semver"
)

// SetDefaultsScyllaDBDatacenter sets the create-time defaults on a ScyllaDBDatacenter.
// It must not fail: under failurePolicy: Fail, a defaulter returning an error would block object creation.
// Mutating admission runs before the object is validated against the CRD schema, so it must also tolerate specs
// missing fields that validation guarantees.
func SetDefaultsScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	setDefaultScyllaDBDatacenterBootstrapPolicy(sdc)
}

// setDefaultScyllaDBDatacenterBootstrapPolicy stamps an explicit Parallel bootstrapPolicy on creation when the ScyllaDB
// image in the spec supports bootstrapping nodes in parallel.
// Sequential is deliberately never stamped. Leaving the field unset keeps the object on whatever the resolution of an
// unset bootstrapPolicy is, rather than pinning it to today's one, so that objects whose owners never made a choice
// can be moved to a new default later without rewriting their specs.
// An explicitly set value, including one the validation rejects, is left untouched: correcting the user's own value is
// the validating webhook's job, not the defaulter's.
// ScyllaDBDatacenters managed by a parent controller always arrive at admission with the field explicitly set, so this
// only ever takes effect for directly created ones.
func setDefaultScyllaDBDatacenterBootstrapPolicy(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	if sdc.Spec.BootstrapPolicy != nil {
		return
	}

	// An image whose version can't be determined, e.g. one pinned by a digest, is deliberately treated as not
	// supporting parallel bootstrap. This is the very value an explicitly set Parallel bootstrapPolicy is validated
	// against, so the stamped value can never be rejected by the validating webhook, which runs after this one.
	scyllaDBVersion, _ := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if !semver.SupportsParallelBootstrap(scyllaDBVersion) {
		return
	}

	sdc.Spec.BootstrapPolicy = new(scyllav1alpha1.BootstrapPolicyParallel)
}
