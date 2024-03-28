// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagerSynchronization(t *testing.T) {
	const (
		clusterAuthToken = "token"
		namespace        = "test"
		name             = "cluster"
		clusterName      = "test/cluster"
		clusterID        = "cluster-id"
	)

	tcs := []struct {
		Name   string
		Spec   scyllav1.ScyllaClusterSpec
		Status scyllav1.ScyllaClusterStatus
		State  state

		Actions []action
		Requeue bool
	}{
		{
			Name:   "Empty manager, empty spec, add cluster and requeue",
			Spec:   scyllav1.ScyllaClusterSpec{},
			Status: scyllav1.ScyllaClusterStatus{},
			State:  state{},

			Requeue: true,
			Actions: []action{&addClusterAction{cluster: &managerclient.Cluster{Name: clusterName}}},
		},
		{
			Name: "Empty manager, task in spec, add cluster and requeue",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{},
			},
			Status: scyllav1.ScyllaClusterStatus{},
			State:  state{},

			Requeue: true,
			Actions: []action{&addClusterAction{cluster: &managerclient.Cluster{Name: clusterName}}},
		},
		{
			Name:   "Cluster registered in manager do nothing",
			Spec:   scyllav1.ScyllaClusterSpec{},
			Status: scyllav1.ScyllaClusterStatus{},
			State: state{
				Clusters: []*managerclient.Cluster{{
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Requeue: false,
			Actions: nil,
		},
		{
			Name: "Cluster registered in manager but auth token is different, update and requeue",
			Spec: scyllav1.ScyllaClusterSpec{},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: "different-auth-token",
				}},
			},

			Requeue: true,
			Actions: []action{&updateClusterAction{cluster: &managerclient.Cluster{ID: clusterID}}},
		},
		{
			Name: "Name collision, delete old one, add new and requeue",
			Spec: scyllav1.ScyllaClusterSpec{},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:   "different-id",
					Name: clusterName,
				}},
			},

			Requeue: true,
			Actions: []action{
				&deleteClusterAction{clusterID: "different-id"},
				&addClusterAction{cluster: &managerclient.Cluster{Name: clusterName}},
			},
		},
		{
			Name: "Schedule repair task",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "my-repair",
							StartDate: "2006-01-02T15:04:05Z",
							Interval:  "0",
						},
						SmallTableThreshold: "1GiB",
						DC:                  []string{"dc1"},
						FailFast:            false,
						Intensity:           "0.5",
						Keyspace:            []string{"keyspace1"},
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Actions: []action{&addTaskAction{clusterID: clusterID, task: &managerclient.Task{Name: "my-repair"}}},
		},
		{
			Name: "Schedule backup task",
			Spec: scyllav1.ScyllaClusterSpec{
				Backups: []scyllav1.BackupTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "my-backup",
							StartDate: "2006-01-02T15:04:05Z",
							Interval:  "0",
						},
						DC:               []string{"dc1"},
						Keyspace:         []string{"keyspace1"},
						Location:         []string{"s3:abc"},
						RateLimit:        []string{"dc1:1"},
						Retention:        3,
						SnapshotParallel: []string{"dc1:1"},
						UploadParallel:   []string{"dc1:1"},
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Actions: []action{&addTaskAction{clusterID: clusterID, task: &managerclient.Task{Name: "my-backup"}}},
		},
		{
			Name: "Update repair if it's already registered in Manager",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "2006-01-02T15:04:05Z",
							Interval:  "0",
						},
						SmallTableThreshold: "1GiB",
						Intensity:           "0",
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
				Repairs: []scyllav1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2006-01-02T15:04:05Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2006-01-02T15:04:05Z",
								Interval:  "0",
							},
							Intensity:           "123",
							SmallTableThreshold: "1GiB",
						},
						ID: "repair-id",
					},
				},
			},

			Actions: []action{&updateTaskAction{clusterID: clusterID, task: &managerclient.Task{ID: "repair-id"}}},
		},
		{
			Name: "Do not update task when it didn't change",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "2021-01-01T11:11:11Z",
							Interval:  "0",
						},
						Intensity:           "666",
						SmallTableThreshold: "1GiB",
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
				Repairs: []scyllav1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
						ID: "repair-id",
					},
				},
			},

			Actions: nil,
		},
		{
			Name: "Delete tasks from Manager unknown to spec",
			Spec: scyllav1.ScyllaClusterSpec{},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "other-repair",
								StartDate: "2006-01-02T15:04:05Z",
								Interval:  "0",
							},
						},
						ID: "other-repair-id",
					},
				},
			},

			Actions: []action{&deleteTaskAction{clusterID: clusterID, taskID: "other-repair-id"}},
		},
		{
			Name: "Special 'now' startDate is not compared during update decision",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "now",
							Interval:  "0",
						},
						Intensity:           "666",
						SmallTableThreshold: "1GiB",
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
				Repairs: []scyllav1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						ID: "repair-id",
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},

			Actions: nil,
		},
		{
			Name: "Task gets updated when startDate is changed",
			Spec: scyllav1.ScyllaClusterSpec{
				Repairs: []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "2006-01-02T15:04:05Z",
							Interval:  "0",
						},
						Intensity:           "666",
						SmallTableThreshold: "1GiB",
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				ManagerID: pointer.Ptr(clusterID),
				Repairs: []scyllav1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*managerclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: scyllav1.RepairTaskSpec{
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "2021-01-01T11:11:11Z",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
						ID: "repair-id",
					},
				},
			},

			Actions: []action{&updateTaskAction{clusterID: clusterID, task: &managerclient.Task{ID: "repair-id"}}},
		},
	}

	for _, test := range tcs {
		t.Run(test.Name, func(t *testing.T) {
			ctx := context.Background()
			cluster := &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       test.Spec,
				Status:     test.Status,
			}

			actions, requeue, err := runSync(ctx, cluster, clusterAuthToken, &test.State)
			if err != nil {
				t.Error(err)
			}
			if requeue != test.Requeue {
				t.Error(err, "Unexpected requeue")
			}
			if !cmp.Equal(actions, test.Actions, cmp.Comparer(actionComparer)) {
				t.Error(err, "Unexpected actions", cmp.Diff(actions, test.Actions, cmp.Comparer(actionComparer)))
			}
		})
	}
}

func actionComparer(a action, b action) bool {
	switch va := a.(type) {
	case *addClusterAction:
		vb := b.(*addClusterAction)
		return va.cluster.Name == vb.cluster.Name
	case *updateClusterAction:
		vb := b.(*updateClusterAction)
		return va.cluster.ID == vb.cluster.ID
	case *deleteClusterAction:
		vb := b.(*deleteClusterAction)
		return va.clusterID == vb.clusterID
	case *updateTaskAction:
		vb := b.(*updateTaskAction)
		return va.clusterID == vb.clusterID && va.task.ID == vb.task.ID
	case *addTaskAction:
		vb := b.(*addTaskAction)
		return va.clusterID == vb.clusterID && va.task.Name == vb.task.Name
	case *deleteTaskAction:
		vb := b.(*deleteTaskAction)
		return va.clusterID == vb.clusterID && va.taskID == vb.taskID
	default:
	}
	return false
}

func TestBackupTaskChanged(t *testing.T) {
	ts := []struct {
		name        string
		spec        *BackupTask
		managerTask *BackupTask
		expected    *BackupTask
	}{
		{
			name: "Task startDate is changed to one from manager state when prefix is 'now'",
			spec: &BackupTask{BackupTaskSpec: scyllav1.BackupTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "now",
				},
			}},
			managerTask: &BackupTask{BackupTaskSpec: scyllav1.BackupTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "2021-01-01T11:11:11Z",
				},
			}},
			expected: &BackupTask{BackupTaskSpec: scyllav1.BackupTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "2021-01-01T11:11:11Z",
				},
			}},
		},
	}

	for _, test := range ts {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			evaluateDates(test.spec, test.managerTask)
			if !reflect.DeepEqual(test.spec, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, test.spec)
			}
		})
	}
}

func TestRepairTaskChanged(t *testing.T) {
	ts := []struct {
		name        string
		spec        *RepairTask
		managerTask *RepairTask
		expected    *RepairTask
	}{
		{
			name: "Task startDate is changed to one from manager state when prefix is 'now'",
			spec: &RepairTask{RepairTaskSpec: scyllav1.RepairTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "now",
				},
			}},
			managerTask: &RepairTask{RepairTaskSpec: scyllav1.RepairTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "2021-01-01T11:11:11Z",
				},
			}},
			expected: &RepairTask{RepairTaskSpec: scyllav1.RepairTaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					StartDate: "2021-01-01T11:11:11Z",
				},
			}},
		},
	}

	for _, test := range ts {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			evaluateDates(test.spec, test.managerTask)
			if !reflect.DeepEqual(test.spec, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, test.spec)
			}
		})
	}
}
