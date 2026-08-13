// Copyright (c) 2026 ScyllaDB.

package defaulting_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/defaulting"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetDefaultsScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	newScyllaDBDatacenter := func(image string, bootstrapPolicy *scyllav1alpha1.BootstrapPolicy) *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic",
				Namespace: "test",
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName: "basic",
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: image,
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "rack",
					},
				},
				BootstrapPolicy: bootstrapPolicy,
			},
		}
	}

	tt := []struct {
		name       string
		datacenter *scyllav1alpha1.ScyllaDBDatacenter
		expected   *scyllav1alpha1.ScyllaDBDatacenter
	}{
		{
			name:       "unset bootstrapPolicy with the minimal version supporting parallel bootstrap is defaulted to Parallel",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
		},
		{
			name:       "unset bootstrapPolicy with a version newer than the minimal one supporting parallel bootstrap is defaulted to Parallel",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageAboveParallelBootstrapThreshold, nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageAboveParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
		},
		{
			name:       "unset bootstrapPolicy with an older version is left unset",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageBelowParallelBootstrapThreshold, nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageBelowParallelBootstrapThreshold, nil),
		},
		{
			// Per semver precedence a pre-release version is lower than the same version without one, matching how
			// an explicitly set Parallel bootstrapPolicy is validated.
			name:       "unset bootstrapPolicy with a pre-release of the minimal version supporting parallel bootstrap is left unset",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImagePreReleaseOfParallelBootstrapThreshold, nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImagePreReleaseOfParallelBootstrapThreshold, nil),
		},
		{
			name:       "unset bootstrapPolicy with an unparseable tag is left unset",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImage, nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImage, nil),
		},
		{
			// A digest pinned image carries no tag to determine the version from.
			name:       "unset bootstrapPolicy with a digest pinned image is left unset",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageRepository+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageRepository+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", nil),
		},
		{
			name:       "unset bootstrapPolicy with a tagged and digest pinned image is defaulted from the tag",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", nil),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold+"@sha256:d450d2d8636ab7b511b68180b4c535bf2294d0d6427adf11fc7f4bddfdbff35a", new(scyllav1alpha1.BootstrapPolicyParallel)),
		},
		{
			// Mutating admission runs before the object is validated against the CRD schema, so the defaulter has to
			// tolerate a spec missing the fields it reads.
			name:       "unset bootstrapPolicy with an empty image is left unset",
			datacenter: newScyllaDBDatacenter("", nil),
			expected:   newScyllaDBDatacenter("", nil),
		},
		{
			name:       "explicit Sequential bootstrapPolicy is preserved",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicySequential)),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicySequential)),
		},
		{
			name:       "explicit Parallel bootstrapPolicy is preserved",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageAtParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
		},
		{
			// Rejecting a value the user set themselves is the validating webhook's job. The defaulter must not
			// silently turn an invalid request into a valid one.
			name:       "explicit Parallel bootstrapPolicy with a version not supporting it is preserved",
			datacenter: newScyllaDBDatacenter(unit.ScyllaDBImageBelowParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
			expected:   newScyllaDBDatacenter(unit.ScyllaDBImageBelowParallelBootstrapThreshold, new(scyllav1alpha1.BootstrapPolicyParallel)),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.datacenter.DeepCopy()
			defaulting.SetDefaultsScyllaDBDatacenter(got)

			if !cmp.Equal(tc.expected, got) {
				t.Errorf("expected and actual ScyllaDBDatacenters differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
