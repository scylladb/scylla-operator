// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

var (
	validTimeValue = "2024-03-20T14:49:33.590Z"
	validTime      = helpers.Must(time.Parse(time.RFC3339, validTimeValue))
)

func Test_makeRequiredScyllaDBManagerClientTask(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		smt             *scyllav1alpha1.ScyllaDBManagerTask
		clusterID       string
		overrideOptions []scyllaDBManagerClientTaskOverrideOption
		expected        *managerclient.Task
		expectedErr     error
	}{
		{
			name:            "basic backup",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "p2WwP2A/kXBvlMZMordNbf2iKDUvZpSBtUViRMA/s8t+jOAgpZtTevJ8LfudtdfGUz+6CVoZYJJVnxsVT5rXQg==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "basic repair",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "JAEIVbaryJJMMKzZgsuM8621a7pfe3X3rTT/CuIw9rjF4i+w3zagkB+uRMypYYofhmlWPYLsu5eKJG/hf88FMA==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeScyllaDBManagerClientTask(tc.smt, tc.clusterID, tc.overrideOptions...)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager client tasks differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_makeRequiredScyllaDBManagerClientTaskWithManagedHashFunc(t *testing.T) {
	t.Parallel()

	const mockManagedHash = "mock-managed-hash"
	getMockManagedHash := func(_ *managerclient.Task) (string, error) {
		return mockManagedHash, nil
	}

	tt := []struct {
		name            string
		smt             *scyllav1alpha1.ScyllaDBManagerTask
		clusterID       string
		managedHashFunc func(*managerclient.Task) (string, error)
		overrideOptions []scyllaDBManagerClientTaskOverrideOption
		expected        *managerclient.Task
		expectedErr     error
	}{
		{
			name:            "backup, sdc ref",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, without optional fields",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				smt.Spec.Backup = &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					Location: []string{"s3:test"},
				}

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"location": []string{"s3:test"},
				},
				Schedule: &managerclient.Schedule{},
				Tags:     nil,
				Type:     "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with name override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskNameOverrideAnnotation, "override")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "override",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with interval override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation, "7d")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "7d",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with valid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with invalid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "backup, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "backup, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with 'now+duration' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+3d2h10m")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with valid time schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "backup, sdc ref, without startDate override annotation and with start date retention override option",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with schedule timezone override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation, "CET")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "CET",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "repair, sdc ref",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, without optional fields",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				smt.Spec.Repair = &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{}

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name:       "repair",
				Properties: map[string]any{},
				Schedule:   &managerclient.Schedule{},
				Tags:       nil,
				Type:       "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with name override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskNameOverrideAnnotation, "override")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "override",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid intensity override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation, "0.5")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(0.5),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid intensity override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation, "invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse intensity override: %w", &strconv.NumError{Func: "ParseFloat", Num: "invalid", Err: strconv.ErrSyntax})})),
		},
		{
			name: "repair, sdc ref, with interval override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation, "7d")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "7d",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "repair, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "repair, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with 'now+duration' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+3d2h10m")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid time schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name:            "repair, sdc ref, without startDate override annotation and with start date retention override option",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with schedule timezone override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation, "CET")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "CET",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid small table threshold override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation, "512MiB")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             int64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(536870912),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid small table threshold override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation, "invalid size")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse small table threshold override: %w", fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "invalid size", "B,KiB,MiB,GiB,TiB,PiB,EiB"))})),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeScyllaDBManagerClientTaskWithManagedHashFunc(tc.smt, tc.clusterID, tc.managedHashFunc, tc.overrideOptions...)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager client tasks differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "scylla",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: "basic",
			},
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
			Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 23 * * SAT"),
					NumRetries: pointer.Ptr[int64](3),
					StartDate:  pointer.Ptr(metav1.NewTime(validTime)),
				},
				DC:       []string{"dc1", "!otherdc*"},
				Keyspace: []string{"keyspace", "!keyspace.table_prefix_*"},
				Location: []string{"gcs:test"},
				RateLimit: []string{
					"dc1:1",
					"2",
				},
				Retention: pointer.Ptr[int64](3),
				SnapshotParallel: []string{
					"dc1:2",
					"3",
				},
				UploadParallel: []string{
					"dc1:3",
					"4",
				},
			},
		},
	}
}

func newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repair",
			Namespace: "scylla",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: "basic",
			},
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
			Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 23 * * SAT"),
					NumRetries: pointer.Ptr[int64](3),
					StartDate:  pointer.Ptr(metav1.NewTime(validTime)),
				},
				DC:                  []string{"dc1", "!otherdc*"},
				Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
				FailFast:            pointer.Ptr(true),
				Host:                pointer.Ptr("10.0.0.1"),
				IgnoreDownHosts:     pointer.Ptr(false),
				Intensity:           pointer.Ptr[int64](1),
				Parallel:            pointer.Ptr[int64](1),
				SmallTableThreshold: pointer.Ptr(resource.MustParse("1Gi")),
			},
		},
	}
}

func Test_parseByteCount(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		byteSize    string
		expected    int64
		expectedErr error
	}{
		{
			name:        "B unit",
			byteSize:    "1024B",
			expected:    1024,
			expectedErr: nil,
		},
		{
			name:        "KiB unit",
			byteSize:    "1KiB",
			expected:    1024,
			expectedErr: nil,
		},
		{
			name:        "MiB unit",
			byteSize:    "2.5MiB",
			expected:    2621440,
			expectedErr: nil,
		},
		{
			name:        "GiB unit",
			byteSize:    "1GiB",
			expected:    1073741824,
			expectedErr: nil,
		},
		{
			name:        "TiB unit",
			byteSize:    "1TiB",
			expected:    1099511627776,
			expectedErr: nil,
		},
		{
			name:        "invalid format - no unit",
			byteSize:    "1024",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "1024", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
		{
			name:        "invalid format - invalid unit",
			byteSize:    "1024KB",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "1024KB", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
		{
			name:        "invalid format - invalid number",
			byteSize:    "abc GiB",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "abc GiB", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseByteCount(tc.byteSize)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err))
			}

			if result != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

func Test_parseStartDate(t *testing.T) {
	t.Parallel()

	mockNowFunc := func() time.Time {
		return time.Date(2025, 5, 29, 13, 26, 5, 0, time.UTC)
	}

	tt := []struct {
		name        string
		startDate   string
		nowFunc     func() time.Time
		expected    strfmt.DateTime
		expectedErr error
	}{
		{
			name:        "exact 'now'",
			startDate:   "now",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: nil,
		},
		{
			name:        "now plus 1 hour",
			startDate:   "now+1h",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2025, 5, 29, 14, 26, 5, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "now plus complex duration",
			startDate:   "now+3d2h10m",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2025, 6, 01, 15, 36, 5, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "now plus zero duration",
			startDate:   "now+0h",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: nil,
		},
		{
			name:        "now plus invalid duration",
			startDate:   "now+invalid",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")),
		},
		{
			name:        "RFC3339 timestamp",
			startDate:   "2023-05-08T17:24:00Z",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2023, 5, 8, 17, 24, 0, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "RFC3339 timestamp with milliseconds",
			startDate:   "2023-05-08T17:24:00.123Z",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2023, 5, 8, 17, 24, 0, 123000000, time.UTC)),
			expectedErr: nil,
		},
		{
			name:      "invalid timestamp format",
			startDate: "2023-05-08 17:24:00",
			nowFunc:   mockNowFunc,
			expected:  strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse time: %w", &time.ParseError{
				Layout:     time.RFC3339,
				Value:      "2023-05-08 17:24:00",
				LayoutElem: "T",
				ValueElem:  " 17:24:00",
				Message:    "",
			}),
		},
		{
			name:      "empty string",
			startDate: "",
			nowFunc:   mockNowFunc,
			expected:  strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse time: %w", &time.ParseError{
				Layout:     time.RFC3339,
				Value:      "",
				LayoutElem: "2006",
				ValueElem:  "",
				Message:    "",
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseStartDate(tc.startDate, tc.nowFunc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}
