// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_runSync(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		sc              *scyllav1.ScyllaCluster
		authToken       string
		state           *managerClusterState
		expectedActions []action
		expectedRequeue bool
		expectedErr     error
	}{
		{
			name:      "no cluster in state, no tasks in state, return add cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: nil,
			},
			expectedActions: []action{
				&addClusterAction{
					Cluster: &managerclient.Cluster{
						AuthToken:              "token",
						ForceNonSslSessionPort: true,
						ForceTLSDisabled:       true,
						Host:                   "test-client.test.svc",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
							"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
						},
						Name: "test/test",
					},
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, missing owner UID label, no tasks in state, return delete cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&deleteClusterAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, empty owner UID label, no tasks in state, return delete cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&deleteClusterAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, mismatching owner UID label, no tasks in state, return delete cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "other-uid",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&deleteClusterAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, matching owner UID label, missing managed hash label, no tasks in state, return update cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid": "1bbeb48b-101f-4d49-8ba4-67adc9878721",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&updateClusterAction{
					Cluster: &managerclient.Cluster{
						AuthToken:              "token",
						ForceNonSslSessionPort: true,
						ForceTLSDisabled:       true,
						Host:                   "test-client.test.svc",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
							"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
						},
						Name: "test/test",
						ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					},
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, matching owner UID label, empty managed hash label, no tasks in state, return update cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&updateClusterAction{
					Cluster: &managerclient.Cluster{
						AuthToken:              "token",
						ForceNonSslSessionPort: true,
						ForceTLSDisabled:       true,
						Host:                   "test-client.test.svc",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
							"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
						},
						Name: "test/test",
						ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					},
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, matching owner UID label, mismatching managed hash label, no tasks in state, return update cluster action and requeue",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "other-managed-hash",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&updateClusterAction{
					Cluster: &managerclient.Cluster{
						AuthToken:              "token",
						ForceNonSslSessionPort: true,
						ForceTLSDisabled:       true,
						Host:                   "test-client.test.svc",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
							"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
						},
						Name: "test/test",
						ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					},
				},
			},
			expectedRequeue: true,
			expectedErr:     nil,
		},
		{
			name:      "cluster in state, missing clusterID, return err",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "",
				},
			},
			expectedActions: nil,
			expectedRequeue: false,
			expectedErr:     fmt.Errorf(`can't sync cluster "test/test": %w`, fmt.Errorf(`manager cluster is missing an id`)),
		},
		{
			name:      "matching cluster in state, no tasks in state, return add task actions",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
			},
			expectedActions: []action{
				&addTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "e6nOWps39EWcHS4BmiEti8MfyH6PL5Yt3kpM4Vej1Rd0SAsCM39CXe/XH1Yjhsi3b611nx7yfK9paShMxaNqGw==",
						},
						Name: "repair",
						Properties: map[string]any{
							"intensity":             float64(0.5),
							"parallel":              int64(0),
							"small_table_threshold": int64(1073741824),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "repair",
					},
				},
				&addTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "c+YqtlphqYcYveZmyspNBzt77JuW9zNWKMdUPv8AGbP+9j3Gi4KvQqJyBrq5DPFDV6E8rWxcyXHF2mNYFzwIaA==",
						},
						Name: "backup",
						Properties: map[string]any{
							"location":  []string{"gcs:location"},
							"retention": int64(0),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "backup",
					},
				},
			},
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name:      "matching cluster in state, tasks in state, missing managed hash labels, return update task actions",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name:   "backup",
							ID:     pointer.Ptr("backup-id"),
							Labels: map[string]string{},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name:   "repair",
							ID:     pointer.Ptr("repair-id"),
							Labels: map[string]string{},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: []action{
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "repair-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "e6nOWps39EWcHS4BmiEti8MfyH6PL5Yt3kpM4Vej1Rd0SAsCM39CXe/XH1Yjhsi3b611nx7yfK9paShMxaNqGw==",
						},
						Name: "repair",
						Properties: map[string]any{
							"intensity":             float64(0.5),
							"parallel":              int64(0),
							"small_table_threshold": int64(1073741824),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "repair",
					},
				},
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "backup-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "c+YqtlphqYcYveZmyspNBzt77JuW9zNWKMdUPv8AGbP+9j3Gi4KvQqJyBrq5DPFDV6E8rWxcyXHF2mNYFzwIaA==",
						},
						Name: "backup",
						Properties: map[string]any{
							"location":  []string{"gcs:location"},
							"retention": int64(0),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "backup",
					},
				},
			},
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name:      "matching cluster in state, tasks in state, empty managed hash labels, return update task actions",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "backup",
							ID:   pointer.Ptr("backup-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "",
							},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "repair",
							ID:   pointer.Ptr("repair-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "",
							},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: []action{
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "repair-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "e6nOWps39EWcHS4BmiEti8MfyH6PL5Yt3kpM4Vej1Rd0SAsCM39CXe/XH1Yjhsi3b611nx7yfK9paShMxaNqGw==",
						},
						Name: "repair",
						Properties: map[string]any{
							"intensity":             float64(0.5),
							"parallel":              int64(0),
							"small_table_threshold": int64(1073741824),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "repair",
					},
				},
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "backup-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "c+YqtlphqYcYveZmyspNBzt77JuW9zNWKMdUPv8AGbP+9j3Gi4KvQqJyBrq5DPFDV6E8rWxcyXHF2mNYFzwIaA==",
						},
						Name: "backup",
						Properties: map[string]any{
							"location":  []string{"gcs:location"},
							"retention": int64(0),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "backup",
					},
				},
			},
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name:      "matching cluster in state, tasks in state, mismatching managed hash labels, return update task actions",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "backup",
							ID:   pointer.Ptr("backup-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "other-managed-hash",
							},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "repair",
							ID:   pointer.Ptr("repair-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "other-managed-hash",
							},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: []action{
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "repair-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "e6nOWps39EWcHS4BmiEti8MfyH6PL5Yt3kpM4Vej1Rd0SAsCM39CXe/XH1Yjhsi3b611nx7yfK9paShMxaNqGw==",
						},
						Name: "repair",
						Properties: map[string]any{
							"intensity":             float64(0.5),
							"parallel":              int64(0),
							"small_table_threshold": int64(1073741824),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "repair",
					},
				},
				&updateTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					Task: &managerclient.Task{
						ID:      "backup-id",
						Enabled: true,
						Labels: map[string]string{
							"scylla-operator.scylladb.com/managed-hash": "c+YqtlphqYcYveZmyspNBzt77JuW9zNWKMdUPv8AGbP+9j3Gi4KvQqJyBrq5DPFDV6E8rWxcyXHF2mNYFzwIaA==",
						},
						Name: "backup",
						Properties: map[string]any{
							"location":  []string{"gcs:location"},
							"retention": int64(0),
						},
						Schedule: &managerclient.Schedule{
							StartDate: validDateTime,
						},
						Type: "backup",
					},
				},
			},
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name:      "matching cluster in state, tasks in state, matching managed hash labels, return no actions",
			sc:        newBasicScyllaClusterWithBackupAndRepairTasks(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "backup",
							ID:   pointer.Ptr("backup-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "c+YqtlphqYcYveZmyspNBzt77JuW9zNWKMdUPv8AGbP+9j3Gi4KvQqJyBrq5DPFDV6E8rWxcyXHF2mNYFzwIaA==",
							},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "repair",
							ID:   pointer.Ptr("repair-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "e6nOWps39EWcHS4BmiEti8MfyH6PL5Yt3kpM4Vej1Rd0SAsCM39CXe/XH1Yjhsi3b611nx7yfK9paShMxaNqGw==",
							},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: nil,
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name: "matching cluster in state, tasks in state, tasks with 'now' startDate and matching managed hash labels, return no actions",
			sc: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()

				sc.Spec.Backups = []scyllav1.BackupTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "backup",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								StartDate: pointer.Ptr("now"),
							},
						},
						Location: []string{"gcs:location"},
					},
				}

				sc.Spec.Repairs = []scyllav1.RepairTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "repair",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								StartDate: pointer.Ptr("now"),
							},
						},
						SmallTableThreshold: "1GiB",
						Intensity:           "0.5",
					},
				}

				return sc
			}(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "backup",
							ID:   pointer.Ptr("backup-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "0aeio9mEaSKenZ7/GRW4l0TFm7f9kY2w7A3wpO3du5+EfjM9zIqJunon9vT+VvGNcwYxQQqF4gmZW1GSLpZU6g==",
							},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "repair",
							ID:   pointer.Ptr("repair-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "Il1bCvnHQAs7fjjuE4OHn6Au6tvMxNsVrfvdbwg3uYeNZPzYR+qgvDyAEjVrwakzfOTu/e+viMTWzaCRLqYetA==",
							},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: nil,
			expectedRequeue: false,
			expectedErr:     nil,
		},
		{
			name:      "matching cluster in state, superfluous tasks in state, return delete task actions",
			sc:        newBasicScyllaCluster(),
			authToken: "token",
			state: &managerClusterState{
				Cluster: &managerclient.Cluster{
					AuthToken:              "token",
					ForceNonSslSessionPort: true,
					ForceTLSDisabled:       true,
					Host:                   "test-client.test.svc",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":    "1bbeb48b-101f-4d49-8ba4-67adc9878721",
						"scylla-operator.scylladb.com/managed-hash": "UfHEc1kxt3UHl1r2ETXinXfhAtyYrha5RRJn544zkvY6bLDyzl1Q7wSskU355iHlIuwFHyeePE80I0ZOSmoChA==",
					},
					Name: "test/test",
					ID:   "bead6247-d9e4-401c-84b4-ad0bffe36eac",
				},
				BackupTasks: map[string]scyllav1.BackupTaskStatus{
					"backup": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "backup",
							ID:   pointer.Ptr("backup-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "0aeio9mEaSKenZ7/GRW4l0TFm7f9kY2w7A3wpO3du5+EfjM9zIqJunon9vT+VvGNcwYxQQqF4gmZW1GSLpZU6g==",
							},
						},
						Location: []string{"gcs:location"},
					},
				},
				RepairTasks: map[string]scyllav1.RepairTaskStatus{
					"repair": {
						TaskStatus: scyllav1.TaskStatus{
							SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
								StartDate: pointer.Ptr(validDate),
							},
							Name: "repair",
							ID:   pointer.Ptr("repair-id"),
							Labels: map[string]string{
								"scylla-operator.scylladb.com/managed-hash": "Il1bCvnHQAs7fjjuE4OHn6Au6tvMxNsVrfvdbwg3uYeNZPzYR+qgvDyAEjVrwakzfOTu/e+viMTWzaCRLqYetA==",
							},
						},
						Intensity:           pointer.Ptr("0.5"),
						Parallel:            pointer.Ptr[int64](0),
						SmallTableThreshold: pointer.Ptr("1GiB"),
					},
				},
			},
			expectedActions: []action{
				&deleteTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					TaskType:  "repair",
					TaskID:    "repair-id",
				},
				&deleteTaskAction{
					ClusterID: "bead6247-d9e4-401c-84b4-ad0bffe36eac",
					TaskType:  "backup",
					TaskID:    "backup-id",
				},
			},
			expectedRequeue: false,
			expectedErr:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			actions, requeue, err := runSync(ctx, tc.sc, tc.authToken, tc.state)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}
			if requeue != tc.expectedRequeue {
				t.Errorf("expected requeue %t, got %t", tc.expectedRequeue, requeue)
			}
			if !reflect.DeepEqual(actions, tc.expectedActions) {
				t.Errorf("expected and got actions differ:\n%s", cmp.Diff(tc.expectedActions, actions))
			}
		})
	}
}

func Test_evaluateDates(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		spec     *scyllav1.TaskSpec
		status   *scyllav1.TaskStatus
		expected scyllav1.TaskStatus
	}{
		{
			name: "Task startDate is changed to one from manager state when it's not provided",
			spec: &scyllav1.TaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{},
			},
			status: &scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
			expected: scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
		},
		{
			name: "Task startDate is changed to one from manager state when it's an empty string",
			spec: &scyllav1.TaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: pointer.Ptr(""),
				},
			},
			status: &scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
			expected: scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
		},
		{
			name: "Task startDate is changed to one from manager state when it's 'now' literal",
			spec: &scyllav1.TaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: pointer.Ptr("now"),
				},
			},
			status: &scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
			expected: scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
		},
		{
			name: "Task startDate is changed to one from manager state when prefix is 'now'",
			spec: &scyllav1.TaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: pointer.Ptr("now+3d2h10m"),
				},
			},
			status: &scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
			expected: scyllav1.TaskStatus{
				SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
					StartDate: pointer.Ptr("2021-01-01T11:11:11Z"),
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evaluateDates(tc.spec, tc.status)
			got := taskSpecToStatus(tc.spec)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got repair task statuses differ:\n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func newBasicScyllaClusterWithBackupAndRepairTasks() *scyllav1.ScyllaCluster {
	sc := newBasicScyllaCluster()

	sc.Spec.Backups = []scyllav1.BackupTaskSpec{
		{
			TaskSpec: scyllav1.TaskSpec{
				Name: "backup",
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: pointer.Ptr(validDate),
				},
			},
			Location: []string{"gcs:location"},
		},
	}

	sc.Spec.Repairs = []scyllav1.RepairTaskSpec{
		{
			TaskSpec: scyllav1.TaskSpec{
				Name: "repair",
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: pointer.Ptr(validDate),
				},
			},
			SmallTableThreshold: "1GiB",
			Intensity:           "0.5",
		},
	}

	return sc
}

func newBasicScyllaCluster() *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "1bbeb48b-101f-4d49-8ba4-67adc9878721",
		},
		Spec:   scyllav1.ScyllaClusterSpec{},
		Status: scyllav1.ScyllaClusterStatus{},
	}
}
