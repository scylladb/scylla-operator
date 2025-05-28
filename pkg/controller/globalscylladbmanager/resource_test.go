// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		sdc         *scyllav1alpha1.ScyllaDBDatacenter
		expected    *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expectedErr error
	}{
		{
			name: "basic",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
				},
			},
			expected: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "scylladbdatacenter-basic-3tpwb",
					Namespace:   "scylla",
					Annotations: map[string]string{},
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Kind: "ScyllaDBDatacenter",
						Name: "basic",
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "with name override annotation",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "name-override",
					},
				},
			},
			expected: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scylladbdatacenter-basic-3tpwb",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "name-override",
					},
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Kind: "ScyllaDBDatacenter",
						Name: "basic",
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(tc.sdc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDBManagerClusterRegistrations differ:\n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
