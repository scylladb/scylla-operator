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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backup.location[0]",
					BadValue: "",
					Detail:   "must be in [dc:]<provider>:<bucket> format",
				},
			},
			expectedErrorString: `spec.backup.location[0]: Invalid value: "": must be in [dc:]<provider>:<bucket> format`,
		},
		{
			name: "disabled location validation via annotation",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						naming.ScyllaDBManagerTaskBackupLocationNoValidateAnnotation: "",
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
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with valid DC",
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
						DC: []string{
							"dc1",
							"!otherdc*",
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with invalid DC",
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
						DC: []string{
							"dc1,!otherdc*",
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
					Field:    "spec.backup.dc[0]",
					BadValue: "dc1,!otherdc*",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `spec.backup.dc[0]: Invalid value: "dc1,!otherdc*": must not contain commas, use multiple entries instead`,
		},
		{
			name: "backup with invalid DC and disable DC validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-dc-no-validate": "",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						DC: []string{
							"dc1,!otherdc*",
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "repair with valid DC",
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
						DC: []string{
							"dc1",
							"!otherdc*",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "repair with invalid DC",
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
						DC: []string{
							"dc1,!otherdc*",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.dc[0]",
					BadValue: "dc1,!otherdc*",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `spec.repair.dc[0]: Invalid value: "dc1,!otherdc*": must not contain commas, use multiple entries instead`,
		},
		{
			name: "repair with invalid DC and disable DC validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-dc-no-validate": "",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						DC: []string{
							"dc1,!otherdc*",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with valid keyspace",
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
						Keyspace: []string{
							"!keyspace.table_prefix_*",
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with invalid keyspace",
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
						Keyspace: []string{
							"!.table_prefix_*",
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
					Field:    "spec.backup.keyspace[0]",
					BadValue: "!.table_prefix_*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `spec.backup.keyspace[0]: Invalid value: "!.table_prefix_*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name: "backup with invalid keyspace and disable keyspace validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-keyspace-no-validate": "",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
					Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
						Keyspace: []string{
							"!.table_prefix_*",
						},
						Location: []string{
							"gcs:test",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "repair with valid keyspace",
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
						Keyspace: []string{
							"!keyspace.table_prefix_*",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "repair with invalid keyspace",
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
						Keyspace: []string{
							"!.table_prefix_*",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repair.keyspace[0]",
					BadValue: "!.table_prefix_*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `spec.repair.keyspace[0]: Invalid value: "!.table_prefix_*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name: "repair with invalid keyspace and disable keyspace validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repair",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-keyspace-no-validate": "",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
					Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
					Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
						Keyspace: []string{
							"!.table_prefix_*",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with valid rate limit",
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
						RateLimit: []string{
							"100",
							"dc1:50",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with invalid rate limit",
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
						RateLimit: []string{
							"dc1:1,dc2:2",
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
					Field:    "spec.backup.rateLimit[0]",
					BadValue: "dc1:1,dc2:2",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `spec.backup.rateLimit[0]: Invalid value: "dc1:1,dc2:2": must not contain commas, use multiple entries instead`,
		},
		{
			name: "backup with invalid rate limit and disable rate limit validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-rate-limit-no-validate": "",
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
						RateLimit: []string{
							"dc1:1,dc2:2",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with valid snapshot parallel",
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
						SnapshotParallel: []string{
							"5",
							"dc1:3",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with invalid snapshot parallel",
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
						SnapshotParallel: []string{
							"dc1:2,dc2:4",
						},
					},
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backup.snapshotParallel[0]",
					BadValue: "dc1:2,dc2:4",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `spec.backup.snapshotParallel[0]: Invalid value: "dc1:2,dc2:4": must not contain commas, use multiple entries instead`,
		},
		{
			name: "backup with invalid snapshot parallel and disable snapshot parallel validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-snapshot-parallel-no-validate": "",
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
						SnapshotParallel: []string{
							"dc1:2,dc2:4",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with valid upload parallel",
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
						UploadParallel: []string{
							"8",
							"dc1:6",
						},
					},
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "backup with invalid upload parallel",
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
						UploadParallel: []string{
							"dc1:3,dc2:5",
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
					Field:    "spec.backup.uploadParallel[0]",
					BadValue: "dc1:3,dc2:5",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `spec.backup.uploadParallel[0]: Invalid value: "dc1:3,dc2:5": must not contain commas, use multiple entries instead`,
		},
		{
			name: "backup with invalid upload parallel and disable upload parallel validation annotation set",
			scyllaDBManagerTask: &scyllav1alpha1.ScyllaDBManagerTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backup",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-upload-parallel-no-validate": "",
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
						UploadParallel: []string{
							"dc1:3,dc2:5",
						},
					},
				},
			},
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
						naming.ScyllaDBManagerTaskRepairParallelNoValidateAnnotation: "",
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
						naming.ScyllaDBManagerTaskBackupRetentionNoValidateAnnotation: "",
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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

func TestValidateLocation(t *testing.T) {
	t.Parallel()

	testFieldPath := field.NewPath("test")

	tests := []struct {
		name                string
		location            string
		fldPath             *field.Path
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid without dc",
			location:            "s3:bucket.domain",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "valid with dc",
			location:            "dc:s3:bucket.domain",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:     "empty",
			location: "",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "",
					Detail:   "must be in [dc:]<provider>:<bucket> format",
				},
			},
			expectedErrorString: `test: Invalid value: "": must be in [dc:]<provider>:<bucket> format`,
		},
		{
			name:     "empty bucket name",
			location: "s3:",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "s3:",
					Detail:   "must be in [dc:]<provider>:<bucket> format",
				},
			},
			expectedErrorString: `test: Invalid value: "s3:": must be in [dc:]<provider>:<bucket> format`,
		},
		{
			name:     "empty bucket name with dc",
			location: "dc:s3:",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc:s3:",
					Detail:   "must be in [dc:]<provider>:<bucket> format",
				},
			},
			expectedErrorString: `test: Invalid value: "dc:s3:": must be in [dc:]<provider>:<bucket> format`,
		},
		{
			name:     "invalid dc",
			location: "invalid dc:s3:bucket.domain",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "invalid dc:s3:bucket.domain",
					Detail:   "must be in [dc:]<provider>:<bucket> format",
				},
			},
			expectedErrorString: `test: Invalid value: "invalid dc:s3:bucket.domain": must be in [dc:]<provider>:<bucket> format`,
		},
		{
			name:     "invalid bucket name",
			location: "dc:s3:-bucket.domain",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc:s3:-bucket.domain",
					Detail:   `bucket name must be a valid DNS subdomain: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
				},
			},
			expectedErrorString: `test: Invalid value: "dc:s3:-bucket.domain": bucket name must be a valid DNS subdomain: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name:     "unsupported provider",
			location: "unsupported:bucket.domain",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeNotSupported,
					Field:    "test",
					BadValue: "unsupported:bucket.domain",
					Detail:   `unsupported provider, supported providers are: "azure", "gcs", "s3"`,
				},
			},
			expectedErrorString: `test: Unsupported value: "unsupported:bucket.domain": unsupported provider, supported providers are: "azure", "gcs", "s3"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validateLocation(tc.location, tc.fldPath)
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

func TestValidateDCFilter(t *testing.T) {
	t.Parallel()

	testFieldPath := field.NewPath("test")

	tests := []struct {
		name                string
		dcFilter            string
		fldPath             *field.Path
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "empty",
			dcFilter:            "",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "dc",
			dcFilter:            "dc",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "negated dc",
			dcFilter:            "!dc",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "dc with pattern",
			dcFilter:            "dc*",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "dc with pattern and negation",
			dcFilter:            "!dc*",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "dc with pattern, negation and whitespace",
			dcFilter:            "   !dc*",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:     "invalid glob pattern",
			dcFilter: "[invalid",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "[invalid",
					Detail:   "invalid glob pattern: unexpected end of input",
				},
			},
			expectedErrorString: `test: Invalid value: "[invalid": invalid glob pattern: unexpected end of input`,
		},
		{
			name:     "comma-separated list",
			dcFilter: "dc1,!dc2",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc1,!dc2",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `test: Invalid value: "dc1,!dc2": must not contain commas, use multiple entries instead`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validateDCFilter(tc.dcFilter, tc.fldPath)
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

func TestValidateKeyspaceFilter(t *testing.T) {
	t.Parallel()

	testFieldPath := field.NewPath("test")

	tests := []struct {
		name                string
		ksFilter            string
		fldPath             *field.Path
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "empty",
			ksFilter:            "",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "keyspace with no pattern",
			ksFilter:            "keyspace",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "keyspace with pattern",
			ksFilter:            "keyspace*",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:                "keyspace with pattern and negation",
			ksFilter:            "!keyspace*",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:     "no keyspace",
			ksFilter: ".table*",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: ".table*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `test: Invalid value: ".table*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name:     "no keyspace with whitespace",
			ksFilter: "   .table*",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "   .table*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `test: Invalid value: "   .table*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name:     "no keyspace with negation",
			ksFilter: "!.table*",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "!.table*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `test: Invalid value: "!.table*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name:     "no keyspace with negation and whitespace",
			ksFilter: "  !.table*",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "  !.table*",
					Detail:   "must contain keyspace name, e.g. <keyspace>.<table>",
				},
			},
			expectedErrorString: `test: Invalid value: "  !.table*": must contain keyspace name, e.g. <keyspace>.<table>`,
		},
		{
			name:     "invalid glob pattern in keyspace",
			ksFilter: "[invalid",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "[invalid",
					Detail:   "invalid glob pattern: unexpected end of input",
				},
			},
			expectedErrorString: `test: Invalid value: "[invalid": invalid glob pattern: unexpected end of input`,
		},
		{
			name:     "invalid glob pattern in table",
			ksFilter: "keyspace.[invalid",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "keyspace.[invalid",
					Detail:   "invalid glob pattern: unexpected end of input",
				},
			},
			expectedErrorString: `test: Invalid value: "keyspace.[invalid": invalid glob pattern: unexpected end of input`,
		},
		{
			name:     "comma-separated list",
			ksFilter: "keyspace,!keyspace.table_prefix_*",
			fldPath:  testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "keyspace,!keyspace.table_prefix_*",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `test: Invalid value: "keyspace,!keyspace.table_prefix_*": must not contain commas, use multiple entries instead`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validateKeyspaceFilter(tc.ksFilter, tc.fldPath)
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

func TestValidateDCLimit(t *testing.T) {
	t.Parallel()

	testFieldPath := field.NewPath("test")

	tests := []struct {
		name                string
		dcLimit             string
		fldPath             *field.Path
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:    "empty",
			dcLimit: "",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "",
					Detail:   "must be in [<dc>:]<limit> format",
				},
			},
			expectedErrorString: `test: Invalid value: "": must be in [<dc>:]<limit> format`,
		},
		{
			name:                "limit without dc",
			dcLimit:             "100",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:    "limit without dc in invalid format",
			dcLimit: "   100",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "   100",
					Detail:   "must be in [<dc>:]<limit> format",
				},
			},
			expectedErrorString: `test: Invalid value: "   100": must be in [<dc>:]<limit> format`,
		},
		{
			name:                "limit with dc",
			dcLimit:             "dc:100",
			fldPath:             testFieldPath,
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name:    "limit with empty dc",
			dcLimit: ":100",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: ":100",
					Detail:   "must be in [<dc>:]<limit> format",
				},
			},
			expectedErrorString: `test: Invalid value: ":100": must be in [<dc>:]<limit> format`,
		},
		{
			name:    "limit with dc in invalid format",
			dcLimit: "dc:  100",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc:  100",
					Detail:   "must be in [<dc>:]<limit> format",
				},
			},
			expectedErrorString: `test: Invalid value: "dc:  100": must be in [<dc>:]<limit> format`,
		},
		{
			name:    "limit out of range",
			dcLimit: "dc:9223372036854775808",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc:9223372036854775808",
					Detail:   `invalid limit value: strconv.ParseInt: parsing "9223372036854775808": value out of range`,
				},
			},
			expectedErrorString: `test: Invalid value: "dc:9223372036854775808": invalid limit value: strconv.ParseInt: parsing "9223372036854775808": value out of range`,
		},
		{
			name:    "comma-separated list",
			dcLimit: "dc1:1,dc2:2",
			fldPath: testFieldPath,
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "dc1:1,dc2:2",
					Detail:   "must not contain commas, use multiple entries instead",
				},
			},
			expectedErrorString: `test: Invalid value: "dc1:1,dc2:2": must not contain commas, use multiple entries instead`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validateDCLimit(tc.dcLimit, tc.fldPath)
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
