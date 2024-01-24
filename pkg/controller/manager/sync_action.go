// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

type state struct {
	Clusters    []*managerclient.Cluster
	RepairTasks []*RepairTask
	BackupTasks []*BackupTask
}

func runSync(ctx context.Context, cluster *v1.ScyllaCluster, authToken string, state *state) ([]action, bool, error) {
	var actions []action
	requeue := false
	clusterID := ""
	clusterName := naming.ManagerClusterName(cluster)
	if cluster.Status.ManagerID != nil {
		clusterID = *cluster.Status.ManagerID
	}
	found := false
	for _, c := range state.Clusters {
		if c.Name == clusterName || c.ID == clusterID {
			found = true
			if c.ID == clusterID {
				// TODO: can't detect changes for following params:
				// * known hosts aren't returned by the API
				// * username/password are not part of Cluster CRD
				if c.AuthToken != authToken {
					actions = append(actions, &updateClusterAction{
						cluster: &managerclient.Cluster{
							ID:   c.ID,
							Name: naming.ManagerClusterName(cluster),
							Host: naming.CrossNamespaceServiceNameForCluster(cluster),
						},
					})
					requeue = true
				}
			} else {
				// Delete old to avoid name collision.
				actions = append(actions, &deleteClusterAction{clusterID: c.ID})
				found = false
			}
		}
	}
	if !found {
		actions = append(actions, &addClusterAction{
			cluster: &managerclient.Cluster{
				Host:      naming.CrossNamespaceServiceNameForCluster(cluster),
				Name:      naming.ManagerClusterName(cluster),
				AuthToken: authToken,
			},
		})
		requeue = true
	}

	if found {
		taskActions, err := syncTasks(clusterID, cluster, state)
		if err != nil {
			return nil, false, err
		}
		actions = append(actions, taskActions...)
	}

	return actions, requeue, nil
}

func syncTasks(clusterID string, cluster *v1.ScyllaCluster, state *state) ([]action, error) {
	syncer := newStateCache(cluster, state)

	var actions []action

	for _, task := range state.BackupTasks {
		if syncer.shouldDeleteTask(task.ID) {
			actions = append(actions, &deleteTaskAction{
				clusterID: clusterID,
				taskType:  "backup",
				taskID:    task.ID,
			})
		}
	}
	for _, task := range state.RepairTasks {
		if syncer.shouldDeleteTask(task.ID) {
			actions = append(actions, &deleteTaskAction{
				clusterID: clusterID,
				taskType:  "repair",
				taskID:    task.ID,
			})
		}
	}

	repairActions, err := syncRepairTasks(clusterID, cluster, syncer, state)
	if err != nil {
		return nil, errors.Wrap(err, "runSync repair tasks")
	}
	actions = append(actions, repairActions...)

	backupActions, err := syncBackupTasks(clusterID, cluster, syncer, state)
	if err != nil {
		return nil, errors.Wrap(err, "runSync repair tasks")
	}

	actions = append(actions, backupActions...)

	return actions, nil
}

func syncBackupTasks(clusterID string, cluster *v1.ScyllaCluster, syncer stateCache, managerState *state) ([]action, error) {
	var actions []action

	for _, bt := range cluster.Spec.Backups {
		backupTask := &BackupTask{BackupTaskSpec: bt}

		if syncer.shouldCreateTask(backupTask.Name) {
			mt, err := backupTask.ToManager()
			if err != nil {
				return nil, errors.Wrap(err, "transform to manager task")
			}
			actions = append(actions, &addTaskAction{
				clusterID: clusterID,
				task:      mt,
				taskSpec:  bt,
			})
		} else if syncer.shouldUpdateTask(backupTask.Name) {
			backupTask.ID = syncer.taskID(backupTask.Name)

			update := false
			for _, managerTask := range managerState.BackupTasks {
				if managerTask.ID == backupTask.ID {
					update = !reflect.DeepEqual(backupTask, managerTask)
				}
			}
			if update {
				mt, err := backupTask.ToManager()
				if err != nil {
					return nil, errors.Wrap(err, "transform to manager task")
				}

				actions = append(actions, &updateTaskAction{
					clusterID: clusterID,
					task:      mt,
					taskSpec:  bt,
				})
			}
		}
	}

	return actions, nil
}

func syncRepairTasks(clusterID string, cluster *v1.ScyllaCluster, syncer stateCache, managerState *state) ([]action, error) {
	var actions []action

	for _, rt := range cluster.Spec.Repairs {
		repairTask := &RepairTask{RepairTaskSpec: rt}

		if syncer.shouldCreateTask(rt.Name) {
			mt, err := repairTask.ToManager()
			if err != nil {
				return nil, errors.Wrap(err, "transform to manager task")
			}
			actions = append(actions, &addTaskAction{
				clusterID: clusterID,
				task:      mt,
				taskSpec:  rt,
			})
		} else if syncer.shouldUpdateTask(rt.Name) {
			repairTask.ID = syncer.taskID(rt.Name)

			update := false
			for _, managerTask := range managerState.RepairTasks {
				if managerTask.ID == repairTask.ID {
					update = !reflect.DeepEqual(repairTask, managerTask)
				}
			}
			if update {
				mt, err := repairTask.ToManager()
				if err != nil {
					return nil, errors.Wrap(err, "transform to manager task")
				}
				actions = append(actions, &updateTaskAction{
					clusterID: clusterID,
					task:      mt,
					taskSpec:  rt,
				})
			}
		}
	}

	return actions, nil
}

type stateCache struct {
	stateTasks          *strset.Set
	specTasks           *strset.Set
	statusNameIDMapping map[string]string
	statusIDNameMapping map[string]string
}

func newStateCache(cluster *v1.ScyllaCluster, state *state) stateCache {
	s := stateCache{
		stateTasks:          strset.New(),
		specTasks:           strset.New(),
		statusNameIDMapping: make(map[string]string),
		statusIDNameMapping: make(map[string]string),
	}
	for _, task := range state.RepairTasks {
		s.stateTasks.Add(task.ID)
	}
	for _, task := range cluster.Spec.Repairs {
		s.specTasks.Add(task.Name)
	}
	for _, task := range cluster.Status.Repairs {
		s.statusNameIDMapping[task.Name] = task.ID
		s.statusIDNameMapping[task.ID] = task.Name
	}

	for _, task := range state.BackupTasks {
		s.stateTasks.Add(task.ID)
	}
	for _, task := range cluster.Spec.Backups {
		s.specTasks.Add(task.Name)
	}
	for _, task := range cluster.Status.Backups {
		s.statusNameIDMapping[task.Name] = task.ID
		s.statusIDNameMapping[task.ID] = task.Name
	}
	return s
}

func (s stateCache) shouldDeleteTask(id string) bool {
	if _, definedInStatus := s.statusIDNameMapping[id]; !definedInStatus {
		return true
	} else {
		definedInSpec := s.specTasks.Has(s.statusIDNameMapping[id])
		if !definedInSpec {
			return true
		}
	}
	return false
}

func (s stateCache) shouldCreateTask(name string) bool {
	taskID, foundInStatus := s.statusNameIDMapping[name]
	if !foundInStatus {
		return true
	} else {
		return !s.stateTasks.Has(taskID)
	}
}

func (s stateCache) shouldUpdateTask(name string) bool {
	taskID, foundInStatus := s.statusNameIDMapping[name]
	if !foundInStatus {
		return false
	} else {
		return s.stateTasks.Has(taskID)
	}
}

func (s stateCache) taskID(taskName string) string {
	return s.statusNameIDMapping[taskName]
}

type action interface {
	Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error
}

type addClusterAction struct {
	cluster   *managerclient.Cluster
	clusterID string
}

func (a *addClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error {
	id, err := client.CreateCluster(ctx, a.cluster)
	if err != nil {
		return err
	} else {
		status.ManagerID = &id
	}

	return nil
}

func (a addClusterAction) String() string {
	return fmt.Sprintf("add cluster %q", a.clusterID)
}

type updateClusterAction struct {
	cluster *managerclient.Cluster
}

func (a *updateClusterAction) Execute(ctx context.Context, client *managerclient.Client, _ *v1.ScyllaClusterStatus) error {
	return client.UpdateCluster(ctx, a.cluster)
}

func (a updateClusterAction) String() string {
	return fmt.Sprintf("update cluster %q", a.cluster.ID)
}

type deleteClusterAction struct {
	clusterID string
}

func (a *deleteClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error {
	return client.DeleteCluster(ctx, a.clusterID)
}

func (a deleteClusterAction) String() string {
	return fmt.Sprintf("delete cluster %q", a.clusterID)
}

type deleteTaskAction struct {
	clusterID string
	taskType  string
	taskID    string
}

func (a *deleteTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error {
	err := client.DeleteTask(ctx, a.clusterID, a.taskType, uuid.MustParse(a.taskID))

	if a.taskType == "repair" {
		filteredStatuses := status.Repairs[:0]
		for i, repairTaskStatus := range status.Repairs {
			if err != nil && repairTaskStatus.ID == a.taskID {
				status.Repairs[i].Error = messageOf(err)
			}
			if err != nil || repairTaskStatus.ID != a.taskID {
				filteredStatuses = append(filteredStatuses, repairTaskStatus)
			}
		}
		status.Repairs = filteredStatuses
	}
	if a.taskType == "backup" {
		filteredStatuses := status.Backups[:0]
		for i, backupTaskStatus := range status.Backups {
			if err != nil && backupTaskStatus.ID == a.taskID {
				status.Backups[i].Error = messageOf(err)
			}
			if err != nil || backupTaskStatus.ID != a.taskID {
				filteredStatuses = append(filteredStatuses, backupTaskStatus)
			}
		}
		status.Backups = filteredStatuses
	}

	return err
}

func (a deleteTaskAction) String() string {
	return fmt.Sprintf("delete task %q", a.taskID)
}

type addTaskAction struct {
	clusterID string
	task      *managerclient.Task
	taskSpec  interface{}
}

func (a addTaskAction) String() string {
	return fmt.Sprintf("add task %+v", a.task)
}

func (a *addTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error {
	id, err := client.CreateTask(ctx, a.clusterID, a.task)

	if a.task.Type == "repair" {
		rt := v1.RepairTaskStatus{
			RepairTaskSpec: a.taskSpec.(v1.RepairTaskSpec),
			ID:             id.String(),
		}
		if err != nil {
			rt.Error = messageOf(err)
		}

		found := false
		for i, repairStatus := range status.Repairs {
			if repairStatus.Name == a.task.Name {
				found = true
				status.Repairs[i] = rt
			}
		}

		if !found {
			status.Repairs = append(status.Repairs, rt)
		}
	}
	if a.task.Type == "backup" {
		bt := v1.BackupTaskStatus{
			BackupTaskSpec: a.taskSpec.(v1.BackupTaskSpec),
			ID:             id.String(),
		}
		if err != nil {
			bt.Error = messageOf(err)
		}

		found := false
		for i, backupStatus := range status.Backups {
			if backupStatus.Name == a.task.Name {
				found = true
				status.Backups[i] = bt
			}
		}

		if !found {
			status.Backups = append(status.Backups, bt)
		}
	}

	return err
}

type updateTaskAction struct {
	clusterID string
	task      *managerclient.Task
	taskSpec  interface{}
}

func (a updateTaskAction) String() string {
	return fmt.Sprintf("update task %+v", a.task)
}

func (a *updateTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *v1.ScyllaClusterStatus) error {
	err := client.UpdateTask(ctx, a.clusterID, a.task)

	if a.task.Type == "repair" {
		for i, repairStatus := range status.Repairs {
			if a.task.ID == repairStatus.ID {
				status.Repairs[i].RepairTaskSpec = a.taskSpec.(v1.RepairTaskSpec)
				if err != nil {
					status.Repairs[i].Error = messageOf(err)
				}
			}
			break
		}
	}
	if a.task.Type == "backup" {
		for i, backupStatus := range status.Backups {
			if a.task.ID == backupStatus.ID {
				status.Backups[i].BackupTaskSpec = a.taskSpec.(v1.BackupTaskSpec)
				if err != nil {
					status.Backups[i].Error = messageOf(err)
				}
			}
			break
		}
	}

	return err
}

// messageOf returns error message embedded in returned error.
func messageOf(err error) string {
	err = errors.Cause(err)
	switch v := err.(type) {
	case interface {
		GetPayload() *models.ErrorResponse
	}:
		return v.GetPayload().Message
	}
	return err.Error()
}
