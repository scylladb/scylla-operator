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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

var (
	validTimeValue = "2024-03-20T14:49:33.590Z"
	validTime      = helpers.Must(time.Parse(time.RFC3339, validTimeValue))
)

func Test_makeRequiredScyllaDBManagerClientTask(t *testing.T) {
	t.Parallel()

	newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef := func() *scyllav1alpha1.ScyllaDBManagerTask {
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

	newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef := func() *scyllav1alpha1.ScyllaDBManagerTask {
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
					Intensity:           pointer.Ptr[int64](1),
					Parallel:            pointer.Ptr[int64](1),
					SmallTableThreshold: pointer.Ptr(resource.MustParse("1Gi")),
				},
			},
		}
	}

	tt := []struct {
		name            string
		smt             *scyllav1alpha1.ScyllaDBManagerTask
		clusterID       string
		overrideOptions []ScyllaDBManagerClientTaskOverrideOption
		expected        *managerclient.Task
		expectedErr     error
	}{
		{
			name:            "backup, sdc ref",
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
			name: "backup, sdc ref, without optional fields",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				smt.Spec.Backup = &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					Location: []string{"s3:test"},
				}

				return smt
			}(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "T5G8Gq6d57eDIXsPFZ+dwmW4nlU23DRApSj0alnkhsMs500IBuOOmj7EGUxaMtrcmMqGKOUHCUufq4HULj5rfQ==",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "NvWFWX5rjnaNd+DUOGXY51GXeGti2DN3MqYPfLS6xYc/2Nd2A8wyaDDyZMSXQHBLJ9BFGPdxJsn7iikMC2WLKA==",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "6HnivVqLpVb16NDTYjM+RzWHeqGo1ujZxzd6Qrxy/cpmU7fvRO6NvVP6zJ/24b85knGJYL3nprGJZz6APsgb8w==",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "GhcaxI4G+qbAK0XwbKsc4u+NwaDThEG1JbI4WWAyhejDAbdv160RicxioPyeieR+gl0G6R+fGUHOjVYx9gYREw==",
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
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", utilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", errors.New("time: invalid duration +invalid"))})),
		},
		{
			name: "backup, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", utilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", errors.New("time: invalid duration +invalid"))})),
		},
		{
			name: "backup, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "4UTdVpY6eSf5DWFXRAYX0LiCxQBSiGV1qjC15OxNl48QkVby2U/+PIUwLvwE58DbV8GcMsitfH875MO6ESDCtA==",
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
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "4UTdVpY6eSf5DWFXRAYX0LiCxQBSiGV1qjC15OxNl48QkVby2U/+PIUwLvwE58DbV8GcMsitfH875MO6ESDCtA==",
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
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "GhcaxI4G+qbAK0XwbKsc4u+NwaDThEG1JbI4WWAyhejDAbdv160RicxioPyeieR+gl0G6R+fGUHOjVYx9gYREw==",
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
			name:      "backup, sdc ref, without startDate override annotation and with start date retention override option",
			smt:       newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
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
			name: "backup, sdc ref, with schedule timezone override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation, "CET")

				return smt
			}(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "SbJqyEOwN8IFq0DEOXcvNwViISRjQu4PDhRppECp6LfFT8kR4SFTLTk4wuY1UTs6GLYcil3bVbHUzewt4XVa3g==",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "s72uFeCatEQo3EBouTg+Wzi55bFIsgg897hEyxxh9AjgQ09VQUg0xUjKxjKXE7L18KFNAFiBHrV5RSFhJ1tUUQ==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "79j/e7ztL7q+/SWGxBRj63WOxorQUF3eMsEywS1j9i8cmMEUoXj2lClfmBAruNAIsFBIHBCVjZ3KK6bUO3Q2rg==",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "UGu6TTHziXIRZ3sQGUHliorEbZt0AyOqJiR6m3s7UvzCn6vnMOJjts8knWEZrnZ2quU7f62rmNh7WrjZtR5xUA==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "override",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "nxI5/IlJZIhDZEZraUD89hCRjT2lnYIeDNdBX+u/Amj25DlvBndU0PtKEdcNXa/22zlbuB75ocR0HLqf7SOhkA==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", utilerrors.NewAggregate([]error{fmt.Errorf("can't parse intensity: %w", &strconv.NumError{Func: "ParseFloat", Num: "invalid", Err: strconv.ErrSyntax})})),
		},
		{
			name: "repair, sdc ref, with interval override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation, "7d")

				return smt
			}(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "U0W71XgnGX4o7783qaygjL7nEZD48FI1bO4HeZKazNcy3AmZ8FbkJE7/y0SXqgXJxXlDC/nFSd8kbnagK91NkQ==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "wFRIvwVi08yC5GlpHCt3YjpgND0POvmPQjaLRktyQt3ZDpPCe1+kHdP0PwH2QBAX4DotDWlN5mPXe9cp9b8iBw==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", utilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", errors.New("time: invalid duration +invalid"))})),
		},
		{
			name: "repair, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", utilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", errors.New("time: invalid duration +invalid"))})),
		},
		{
			name: "repair, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "YHkls2D3hu2ZXTF0/W9kJwma1UrHPkWoi+OWOKEPh5/rW3b9+hLCqIxTG94oUVRdJJVT+HlgCIEY1w+51QRjWw==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "YHkls2D3hu2ZXTF0/W9kJwma1UrHPkWoi+OWOKEPh5/rW3b9+hLCqIxTG94oUVRdJJVT+HlgCIEY1w+51QRjWw==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "wFRIvwVi08yC5GlpHCt3YjpgND0POvmPQjaLRktyQt3ZDpPCe1+kHdP0PwH2QBAX4DotDWlN5mPXe9cp9b8iBw==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			name:      "repair, sdc ref, without startDate override annotation and with start date retention override option",
			smt:       newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID: "cluster-id",
			overrideOptions: []ScyllaDBManagerClientTaskOverrideOption{
				WithScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "s72uFeCatEQo3EBouTg+Wzi55bFIsgg897hEyxxh9AjgQ09VQUg0xUjKxjKXE7L18KFNAFiBHrV5RSFhJ1tUUQ==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "4htlI2tll0/Fvdnoe9FKD7vHmryaW0wBJjE3eujjnEEUnhyE381keJX2E7dcVstekDE5r2a+z50q0C/JytdZKA==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
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
		// TODO: invalid small table threshold
		// TODO: unescape filters test?
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
