// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
		Spec   v1.ScyllaClusterSpec
		Status v1.ScyllaClusterStatus
		State  state

		Actions []action
		Requeue bool
	}{
		{
			Name:   "Empty manager, empty spec, add cluster and requeue",
			Spec:   v1.ScyllaClusterSpec{},
			Status: v1.ScyllaClusterStatus{},
			State:  state{},

			Requeue: true,
			Actions: []action{&addClusterAction{cluster: &mermaidclient.Cluster{Name: clusterName}}},
		},
		{
			Name: "Empty manager, task in spec, add cluster and requeue",
			Spec: v1.ScyllaClusterSpec{
				Repairs: []v1.RepairTaskSpec{},
			},
			Status: v1.ScyllaClusterStatus{},
			State:  state{},

			Requeue: true,
			Actions: []action{&addClusterAction{cluster: &mermaidclient.Cluster{Name: clusterName}}},
		},
		{
			Name:   "Cluster registered in manager do nothing",
			Spec:   v1.ScyllaClusterSpec{},
			Status: v1.ScyllaClusterStatus{},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Requeue: false,
			Actions: nil,
		},
		{
			Name: "Cluster registered in manager but auth token is different, update and requeue",
			Spec: v1.ScyllaClusterSpec{},
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: "different-auth-token",
				}},
			},

			Requeue: true,
			Actions: []action{&updateClusterAction{cluster: &mermaidclient.Cluster{ID: clusterID}}},
		},
		{
			Name: "Name collision, delete old one, add new and requeue",
			Spec: v1.ScyllaClusterSpec{},
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:   "different-id",
					Name: clusterName,
				}},
			},

			Requeue: true,
			Actions: []action{
				&deleteClusterAction{clusterID: "different-id"},
				&addClusterAction{cluster: &mermaidclient.Cluster{Name: clusterName}},
			},
		},
		{
			Name: "Schedule repair task",
			Spec: v1.ScyllaClusterSpec{
				Repairs: []v1.RepairTaskSpec{
					{
						SchedulerTaskSpec: v1.SchedulerTaskSpec{
							Name:      "my-repair",
							StartDate: "now",
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
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Actions: []action{&addTaskAction{clusterID: clusterID, task: &mermaidclient.Task{Name: "my-repair"}}},
		},
		{
			Name: "Schedule backup task",
			Spec: v1.ScyllaClusterSpec{
				Backups: []v1.BackupTaskSpec{
					{
						SchedulerTaskSpec: v1.SchedulerTaskSpec{
							Name:      "my-backup",
							StartDate: "now",
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
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
			},

			Actions: []action{&addTaskAction{clusterID: clusterID, task: &mermaidclient.Task{Name: "my-backup"}}},
		},
		{
			Name: "Update repair if it's already registered in Manager",
			Spec: v1.ScyllaClusterSpec{
				Repairs: []v1.RepairTaskSpec{
					{
						SchedulerTaskSpec: v1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "now",
							Interval:  "0",
						},
						SmallTableThreshold: "1GiB",
						Intensity:           "0",
					},
				},
			},
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
				Repairs: []v1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: v1.RepairTaskSpec{
							SchedulerTaskSpec: v1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "now",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: v1.RepairTaskSpec{
							SchedulerTaskSpec: v1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "now",
								Interval:  "0",
							},
							Intensity:           "123",
							SmallTableThreshold: "1GiB",
						},
						ID: "repair-id",
					},
				},
			},

			Actions: []action{&updateTaskAction{clusterID: clusterID, task: &mermaidclient.Task{ID: "repair-id"}}},
		},
		{
			Name: "Do not update task when it didn't change",
			Spec: v1.ScyllaClusterSpec{
				Repairs: []v1.RepairTaskSpec{
					{
						SchedulerTaskSpec: v1.SchedulerTaskSpec{
							Name:      "repair",
							StartDate: "now",
							Interval:  "0",
						},
						Intensity:           "666",
						SmallTableThreshold: "1GiB",
					},
				},
			},
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
				Repairs: []v1.RepairTaskStatus{
					{
						ID: "repair-id",
						RepairTaskSpec: v1.RepairTaskSpec{
							SchedulerTaskSpec: v1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "now",
								Interval:  "0",
							},
							Intensity:           "666",
							SmallTableThreshold: "1GiB",
						},
					},
				},
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: v1.RepairTaskSpec{
							SchedulerTaskSpec: v1.SchedulerTaskSpec{
								Name:      "repair",
								StartDate: "now",
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
			Spec: v1.ScyllaClusterSpec{},
			Status: v1.ScyllaClusterStatus{
				ManagerID: pointer.StringPtr(clusterID),
			},
			State: state{
				Clusters: []*mermaidclient.Cluster{{
					ID:        clusterID,
					Name:      clusterName,
					AuthToken: clusterAuthToken,
				}},
				RepairTasks: []*RepairTask{
					{
						RepairTaskSpec: v1.RepairTaskSpec{
							SchedulerTaskSpec: v1.SchedulerTaskSpec{
								Name:      "other-repair",
								StartDate: "now",
								Interval:  "0",
							},
						},
						ID: "other-repair-id",
					},
				},
			},

			Actions: []action{&deleteTaskAction{clusterID: clusterID, taskID: "other-repair-id"}},
		},
	}

	for i := range tcs {
		test := tcs[i]
		t.Run(test.Name, func(t *testing.T) {
			ctx := context.Background()
			cluster := &v1.ScyllaCluster{
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
