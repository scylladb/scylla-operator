// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type managerClusterState struct {
	Cluster     *managerclient.Cluster
	RepairTasks map[string]scyllav1.RepairTaskStatus
	BackupTasks map[string]scyllav1.BackupTaskStatus
}

func runSync(ctx context.Context, sc *scyllav1.ScyllaCluster, authToken string, managerClusterState *managerClusterState) ([]action, bool, error) {
	clusterAction, err := syncCluster(sc, authToken, managerClusterState.Cluster)
	if err != nil {
		return nil, false, fmt.Errorf("can't sync cluster %q: %w", naming.ObjRef(sc), err)
	}
	if clusterAction != nil {
		// Execute the cluster action first and requeue to avoid potential errors in task execution in the same iteration.
		return []action{clusterAction}, true, nil
	}

	var taskActions []action
	taskActions, err = syncTasks(managerClusterState.Cluster.ID, sc, managerClusterState)
	if err != nil {
		return nil, false, fmt.Errorf("can't sync tasks for cluster %q: %w", naming.ObjRef(sc), err)
	}

	return taskActions, false, nil
}

func syncCluster(sc *scyllav1.ScyllaCluster, authToken string, existingManagerCluster *managerclient.Cluster) (action, error) {
	clusterName := naming.ManagerClusterName(sc)
	managerCluster := &managerclient.Cluster{
		Name:      clusterName,
		Host:      naming.CrossNamespaceServiceNameForCluster(sc),
		AuthToken: authToken,
		// TODO: enable CQL over TLS when https://github.com/scylladb/scylla-operator/issues/1673 is completed
		ForceNonSslSessionPort: true,
		ForceTLSDisabled:       true,
		Labels: map[string]string{
			naming.OwnerUIDLabel: string(sc.UID),
		},
	}

	managedHash, err := hashutil.HashObjects(managerCluster)
	if err != nil {
		return nil, fmt.Errorf("can't calculate managed hash for cluster %q: %w", clusterName, err)
	}
	managerCluster.Labels[naming.ManagedHash] = managedHash

	if existingManagerCluster == nil {
		return &addClusterAction{
			Cluster: managerCluster,
		}, nil
	}

	// Sanity check.
	if len(existingManagerCluster.ID) == 0 {
		return nil, fmt.Errorf("manager cluster is missing an id")
	}

	existingManagerClusterOwnerUIDLabelValue, existingManagerClusterHasOwnerUIDLabel := existingManagerCluster.Labels[naming.OwnerUIDLabel]
	if !existingManagerClusterHasOwnerUIDLabel {
		klog.Warningf("Cluster %q is missing an owner UID label.", existingManagerCluster.Name)
	}

	if existingManagerClusterOwnerUIDLabelValue != string(sc.UID) {
		klog.V(4).InfoS("Cluster is in manager state, but it's not owned by ScyllaCluster. Scheduling it for deletion to avoid a name collision.", "ClusterName", existingManagerCluster.Name, "ScyllaCluster", naming.ObjRef(sc))

		// We're not certain the cluster is owned by us - we have to delete it to avoid a name collision.
		return &deleteClusterAction{
			ClusterID: existingManagerCluster.ID,
		}, nil
	}

	if existingManagerCluster.Labels != nil && managedHash == existingManagerCluster.Labels[naming.ManagedHash] {
		// Cluster matches the desired state, nothing to do.
		return nil, nil
	}

	managerCluster.ID = existingManagerCluster.ID
	return &updateClusterAction{
		Cluster: managerCluster,
	}, nil
}

func syncTasks(clusterID string, sc *scyllav1.ScyllaCluster, state *managerClusterState) ([]action, error) {
	var errs []error
	var actions []action

	repairTaskSpecNames := sets.New(slices.ConvertSlice(sc.Spec.Repairs, func(b scyllav1.RepairTaskSpec) string {
		return b.Name
	})...)
	for taskName, task := range state.RepairTasks {
		if repairTaskSpecNames.Has(taskName) {
			continue
		}

		actions = append(actions, &deleteTaskAction{
			ClusterID: clusterID,
			TaskType:  managerclient.RepairTask,
			TaskID:    *task.ID,
		})
	}

	backupTaskSpecNames := sets.New(slices.ConvertSlice(sc.Spec.Backups, func(b scyllav1.BackupTaskSpec) string {
		return b.Name
	})...)
	for taskName, task := range state.BackupTasks {
		if backupTaskSpecNames.Has(taskName) {
			continue
		}

		actions = append(actions, &deleteTaskAction{
			ClusterID: clusterID,
			TaskType:  managerclient.BackupTask,
			TaskID:    *task.ID,
		})
	}

	repairActions, err := syncRepairTasks(clusterID, sc, state)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync repair tasks: %w", err))
	}
	actions = append(actions, repairActions...)

	backupActions, err := syncBackupTasks(clusterID, sc, state)
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

func syncBackupTasks(clusterID string, cluster *scyllav1.ScyllaCluster, state *managerClusterState) ([]action, error) {
	var errs []error
	var actions []action

	for _, bt := range cluster.Spec.Backups {
		taskStatusFunc := func() (*scyllav1.TaskStatus, bool) {
			s, ok := state.BackupTasks[bt.Name]
			if !ok {
				return nil, false
			}

			return &s.TaskStatus, true
		}

		backupTaskSpecCopy := bt.DeepCopy()
		backupTaskSpec := BackupTaskSpec(*backupTaskSpecCopy)

		a, err := syncTask(clusterID, &backupTaskSpec, taskStatusFunc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync backup task %q: %w", bt.Name, err))
			continue
		}

		if a != nil {
			actions = append(actions, a)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func syncRepairTasks(clusterID string, cluster *scyllav1.ScyllaCluster, state *managerClusterState) ([]action, error) {
	var errs []error
	var actions []action

	for _, rt := range cluster.Spec.Repairs {
		taskStatusFunc := func() (*scyllav1.TaskStatus, bool) {
			s, ok := state.RepairTasks[rt.Name]
			if !ok {
				return nil, false
			}

			return &s.TaskStatus, true
		}

		repairTaskSpecCopy := rt.DeepCopy()
		repairTaskSpec := RepairTaskSpec(*repairTaskSpecCopy)

		a, err := syncTask(clusterID, &repairTaskSpec, taskStatusFunc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync repair task %q: %w", rt.Name, err))
			continue
		}

		if a != nil {
			actions = append(actions, a)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func syncTask(clusterID string, spec taskSpecInterface, statusFunc func() (*scyllav1.TaskStatus, bool)) (action, error) {
	managedHash, err := spec.GetObjectHash()
	if err != nil {
		return nil, fmt.Errorf("can't get object hash: %w", err)
	}

	var managerClientTask *managerclient.Task
	status, ok := statusFunc()
	if !ok {
		managerClientTask, err = spec.ToManager()
		if err != nil {
			return nil, fmt.Errorf("can't convert task to manager task: %w", err)
		}
		setManagerClientTaskManagedHashLabel(managerClientTask, managedHash)

		return &addTaskAction{
			ClusterID: clusterID,
			Task:      managerClientTask,
		}, nil
	}

	if managedHash == status.Labels[naming.ManagedHash] {
		// Tasks are equal, do nothing.
		return nil, nil
	}

	evaluateDates(spec.GetTaskSpec(), status)
	managerClientTask, err = spec.ToManager()
	if err != nil {
		return nil, fmt.Errorf("can't convert task to manager task: %w", err)
	}
	setManagerClientTaskManagedHashLabel(managerClientTask, managedHash)

	if status.ID == nil {
		// Sanity check.
		return nil, fmt.Errorf("manager task status is missing an id")
	}
	managerClientTask.ID = *status.ID

	return &updateTaskAction{
		ClusterID: clusterID,
		Task:      managerClientTask,
	}, nil
}

func evaluateDates(spec *scyllav1.TaskSpec, taskStatus *scyllav1.TaskStatus) {
	var specStartDate string
	if spec.StartDate != nil {
		specStartDate = *spec.StartDate
	}

	// Keep special "now" value evaluated on task creation.
	if len(specStartDate) == 0 || strings.HasPrefix(specStartDate, "now") {
		var statusStartDate string
		if taskStatus.StartDate != nil {
			statusStartDate = *taskStatus.StartDate
		}

		spec.StartDate = &statusStartDate
	}
}

type action interface {
	Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error
}

type addClusterAction struct {
	Cluster *managerclient.Cluster
}

func (a *addClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	id, err := client.CreateCluster(ctx, a.Cluster)
	if err != nil {
		return fmt.Errorf("can't create cluster %q: %w", a.Cluster.Name, err)
	} else {
		status.ManagerID = &id
	}

	return nil
}

func (a *addClusterAction) String() string {
	return fmt.Sprintf("add cluster %q", a.Cluster.Name)
}

type updateClusterAction struct {
	Cluster *managerclient.Cluster
}

func (a *updateClusterAction) Execute(ctx context.Context, client *managerclient.Client, _ *scyllav1.ScyllaClusterStatus) error {
	err := client.UpdateCluster(ctx, a.Cluster)
	if err != nil {
		return fmt.Errorf("can't update cluster %q: %w", a.Cluster.ID, err)
	}

	return nil
}

func (a *updateClusterAction) String() string {
	return fmt.Sprintf("update cluster %q", a.Cluster.ID)
}

type deleteClusterAction struct {
	ClusterID string
}

func (a *deleteClusterAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := client.DeleteCluster(ctx, a.ClusterID)
	if err != nil {
		return fmt.Errorf("can't delete cluster %q: %w", a.ClusterID, err)
	}

	return nil
}

func (a *deleteClusterAction) String() string {
	return fmt.Sprintf("delete cluster %q", a.ClusterID)
}

type deleteTaskAction struct {
	ClusterID string
	TaskType  string
	TaskID    string
	TaskName  string
}

func (a *deleteTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := a.stopAndDeleteTask(ctx, client)
	if err != nil {
		setTaskStatusError(a.TaskType, a.TaskName, messageOf(err), status)
		return fmt.Errorf("can't stop and delete task %q: %w", a.TaskName, err)
	}

	return nil
}

func (a *deleteTaskAction) stopAndDeleteTask(ctx context.Context, client *managerclient.Client) error {
	// StopTask is idempotent
	err := client.StopTask(ctx, a.ClusterID, a.TaskType, uuid.MustParse(a.TaskID), false)
	if err != nil {
		return fmt.Errorf("can't stop task %q: %w", a.TaskID, err)
	}

	err = client.DeleteTask(ctx, a.ClusterID, a.TaskType, uuid.MustParse(a.TaskID))
	if err != nil {
		return fmt.Errorf("can't delete task %q: %w", a.TaskID, err)
	}

	return nil
}

func (a *deleteTaskAction) String() string {
	return fmt.Sprintf("delete task %q", a.TaskID)
}

type addTaskAction struct {
	ClusterID string
	Task      *managerclient.Task
}

func (a *addTaskAction) String() string {
	return fmt.Sprintf("add task %+v", a.Task)
}

func (a *addTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	_, err := client.CreateTask(ctx, a.ClusterID, a.Task)
	if err != nil {
		setTaskStatusError(a.Task.Type, a.Task.Name, messageOf(err), status)
		return fmt.Errorf("can't create task %q: %w", a.Task.Name, err)
	}

	return nil
}

type updateTaskAction struct {
	ClusterID string
	Task      *managerclient.Task
}

func (a *updateTaskAction) String() string {
	return fmt.Sprintf("update task %+v", a.Task)
}

func (a *updateTaskAction) Execute(ctx context.Context, client *managerclient.Client, status *scyllav1.ScyllaClusterStatus) error {
	err := client.UpdateTask(ctx, a.ClusterID, a.Task)
	if err != nil {
		setTaskStatusError(a.Task.Type, a.Task.Name, messageOf(err), status)
		return fmt.Errorf("can't update task %q: %w", a.Task.Name, err)
	}

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

func setManagerClientTaskManagedHashLabel(task *managerclient.Task, hash string) {
	if task.Labels == nil {
		task.Labels = map[string]string{}
	}
	task.Labels[naming.ManagedHash] = hash
}
