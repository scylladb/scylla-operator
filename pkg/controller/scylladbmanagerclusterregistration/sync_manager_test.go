// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_scyllaDBManagerClusterName(t *testing.T) {
	tt := []struct {
		name     string
		smcr     *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expected string
	}{
		{
			name: "basic",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "ScyllaDBDatacenter/basic",
		},
		{
			name: "name-override annotation, no global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "scylla/scylla",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/scylla",
		},
		{
			name: "no name-override annotation, global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/ScyllaDBDatacenter/basic",
		},
		{
			name: "name-override annotation, global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "scylla/scylla",
					},
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/scylla",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := scyllaDBManagerClusterName(tc.smcr)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager clusters differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_makeRequiredScyllaDBManagerCluster(t *testing.T) {
	t.Parallel()

	tt := []struct {
		testName    string
		name        string
		ownerUID    string
		host        string
		authToken   string
		expected    *managerclient.Cluster
		expectedErr error
	}{
		{
			testName:  "basic",
			name:      "basic",
			ownerUID:  "uid",
			host:      "host",
			authToken: "token",
			expected: &managerclient.Cluster{
				AuthToken:              "token",
				ForceNonSslSessionPort: true,
				ForceTLSDisabled:       true,
				Host:                   "host",
				ID:                     "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "SIOKM/7S4jFXpvNzOMLtDIvUqkRMUcGrB9di8k++vHum+7KQ01UGydXSjYeu0mjXxbQSk0ZpDWFsUaeq2DYR8g==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name:            "basic",
				Password:        "",
				Port:            0,
				SslUserCertFile: nil,
				SslUserKeyFile:  nil,
				Username:        "",
				WithoutRepair:   true,
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeRequiredScyllaDBManagerCluster(tc.name, tc.ownerUID, tc.host, tc.authToken)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager clusters differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}
