// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

type state struct {
	Clusters    []*managerclient.Cluster
	RepairTasks map[string]RepairTaskStatus
	BackupTasks map[string]BackupTaskStatus
}

func runSync(ctx context.Context, cluster *scyllav1.ScyllaCluster, authToken string, state *state) ([]action, bool, error) {
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
							ID:        c.ID,
							Name:      naming.ManagerClusterName(cluster),
							Host:      naming.CrossNamespaceServiceNameForCluster(cluster),
							AuthToken: authToken,
							// TODO: enable CQL over TLS when https://github.com/scylladb/scylla-operator/issues/1766 is completed
							ForceNonSslSessionPort: true,
							ForceTLSDisabled:       true,
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
				// TODO: enable CQL over TLS when https://github.com/scylladb/scylla-operator/issues/1766 is completed
				ForceNonSslSessionPort: true,
				ForceTLSDisabled:       true,
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

func syncTasks(clusterID string, cluster *scyllav1.ScyllaCluster, state *state) ([]action, error) {
	var errs []error
	var actions []action

	repairTaskSpecNames := sets.New(slices.ConvertSlice(cluster.Spec.Repairs, func(b scyllav1.RepairTaskSpec) string {
		return b.Name
	})...)
	for taskName, task := range state.RepairTasks {
		if repairTaskSpecNames.Has(taskName) {
			continue
		}

		actions = append(actions, &deleteTaskAction{
			clusterID: clusterID,
			taskType:  managerclient.RepairTask,
			taskID:    *task.ID,
		})
	}

	backupTaskSpecNames := sets.New(slices.ConvertSlice(cluster.Spec.Backups, func(b scyllav1.BackupTaskSpec) string {
		return b.Name
	})...)
	for taskName, task := range state.BackupTasks {
		if backupTaskSpecNames.Has(taskName) {
			continue
		}

		actions = append(actions, &deleteTaskAction{
			clusterID: clusterID,
			taskType:  managerclient.BackupTask,
			taskID:    *task.ID,
		})
	}

	repairActions, err := syncRepairTasks(clusterID, cluster, state)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync repair tasks: %w", err))
	}
	actions = append(actions, repairActions...)

	backupActions, err := syncBackupTasks(clusterID, cluster, state)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync backup tasks: %w", err))
	}
	actions = append(actions, backupActions...)

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func syncBackupTasks(clusterID string, cluster *scyllav1.ScyllaCluster, managerState *state) ([]action, error) {
	var errs []error
	var actions []action

	for _, bt := range cluster.Spec.Backups {
		action, err := syncBackupTask(clusterID, managerState, &bt)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync backup task %q: %w", bt.Name, err))
			continue
		}

		if action != nil {
			actions = append(actions, action)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func syncBackupTask(clusterID string, managerState *state, backupTaskSpec *scyllav1.BackupTaskSpec) (action, error) {
	backupTaskSpecCopy := backupTaskSpec.DeepCopy()
	backupTask := BackupTaskSpec(*backupTaskSpecCopy)

	managerTaskStatus, ok := managerState.BackupTasks[backupTask.Name]
	if !ok {
		managerClientTask, err := backupTask.ToManager()
		if err != nil {
			return nil, fmt.Errorf("can't convert backup task to manager task: %w", err)
		}

		return &addTaskAction{
			clusterID: clusterID,
			task:      managerClientTask,
		}, nil
	}

	if managerTaskStatus.ID == nil {
		// Sanity check.
		return nil, fmt.Errorf("manager task status is missing an id")
	}

	evaluateDates(&backupTask, &managerTaskStatus)

	// FIXME: Task spec is converted to status to compare it with its current state in manager.
	// This is a temporary workaround and should be replaced with hash comparison when manager allows for storing metadata.
	// Ref: https://github.com/scylladb/scylla-operator/issues/1827.
	if isBackupTaskDeepEqual(&backupTask, &managerTaskStatus) {
		return nil, nil
	}

	managerClientTask, err := backupTask.ToManager()
	if err != nil {
		return nil, fmt.Errorf("can't convert backup task to manager task: %w", err)
	}
	managerClientTask.ID = *managerTaskStatus.ID

	return &updateTaskAction{
		clusterID: clusterID,
		task:      managerClientTask,
	}, nil
}

func isBackupTaskDeepEqual(backupTaskSpec *BackupTaskSpec, managerBackupTaskStatus *BackupTaskStatus) bool {
	backupTaskStatus := backupTaskSpec.ToStatus()
	backupTaskStatus.ID = managerBackupTaskStatus.ID
	return reflect.DeepEqual(backupTaskStatus, managerBackupTaskStatus)
}

func syncRepairTasks(clusterID string, cluster *scyllav1.ScyllaCluster, managerState *state) ([]action, error) {
	var errs []error
	var actions []action

	for _, rt := range cluster.Spec.Repairs {
		action, err := syncRepairTask(clusterID, managerState, &rt)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync repair task %q: %w", rt.Name, err))
			continue
		}

		if action != nil {
			actions = append(actions, action)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func syncRepairTask(clusterID string, managerState *state, repairTaskSpec *scyllav1.RepairTaskSpec) (action, error) {
	repairTaskSpecCopy := repairTaskSpec.DeepCopy()
	repairTask := RepairTaskSpec(*repairTaskSpecCopy)

	managerTaskStatus, ok := managerState.RepairTasks[repairTask.Name]
	if !ok {
		managerClientTask, err := repairTask.ToManager()
		if err != nil {
			return nil, fmt.Errorf("can't convert repair task to manager task: %w", err)
		}

		return &addTaskAction{
			clusterID: clusterID,
			task:      managerClientTask,
		}, nil
	}

	if managerTaskStatus.ID == nil {
		// Sanity check.
		return nil, fmt.Errorf("manager task status is missing an id")
	}

	evaluateDates(&repairTask, &managerTaskStatus)

	// FIXME: Task spec is converted to status to compare it with its current state in manager.
	// This is a temporary workaround and should be replaced with hash comparison when manager allows for storing metadata.
	// Ref: https://github.com/scylladb/scylla-operator/issues/1827.
	if isRepairTaskDeepEqual(&repairTask, &managerTaskStatus) {
		return nil, nil
	}

	managerClientTask, err := repairTask.ToManager()
	if err != nil {
		return nil, fmt.Errorf("can't convert repair task to manager task: %w", err)
	}
	managerClientTask.ID = *managerTaskStatus.ID

	return &updateTaskAction{
		clusterID: clusterID,
		task:      managerClientTask,
	}, nil
}

func isRepairTaskDeepEqual(repairTaskSpec *RepairTaskSpec, managerRepairTaskStatus *RepairTaskStatus) bool {
	repairTaskStatus := repairTaskSpec.ToStatus()
	repairTaskStatus.ID = managerRepairTaskStatus.ID
	return reflect.DeepEqual(repairTaskStatus, managerRepairTaskStatus)
}

func evaluateDates(spec startDateGetterSetter, managerTask startDateGetter) {
	startDate := spec.GetStartDateOrEmpty()
	// Keep special "now" value evaluated on task creation.
	if len(startDate) == 0 || strings.HasPrefix(startDate, "now") {
		spec.SetStartDate(managerTask.GetStartDateOrEmpty())
	}
}

type action interface {
	Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error
}

type addClusterAction struct {
	cluster   *managerclient.Cluster
	clusterID string
}

func (a *addClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	id, err := client.CreateCluster(ctx, a.cluster)
	if err != nil {
		return fmt.Errorf("can't create cluster %q: %w", a.cluster.Name, err)
	} else {
		status.ManagerID = &id
	}

	return nil
}

func (a *addClusterAction) String() string {
	return fmt.Sprintf("add cluster %q", a.clusterID)
}

type updateClusterAction struct {
	cluster *managerclient.Cluster
}

func (a *updateClusterAction) Execute(ctx context.Context, client *managerclient.Client, _ *scyllav1.ScyllaClusterStatus) error {
	err := client.UpdateCluster(ctx, a.cluster)
	if err != nil {
		return fmt.Errorf("can't update cluster %q: %w", a.cluster.ID, err)
	}

	return nil
}

func (a *updateClusterAction) String() string {
	return fmt.Sprintf("update cluster %q", a.cluster.ID)
}

type deleteClusterAction struct {
	clusterID string
}

func (a *deleteClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := client.DeleteCluster(ctx, a.clusterID)
	if err != nil {
		return fmt.Errorf("can't delete cluster %q: %w", a.clusterID, err)
	}

	return nil
}

func (a *deleteClusterAction) String() string {
	return fmt.Sprintf("delete cluster %q", a.clusterID)
}

type deleteTaskAction struct {
	clusterID string
	taskType  string
	taskID    string
	taskName  string
}

func (a *deleteTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := a.stopAndDeleteTask(ctx, client)
	if err != nil {
		setTaskStatusError(a.taskType, a.taskName, messageOf(err), status)
		return fmt.Errorf("can't stop and delete task %q: %w", a.taskName, err)
	}

	clearTaskStatusError(a.taskType, a.taskName, status)

	return nil
}

func (a *deleteTaskAction) stopAndDeleteTask(ctx context.Context, client *managerclient.Client) error {
	// StopTask is idempotent
	err := client.StopTask(ctx, a.clusterID, a.taskType, uuid.MustParse(a.taskID), false)
	if err != nil {
		return fmt.Errorf("can't stop task %q: %w", a.taskID, err)
	}

	err = client.DeleteTask(ctx, a.clusterID, a.taskType, uuid.MustParse(a.taskID))
	if err != nil {
		return fmt.Errorf("can't delete task %q: %w", a.taskID, err)
	}

	return nil
}

func (a *deleteTaskAction) String() string {
	return fmt.Sprintf("delete task %q", a.taskID)
}

type addTaskAction struct {
	clusterID string
	task      *managerclient.Task
}

func (a *addTaskAction) String() string {
	return fmt.Sprintf("add task %+v", a.task)
}

func (a *addTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	_, err := client.CreateTask(ctx, a.clusterID, a.task)
	if err != nil {
		setTaskStatusError(a.task.Type, a.task.Name, messageOf(err), status)
		return fmt.Errorf("can't create task %q: %w", a.task.Name, err)
	}

	clearTaskStatusError(a.task.Type, a.task.Name, status)

	return nil
}

type updateTaskAction struct {
	clusterID string
	task      *managerclient.Task
}

func (a *updateTaskAction) String() string {
	return fmt.Sprintf("update task %+v", a.task)
}

func (a *updateTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := client.UpdateTask(ctx, a.clusterID, a.task)
	if err != nil {
		setTaskStatusError(a.task.Type, a.task.Name, messageOf(err), status)
		return fmt.Errorf("can't update task %q: %w", a.task.Name, err)
	}

	clearTaskStatusError(a.task.Type, a.task.Name, status)

	return nil
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

func clearTaskStatusError(taskType string, taskName string, status *scyllav1.ScyllaClusterStatus) {
	switch taskType {
	case managerclient.RepairTask:
		_, i, ok := slices.Find(status.Repairs, func(rts scyllav1.RepairTaskStatus) bool {
			return rts.Name == taskName
		})
		if ok {
			status.Repairs[i].Error = nil
		}
	case managerclient.BackupTask:
		_, i, ok := slices.Find(status.Backups, func(bts scyllav1.BackupTaskStatus) bool {
			return bts.Name == taskName
		})
		if ok {
			status.Backups[i].Error = nil
		}
	}
}

func setTaskStatusError(taskType string, taskName string, taskErr string, status *scyllav1.ScyllaClusterStatus) {
	switch taskType {
	case managerclient.RepairTask:
		rts := scyllav1.RepairTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name:  taskName,
				Error: &taskErr,
			},
		}

		updateRepairTaskStatusError(&status.Repairs, rts)
	case managerclient.BackupTask:
		bts := scyllav1.BackupTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name:  taskName,
				Error: &taskErr,
			},
		}

		updateBackupTaskStatusError(&status.Backups, bts)
	}
}

func updateRepairTaskStatusError(repairTaskStatuses *[]scyllav1.RepairTaskStatus, repairTaskStatus scyllav1.RepairTaskStatus) {
	_, i, ok := slices.Find(*repairTaskStatuses, func(rts scyllav1.RepairTaskStatus) bool {
		return rts.Name == repairTaskStatus.Name
	})

	if ok {
		(*repairTaskStatuses)[i].Error = repairTaskStatus.Error
		return
	}

	*repairTaskStatuses = append(*repairTaskStatuses, repairTaskStatus)
}

func updateBackupTaskStatusError(backupTaskStatuses *[]scyllav1.BackupTaskStatus, backupTaskStatus scyllav1.BackupTaskStatus) {
	_, i, ok := slices.Find(*backupTaskStatuses, func(bts scyllav1.BackupTaskStatus) bool {
		return bts.Name == backupTaskStatus.Name
	})

	if ok {
		(*backupTaskStatuses)[i].Error = backupTaskStatus.Error
		return
	}

	*backupTaskStatuses = append(*backupTaskStatuses, backupTaskStatus)
}
