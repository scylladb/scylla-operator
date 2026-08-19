// Copyright (c) 2026 ScyllaDB.

package defaulting

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/semver"
)

// SetDefaultsScyllaCluster sets the create-time defaults on a ScyllaCluster.
// It must not fail: under failurePolicy: Fail, a defaulter returning an error would block object creation.
// Mutating admission runs before the object is validated against the CRD schema, so it must also tolerate specs
// missing fields that validation guarantees.
func SetDefaultsScyllaCluster(sc *scyllav1.ScyllaCluster) {
	setDefaultScyllaClusterEnableParallelNodeOperations(sc)
}

// setDefaultScyllaClusterEnableParallelNodeOperations stamps an explicit true enableParallelNodeOperations on creation
// when the ScyllaDB version in the spec supports bootstrapping nodes in parallel.
// false is deliberately never stamped. Leaving the field unset keeps the object on whatever the resolution of an unset
// enableParallelNodeOperations is, rather than pinning it to today's one, so that objects whose owners never made a
// choice can be moved to a new default later without rewriting their specs.
// An explicitly set value, including one the validation rejects, is left untouched: correcting the user's own value is
// the validating webhook's job, not the defaulter's.
func setDefaultScyllaClusterEnableParallelNodeOperations(sc *scyllav1.ScyllaCluster) {
	if sc.Spec.EnableParallelNodeOperations != nil {
		return
	}

	// spec.version is used verbatim as the tag of the ScyllaDB image, so there's no image reference to strip the
	// version from. It's also the very value enabled parallel node operations are validated against, so the stamped
	// value can never be rejected by the validating webhook, which runs after this one.
	if !semver.SupportsParallelBootstrap(sc.Spec.Version) {
		return
	}

	sc.Spec.EnableParallelNodeOperations = new(true)
}
