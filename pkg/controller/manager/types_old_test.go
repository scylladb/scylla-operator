// Copyright (C) 2024 ScyllaDB

package manager

import (
	"reflect"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/equality"
)

var (
	validDate     = "2024-03-20T14:49:33.590Z"
	validDateTime = pointer.Ptr(helpers.Must(strfmt.ParseDateTime(validDate)))
)

func TestRepairTaskSpec_ToManager(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		repairTaskSpec *RepairTaskSpec
		expected       *managerclient.Task
		expectedError  error
	}{
		{
			name: "fields and properties are propagated with NumRetries",
			repairTaskSpec: &RepairTaskSpec{
				TaskSpec: scyllav1.TaskSpec{
					Name: "repair_task_name",
					SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
						StartDate:  pointer.Ptr(validDate),
						NumRetries: pointer.Ptr[int64](3),
						Interval:   pointer.Ptr("7d"),
						Cron:       pointer.Ptr("0 23 * * SAT"),
						Timezone:   pointer.Ptr("CET"),
					},
				},
				DC:                  []string{"us-east1"},
				FailFast:            false,
				Intensity:           "1",
				Parallel:            1,
				Keyspace:            []string{"test"},
				SmallTableThreshold: "1GiB",
				Host:                pointer.Ptr("10.0.0.1"),
			},
			expected: &managerclient.Task{
				ClusterID: "",
				Enabled:   true,
				ID:        "",
				Name:      "repair_task_name",
				Properties: map[string]interface{}{
					"dc": []string{
						"us-east1",
					},
					"intensity": float64(1),
					"parallel":  int64(1),
					"keyspace": []string{
						"test",
					},
					"small_table_threshold": int64(1073741824),
					"host":                  "10.0.0.1",
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "7d",
					NumRetries: 3,
					StartDate:  validDateTime,
					Timezone:   "CET",
				},
				Type: managerclient.RepairTask,
			},
			expectedError: nil,
		},
		{
			name: "fields and properties are propagated with FailFast",
			repairTaskSpec: &RepairTaskSpec{
				TaskSpec: scyllav1.TaskSpec{
					Name: "repair_task_name",
					SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
						StartDate:  pointer.Ptr(validDate),
						NumRetries: pointer.Ptr[int64](3),
						Interval:   pointer.Ptr("7d"),
						Cron:       pointer.Ptr("0 23 * * SAT"),
						Timezone:   pointer.Ptr("CET"),
					},
				},
				DC:                  []string{"us-east1"},
				FailFast:            true,
				Intensity:           "1",
				Parallel:            1,
				Keyspace:            []string{"test"},
				SmallTableThreshold: "1GiB",
				Host:                pointer.Ptr("10.0.0.1"),
			},
			expected: &managerclient.Task{
				ClusterID: "",
				Enabled:   true,
				ID:        "",
				Name:      "repair_task_name",
				Properties: map[string]interface{}{
					"dc": []string{
						"us-east1",
					},
					"fail_fast": true,
					"intensity": float64(1),
					"parallel":  int64(1),
					"keyspace": []string{
						"test",
					},
					"small_table_threshold": int64(1073741824),
					"host":                  "10.0.0.1",
				},
				Schedule: &managerclient.Schedule{
					Interval:   "7d",
					Cron:       "0 23 * * SAT",
					NumRetries: 0,
					StartDate:  validDateTime,
					Timezone:   "CET",
				},
				Type: managerclient.RepairTask,
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			managerClientTask, err := tc.repairTaskSpec.ToManager()
			if !equality.Semantic.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(managerClientTask, tc.expected) {
				t.Errorf("expected and got manager client tasks differ: %s", cmp.Diff(tc.expected, managerClientTask))
			}
		})
	}
}

func TestRepairTask_FromManager(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		managerTask   *managerclient.TaskListItem
		expected      *RepairTaskStatus
		expectedError error
	}{
		{
			name: "fields and properties are propagated",
			managerTask: &managerclient.TaskListItem{
				ClusterID:      "cluster_id",
				Name:           "repair_task_name",
				Enabled:        true,
				ErrorCount:     1,
				ID:             "repair_task_id",
				LastError:      validDateTime,
				LastSuccess:    validDateTime,
				NextActivation: validDateTime,
				Properties: map[string]interface{}{
					"dc": []string{
						"us-east1",
					},
					"fail_fast": true,
					"intensity": "1",
					"parallel":  1,
					"keyspace": []string{
						"test",
					},
					"small_table_threshold": "1073741824",
					"host":                  "10.0.0.1",
				},
				Retry: 1,
				Schedule: &managerclient.Schedule{
					Interval:   "7d",
					Cron:       "0 23 * * SAT",
					NumRetries: 3,
					StartDate:  validDateTime,
					Timezone:   "CET",
				},
				Status:       managerclient.TaskStatusRunning,
				SuccessCount: 0,
				Suspended:    false,
				Type:         managerclient.RepairTask,
			},
			expected: &RepairTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					Name: "repair_task_name",
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr(validDate),
						NumRetries: pointer.Ptr[int64](3),
						Interval:   pointer.Ptr("7d"),
						Cron:       pointer.Ptr("0 23 * * SAT"),
						Timezone:   pointer.Ptr("CET"),
					},
					ID:    pointer.Ptr("repair_task_id"),
					Error: nil,
				},
				DC:                  []string{"us-east1"},
				FailFast:            pointer.Ptr(true),
				Intensity:           pointer.Ptr("1"),
				Parallel:            pointer.Ptr[int64](1),
				Keyspace:            []string{"test"},
				SmallTableThreshold: pointer.Ptr("1073741824"),
				Host:                pointer.Ptr("10.0.0.1"),
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rts, err := NewRepairStatusFromManager(tc.managerTask)
			if !equality.Semantic.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(rts, tc.expected) {
				t.Errorf("expected and got repair task statuses differ: %s", cmp.Diff(tc.expected, rts))
			}
		})
	}
}

func TestBackupTask_ToManager(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		backupTaskSpec *BackupTaskSpec
		expected       *managerclient.Task
		expectedError  error
	}{
		{
			name: "fields and properties are propagated",
			backupTaskSpec: &BackupTaskSpec{
				TaskSpec: scyllav1.TaskSpec{
					Name: "backup_task_name",
					SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
						StartDate:  pointer.Ptr(validDate),
						NumRetries: pointer.Ptr[int64](3),
						Interval:   pointer.Ptr("7d"),
						Cron:       pointer.Ptr("0 23 * * SAT"),
						Timezone:   pointer.Ptr("CET"),
					},
				},
				DC:       []string{"us-east1"},
				Keyspace: []string{"test"},
				Location: []string{
					"gcs:test",
				},
				RateLimit: []string{
					"10",
					"us-east1:100",
				},
				Retention: 1,
				SnapshotParallel: []string{
					"10",
					"us-east1:100",
				},
				UploadParallel: []string{
					"10",
					"us-east1:100",
				},
			},
			expected: &managerclient.Task{
				ClusterID: "",
				Enabled:   true,
				ID:        "",
				Name:      "backup_task_name",
				Properties: map[string]interface{}{
					"location": []string{
						"gcs:test",
					},
					"keyspace":  []string{"test"},
					"retention": int64(1),
					"dc":        []string{"us-east1"},
					"rate_limit": []string{
						"10",
						"us-east1:100",
					},
					"snapshot_parallel": []string{
						"10",
						"us-east1:100",
					},
					"upload_parallel": []string{
						"10",
						"us-east1:100",
					},
				},
				Schedule: &managerclient.Schedule{
					Interval:   "7d",
					Cron:       "0 23 * * SAT",
					NumRetries: 3,
					StartDate:  validDateTime,
					Timezone:   "CET",
				},
				Type: managerclient.BackupTask,
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			managerClientTask, err := tc.backupTaskSpec.ToManager()
			if !equality.Semantic.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(managerClientTask, tc.expected) {
				t.Errorf("expected and got manager client tasks differ: %s", cmp.Diff(tc.expected, managerClientTask))
			}
		})
	}
}

func TestBackupTask_FromManager(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		managerTask   *managerclient.TaskListItem
		expected      *BackupTaskStatus
		expectedError error
	}{
		{
			name: "fields and properties are propagated",
			managerTask: &managerclient.TaskListItem{
				ClusterID:      "cluster_id",
				Enabled:        true,
				ErrorCount:     1,
				ID:             "backup_task_id",
				LastError:      validDateTime,
				LastSuccess:    validDateTime,
				Name:           "backup_task_name",
				NextActivation: validDateTime,
				Properties: map[string]interface{}{
					"location": []string{
						"gcs:test",
					},
					"keyspace":       []string{"test"},
					"retention":      1,
					"retention_days": 3,
					"dc":             []string{"us-east1"},
					"rate_limit": []string{
						"10",
						"us-east1:100",
					},
					"snapshot_parallel": []string{
						"10",
						"us-east1:100",
					},
					"upload_parallel": []string{
						"10",
						"us-east1:100",
					},
					"purge_only": false,
				},
				Retry: 1,
				Schedule: &managerclient.Schedule{
					Interval:   "7d",
					Cron:       "0 23 * * SAT",
					NumRetries: 3,
					StartDate:  validDateTime,
					Timezone:   "CET",
				},
				Status:       managerclient.TaskStatusRunning,
				SuccessCount: 0,
				Suspended:    false,
				Type:         managerclient.BackupTask,
			},
			expected: &BackupTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					Name: "backup_task_name",
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr(validDate),
						NumRetries: pointer.Ptr[int64](3),
						Interval:   pointer.Ptr("7d"),
						Cron:       pointer.Ptr("0 23 * * SAT"),
						Timezone:   pointer.Ptr("CET"),
					},
					ID:    pointer.Ptr("backup_task_id"),
					Error: nil,
				},
				DC:               []string{"us-east1"},
				Keyspace:         []string{"test"},
				Location:         []string{"gcs:test"},
				RateLimit:        []string{"10", "us-east1:100"},
				Retention:        pointer.Ptr[int64](1),
				SnapshotParallel: []string{"10", "us-east1:100"},
				UploadParallel:   []string{"10", "us-east1:100"},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bts, err := NewBackupStatusFromManager(tc.managerTask)
			if !equality.Semantic.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(bts, tc.expected) {
				t.Errorf("expected and got backup task statuses differ: %s", cmp.Diff(tc.expected, bts))
			}
		})
	}
}
