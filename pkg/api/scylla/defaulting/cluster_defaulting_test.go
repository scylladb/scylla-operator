// Copyright (c) 2026 ScyllaDB.

package defaulting_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/defaulting"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
)

func TestSetDefaultsScyllaCluster(t *testing.T) {
	t.Parallel()

	newScyllaCluster := func(version string, enableParallelNodeOperations *bool) *scyllav1.ScyllaCluster {
		sc := unit.NewSingleRackCluster(3)
		sc.Spec.Version = version
		sc.Spec.EnableParallelNodeOperations = enableParallelNodeOperations

		return sc
	}

	tt := []struct {
		name     string
		cluster  *scyllav1.ScyllaCluster
		expected *scyllav1.ScyllaCluster
	}{
		{
			name:     "unset enableParallelNodeOperations with the minimal version supporting parallel bootstrap is defaulted to Parallel",
			cluster:  newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, nil),
			expected: newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, new(true)),
		},
		{
			name:     "unset enableParallelNodeOperations with a version newer than the minimal one supporting parallel bootstrap is defaulted to Parallel",
			cluster:  newScyllaCluster(unit.ScyllaDBImageAboveParallelBootstrapThresholdTag, nil),
			expected: newScyllaCluster(unit.ScyllaDBImageAboveParallelBootstrapThresholdTag, new(true)),
		},
		{
			name:     "unset enableParallelNodeOperations with an older version is left unset",
			cluster:  newScyllaCluster(unit.ScyllaDBImageBelowParallelBootstrapThresholdTag, nil),
			expected: newScyllaCluster(unit.ScyllaDBImageBelowParallelBootstrapThresholdTag, nil),
		},
		{
			// Per semver precedence a pre-release version is lower than the same version without one, matching how
			// an explicitly set Parallel enableParallelNodeOperations is validated.
			name:     "unset enableParallelNodeOperations with a pre-release of the minimal version supporting parallel bootstrap is left unset",
			cluster:  newScyllaCluster(unit.ScyllaDBImagePreReleaseOfParallelBootstrapThresholdTag, nil),
			expected: newScyllaCluster(unit.ScyllaDBImagePreReleaseOfParallelBootstrapThresholdTag, nil),
		},
		{
			name:     "unset enableParallelNodeOperations with an unparseable version is left unset",
			cluster:  newScyllaCluster("latest", nil),
			expected: newScyllaCluster("latest", nil),
		},
		{
			// The version is used verbatim as the image tag, so a digest pinned image makes it unparseable.
			name:     "unset enableParallelNodeOperations with a digest pinned version is left unset",
			cluster:  newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", nil),
			expected: newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", nil),
		},
		{
			// Mutating admission runs before the object is validated against the CRD schema, so the defaulter has to
			// tolerate a spec missing the fields it reads.
			name:     "unset enableParallelNodeOperations with an empty version is left unset",
			cluster:  newScyllaCluster("", nil),
			expected: newScyllaCluster("", nil),
		},
		{
			name:     "explicit false enableParallelNodeOperations is preserved",
			cluster:  newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, new(false)),
			expected: newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, new(false)),
		},
		{
			name:     "explicit true enableParallelNodeOperations is preserved",
			cluster:  newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, new(true)),
			expected: newScyllaCluster(unit.ScyllaDBImageAtParallelBootstrapThresholdTag, new(true)),
		},
		{
			// Rejecting a value the user set themselves is the validating webhook's job. The defaulter must not
			// silently turn an invalid request into a valid one.
			name:     "explicit true enableParallelNodeOperations with a version not supporting it is preserved",
			cluster:  newScyllaCluster(unit.ScyllaDBImageBelowParallelBootstrapThresholdTag, new(true)),
			expected: newScyllaCluster(unit.ScyllaDBImageBelowParallelBootstrapThresholdTag, new(true)),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.cluster.DeepCopy()
			defaulting.SetDefaultsScyllaCluster(got)

			if !cmp.Equal(tc.expected, got) {
				t.Errorf("expected and actual ScyllaClusters differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
