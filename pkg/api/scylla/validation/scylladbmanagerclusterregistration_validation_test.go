// Copyright (C) 2025 ScyllaDB

package validation

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaDBManagerClusterRegistration(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                               string
		scyllaDBManagerClusterRegistration *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expectedErrorList                  field.ErrorList
		expectedErrorString                string
	}{
		{
			name:                               "valid",
			scyllaDBManagerClusterRegistration: newValidScyllaDBManagerClusterRegistration(),
			expectedErrorList:                  nil,
			expectedErrorString:                ``,
		},
		{
			name: "empty internal.scylla-operator.scylladb.com/global-scylladb-manager label",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Labels = map[string]string{}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    `metadata.labels[internal.scylla-operator.scylladb.com/global-scylladb-manager]`,
					BadValue: ``,
					Detail:   ``,
				},
			},
			expectedErrorString: `metadata.labels[internal.scylla-operator.scylladb.com/global-scylladb-manager]: Required value`,
		},
		{
			name: "invalid internal.scylla-operator.scylladb.com/global-scylladb-manager label value",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Labels = map[string]string{
					"internal.scylla-operator.scylladb.com/global-scylladb-manager": "invalid",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    `metadata.labels[internal.scylla-operator.scylladb.com/global-scylladb-manager]`,
					BadValue: `invalid`,
					Detail:   `must be "true"`,
				},
			},
			expectedErrorString: `metadata.labels[internal.scylla-operator.scylladb.com/global-scylladb-manager]: Invalid value: "invalid": must be "true"`,
		},
		{
			name: "empty scyllaDBClusterRef name",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: ``,
					Detail:   ``,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Required value`,
		},
		{
			name: "invalid scyllaDBClusterRef name",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "-invalid",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: `-invalid`,
					Detail:   `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Invalid value: "-invalid": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "empty scyllaDBClusterRef kind",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.scyllaDBClusterRef.kind",
					BadValue: ``,
					Detail:   ``,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.kind: Required value`,
		},
		{
			name: "unsupported scyllaDBClusterRef kind",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaCluster",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.scyllaDBClusterRef.kind",
					BadValue: "ScyllaCluster",
					Detail:   `supported values: "ScyllaDBDatacenter", "ScyllaDBCluster"`,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.kind: Unsupported value: "ScyllaCluster": supported values: "ScyllaDBDatacenter", "ScyllaDBCluster"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerClusterRegistration(tc.scyllaDBManagerClusterRegistration)
			if !reflect.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			var errStr string
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaDBManagerClusterRegistrationUpdate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		new                 *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "identity",
			old:                 newValidScyllaDBManagerClusterRegistration(),
			new:                 newValidScyllaDBManagerClusterRegistration(),
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "scyllaDBClusterRef name changed",
			old: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "new-basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: "new-basic",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Invalid value: "new-basic": field is immutable`,
		},
		{
			name: "scyllaDBClusterRef kind changed",
			old: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBCluster",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.kind",
					BadValue: "ScyllaDBCluster",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.kind: Invalid value: "ScyllaDBCluster": field is immutable`,
		},
		{
			name: "internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override annotation changed",
			old:  newValidScyllaDBManagerClusterRegistration(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				if smcr.Annotations == nil {
					smcr.Annotations = map[string]string{}
				}
				smcr.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = "scylla/scylla"

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override]",
					BadValue: "scylla/scylla",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override]: Invalid value: "scylla/scylla": field is immutable`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerClusterRegistrationUpdate(tc.new, tc.old)
			if !reflect.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}

func newValidScyllaDBManagerClusterRegistration() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "uid",
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
	}
}
