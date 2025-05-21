// Copyright (C) 2025 ScyllaDB

package validation

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaDBManagerTask(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		scyllaDBManagerTask *scyllav1alpha1.ScyllaDBManagerTask
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name: "valid repair",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   "Repair",
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "valid backup",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: "Backup",
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		// TODO: repair options for backup, backup options for repair
		{
			name: "empty backup location",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   "Backup",
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.backup.location",
					BadValue: "",
					Detail:   "location must not be empty",
				},
			},
			expectedErrorString: `spec.backup.location: Required value: location must not be empty`,
		},
		// TODO: no validate location annotation
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerTask(tc.scyllaDBManagerTask)
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

// TODO: update
