// Copyright (c) 2026 ScyllaDB.

package defaulting_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/defaulting"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// consistencyTestScyllaDBVersions covers both sides of every branch the defaulters decide on, including the versions
// they can't parse. The expectation is derived from the same predicate the defaulters use, so the table only has to
// cover the versions, not restate which of them support parallel bootstrap.
var consistencyTestScyllaDBVersions = []string{
	unit.ScyllaDBImageAtParallelBootstrapThresholdTag,
	unit.ScyllaDBImageAboveParallelBootstrapThresholdTag,
	unit.ScyllaDBImagePreReleaseOfParallelBootstrapThresholdTag,
	unit.ScyllaDBImageBelowParallelBootstrapThresholdTag,
	"latest",
	"",
}

// TestDefaultingIsConsistentWithValidation asserts that defaulting a bootstrapPolicy never adds a validation error.
// Parallel is only ever stamped for versions the validation accepts it for, which this covers from both sides.
// The mutating webhook runs before the validating one, so a value stamped by the former and rejected by the latter
// would fail a creation the user could have made themselves.
func TestDefaultingIsConsistentWithValidation(t *testing.T) {
	t.Parallel()

	t.Run("ScyllaCluster", func(t *testing.T) {
		t.Parallel()

		for _, version := range consistencyTestScyllaDBVersions {
			t.Run(version, func(t *testing.T) {
				t.Parallel()

				sc := unit.NewSingleRackCluster(3)
				// spec.version is used verbatim as the tag of the ScyllaDB image.
				sc.Spec.Version = version
				sc.Spec.BootstrapPolicy = nil

				errListBeforeDefaulting := validation.ValidateScyllaCluster(sc)

				defaulting.SetDefaultsScyllaCluster(sc)
				verifyDefaultedBootstrapPolicy(t, version, sc.Spec.BootstrapPolicy, scyllav1.BootstrapPolicyParallel)

				errListAfterDefaulting := validation.ValidateScyllaCluster(sc)
				if !reflect.DeepEqual(errListBeforeDefaulting, errListAfterDefaulting) {
					t.Errorf("defaulting bootstrapPolicy changed the validation result: %s", cmp.Diff(errListBeforeDefaulting, errListAfterDefaulting))
				}
			})
		}
	})

	t.Run("ScyllaDBDatacenter", func(t *testing.T) {
		t.Parallel()

		for _, version := range consistencyTestScyllaDBVersions {
			t.Run(version, func(t *testing.T) {
				t.Parallel()

				sdc := newConsistencyTestScyllaDBDatacenter(version)
				sdc.Spec.BootstrapPolicy = nil

				errListBeforeDefaulting := validation.ValidateScyllaDBDatacenter(sdc)

				defaulting.SetDefaultsScyllaDBDatacenter(sdc)
				verifyDefaultedBootstrapPolicy(t, version, sdc.Spec.BootstrapPolicy, scyllav1alpha1.BootstrapPolicyParallel)

				errListAfterDefaulting := validation.ValidateScyllaDBDatacenter(sdc)
				if !reflect.DeepEqual(errListBeforeDefaulting, errListAfterDefaulting) {
					t.Errorf("defaulting bootstrapPolicy changed the validation result: %s", cmp.Diff(errListBeforeDefaulting, errListAfterDefaulting))
				}
			})
		}
	})
}

// verifyDefaultedBootstrapPolicy asserts that defaulting stamped Parallel exactly for the ScyllaDB versions supporting
// parallel bootstrap, and left bootstrapPolicy unset for every other one. Sequential is never stamped, so that objects
// whose owners never made a choice keep resolving an unset bootstrapPolicy rather than being pinned to today's
// resolution of it.
func verifyDefaultedBootstrapPolicy[P ~string](t *testing.T, scyllaDBVersion string, got *P, parallel P) {
	t.Helper()

	if !semver.SupportsParallelBootstrap(scyllaDBVersion) {
		if got != nil {
			t.Errorf("expected bootstrapPolicy to be left unset, got %q", *got)
		}
		return
	}

	if got == nil {
		t.Errorf("expected bootstrapPolicy to be stamped with %q, got none", parallel)
		return
	}

	if *got != parallel {
		t.Errorf("expected bootstrapPolicy to be stamped with %q, got %q", parallel, *got)
	}
}

// newConsistencyTestScyllaDBDatacenter returns a ScyllaDBDatacenter whose only source of validation errors can be the
// ScyllaDB image, so that the comparison of the validation results isolates the effect of the defaulted field.
func newConsistencyTestScyllaDBDatacenter(scyllaDBVersion string) *scyllav1alpha1.ScyllaDBDatacenter {
	image := unit.ScyllaDBImageRepository
	if len(scyllaDBVersion) != 0 {
		image = image + ":" + scyllaDBVersion
	}

	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName:    "basic",
			DatacenterName: new("dc"),
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image: image,
			},
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "rack",
					RackTemplate: scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							Storage: &scyllav1alpha1.StorageOptions{
								Capacity: "1Gi",
							},
						},
					},
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
			Racks: []scyllav1alpha1.RackStatus{},
		},
	}
}
