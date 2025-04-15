// Copyright (C) 2025 ScyllaDB

package validation

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/resource"
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
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
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
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
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
		{
			name: "unsupported task type",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "random",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: "Unsupported",
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.type",
					BadValue: scyllav1alpha1.ScyllaDBManagerTaskType("Unsupported"),
					Detail:   `supported values: "Backup", "Repair"`,
				},
			},
			expectedErrorString: `spec.type: Unsupported value: "Unsupported": supported values: "Backup", "Repair"`,
		},

		{
			name: "missing required options for repair type",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.repair",
					BadValue: "",
					Detail:   `repair options are required when task type is "Repair"`,
				},
			},
			expectedErrorString: `spec.repair: Required value: repair options are required when task type is "Repair"`,
		},
		{
			name: "missing required options for backup type",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.backup",
					BadValue: "",
					Detail:   `backup options are required when task type is "Backup"`,
				},
			},
			expectedErrorString: `spec.backup: Required value: backup options are required when task type is "Backup"`,
		},
		{
			name: "repair options for backup task type",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.repair",
					BadValue: "",
					Detail:   `repair options are forbidden when task type is not "Repair"`,
				},
			},
			expectedErrorString: `spec.repair: Forbidden: repair options are forbidden when task type is not "Repair"`,
		},
		{
			name: "backup options for repair task type",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.backup",
					BadValue: "",
					Detail:   `backup options are forbidden when task type is not "Backup"`,
				},
			},
			expectedErrorString: `spec.backup: Forbidden: backup options are forbidden when task type is not "Backup"`,
		},
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
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{},
					},
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
		{
			name: "empty backup location item",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.backup.location[0]",
					BadValue: "",
					Detail:   "location must not be empty",
				},
			},
			expectedErrorString: `spec.backup.location[0]: Required value: location must not be empty`,
		},
		{
			name: "disabled location validation via annotation",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskBackupLocationDisableValidationAnnotation: naming.AnnotationValueTrue,
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid repair with negative intensity",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Intensity: pointer.Ptr[int64](-1),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.intensity",
					BadValue: int64(-1),
					Detail:   "can't be negative",
				},
			},
			expectedErrorString: `spec.repair.intensity: Invalid value: -1: can't be negative`,
		},
		{
			name: "valid repair intensity override",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation: "0.5",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid repair intensity override",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override]",
					BadValue: "invalid",
					Detail:   "must be a float",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override]: Invalid value: "invalid": must be a float`,
		},
		{
			name: "valid repair with non-negative parallel value",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Parallel: pointer.Ptr[int64](5),
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid repair with negative parallel value",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Parallel: pointer.Ptr[int64](-1),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.parallel",
					BadValue: int64(-1),
					Detail:   "can't be negative",
				},
			},
			expectedErrorString: `spec.repair.parallel: Invalid value: -1: can't be negative`,
		},
		{
			name: "valid repair with negative parallel value and parallel validation disabled annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairParallelDisableValidationAnnotation: naming.AnnotationValueTrue,
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Parallel: pointer.Ptr[int64](-1),
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "valid repair with small table threshold",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						SmallTableThreshold: pointer.Ptr(resource.MustParse("1Gi")),
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid repair with negative small table threshold",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						SmallTableThreshold: pointer.Ptr(resource.MustParse("-1Gi")),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.smallTableThreshold",
					BadValue: "-1Gi",
					Detail:   "must be greater than zero",
				},
			},
			expectedErrorString: `spec.repair.smallTableThreshold: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid repair with non-integer small table threshold",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						SmallTableThreshold: pointer.Ptr(resource.MustParse("100m")),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.smallTableThreshold",
					BadValue: "100m",
					Detail:   "fractional byte value is invalid, must be an integer",
				},
			},
			expectedErrorString: `spec.repair.smallTableThreshold: Invalid value: "100m": fractional byte value is invalid, must be an integer`,
		},
		{
			name: "repair small table threshold override without options small table threshold",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation: "200MiB",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "repair small table threshold override with options small table threshold",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation: "200MiB",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						SmallTableThreshold: pointer.Ptr(resource.MustParse("100Mi")),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-small-table-threshold-override]",
					BadValue: "",
					Detail:   "can't be used together with repair options' smallTableThreshold",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-small-table-threshold-override]: Forbidden: can't be used together with repair options' smallTableThreshold`,
		},
		{
			name: "repair intensity override with options intensity",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation: "0.5",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Intensity: pointer.Ptr[int64](1),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override]",
					BadValue: "",
					Detail:   "can't be used together with repair options' intensity",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override]: Forbidden: can't be used together with repair options' intensity`,
		},
		{
			name: "valid backup with non-negative retention",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
						Retention: pointer.Ptr[int64](7),
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid backup with negative retention",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
						Retention: pointer.Ptr[int64](-1),
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backup.retention",
					BadValue: int64(-1),
					Detail:   "can't be negative",
				},
			},
			expectedErrorString: `spec.backup.retention: Invalid value: -1: can't be negative`,
		},
		{
			name: "valid backup with negative retention but retention validation disabled annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskBackupRetentionDisableValidationAnnotation: naming.AnnotationValueTrue,
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
						Retention: pointer.Ptr[int64](-1),
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "valid name override annotation",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskNameOverrideAnnotation: "valid-name",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid name override annotation",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskNameOverrideAnnotation: "invalid-with-trailing-dash-",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override]",
					BadValue: "invalid-with-trailing-dash-",
					Detail:   `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override]: Invalid value: "invalid-with-trailing-dash-": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid interval override without cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid interval override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]",
					BadValue: "invalid",
					Detail:   "valid units are d, h, m, s",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]: Invalid value: "invalid": valid units are d, h, m, s`,
		},
		{
			name: "non-zero interval override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "24h",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]",
					BadValue: "",
					Detail:   "can't be non-zero when cron is specified",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]: Forbidden: can't be non-zero when cron is specified`,
		},
		{
			name: "zero interval override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "0s",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid interval override with backup cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]",
					BadValue: "invalid",
					Detail:   "valid units are d, h, m, s",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]: Invalid value: "invalid": valid units are d, h, m, s`,
		},
		{
			name: "non-zero interval override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "24h",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]",
					BadValue: "",
					Detail:   "can't be non-zero when cron is specified",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override]: Forbidden: can't be non-zero when cron is specified`,
		},
		{
			name: "zero interval override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation: "0s",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "valid timezone override with repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation: "Europe/Warsaw",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "valid timezone override with backup cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation: "Europe/Warsaw",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: ``,
		},
		{
			name: "invalid timezone override with repair task",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override]",
					BadValue: "invalid",
					Detail:   "unknown time zone invalid",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override]: Invalid value: "invalid": unknown time zone invalid`,
		},
		{
			name: "invalid timezone override with backup task",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation: "invalid",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override]",
					BadValue: "invalid",
					Detail:   "unknown time zone invalid",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override]: Invalid value: "invalid": unknown time zone invalid`,
		},
		{
			name: "unallowed TZ in repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("TZ=Europe/Warsaw 0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.cron",
					BadValue: pointer.Ptr("TZ=Europe/Warsaw 0 0 * * *"),
					Detail:   "TZ and CRON_TZ prefixes are forbidden",
				},
			},
			expectedErrorString: `spec.repair.cron: Invalid value: "TZ=Europe/Warsaw 0 0 * * *": TZ and CRON_TZ prefixes are forbidden`,
		},
		{
			name: "unallowed TZ in backup cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("TZ=Europe/Warsaw 0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backup.cron",
					BadValue: pointer.Ptr("TZ=Europe/Warsaw 0 0 * * *"),
					Detail:   "TZ and CRON_TZ prefixes are forbidden",
				},
			},
			expectedErrorString: `spec.backup.cron: Invalid value: "TZ=Europe/Warsaw 0 0 * * *": TZ and CRON_TZ prefixes are forbidden`,
		},
		{
			name: "unallowed CRON_TZ in repair cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 0 * * *"),
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.cron",
					BadValue: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 0 * * *"),
					Detail:   "TZ and CRON_TZ prefixes are forbidden",
				},
			},
			expectedErrorString: `spec.repair.cron: Invalid value: "CRON_TZ=Europe/Warsaw 0 0 * * *": TZ and CRON_TZ prefixes are forbidden`,
		},
		{
			name: "unallowed CRON_TZ in backup cron",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
							Cron: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 0 * * *"),
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backup.cron",
					BadValue: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 0 * * *"),
					Detail:   "TZ and CRON_TZ prefixes are forbidden",
				},
			},
			expectedErrorString: `spec.backup.cron: Invalid value: "CRON_TZ=Europe/Warsaw 0 0 * * *": TZ and CRON_TZ prefixes are forbidden`,
		},
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

func TestValidateScyllaDBManagerTaskUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDBManagerTask
		new                 *scyllav1alpha1.ScyllaDBManagerTask
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name: "identity repair",
			old: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			new: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "identity backup",
			old: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			new: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "invalid change in immutable task type",
			old: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "basic",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			new: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "basic",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.type",
					BadValue: scyllav1alpha1.ScyllaDBManagerTaskType("Backup"),
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `spec.type: Invalid value: "Backup": field is immutable`,
		},
		{
			name: "invalid change in immutable name override annotation",
			old: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskNameOverrideAnnotation: "valid-name",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			new: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskNameOverrideAnnotation: "different-valid-name",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override]",
					BadValue: "different-valid-name",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override]: Invalid value: "different-valid-name": field is immutable`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerTaskUpdate(tc.new, tc.old)
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
