package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) getAuthToken(sc *scyllav1.ScyllaCluster) (string, error) {
	secretName := naming.AgentAuthTokenSecretNameForScyllaCluster(sc)
	secret, err := c.secretLister.Secrets(sc.Namespace).Get(secretName)
	if err != nil {
		return "", fmt.Errorf("can't get manager agent auth secret %s/%s: %w", sc.Namespace, secretName, err)
	}

	return helpers.GetAgentAuthTokenFromSecret(secret)
}

func (c *Controller) getManagerClusterState(ctx context.Context, sc *scyllav1.ScyllaCluster) (*managerClusterState, error) {
	managerClusters, err := c.managerClient.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't list clusters registered with manager: %w", err)
	}

	clusterName := naming.ManagerClusterName(sc)
	// Cluster names in manager state are unique, so it suffices to only find one with a matching name.
	managerCluster, _, found := slices.Find(managerClusters, func(c *models.Cluster) bool {
		return c.Name == clusterName
	})
	if !found {
		return &managerClusterState{}, nil
	}

	ownerUIDLabel := managerCluster.Labels[naming.OwnerUIDLabel]
	if ownerUIDLabel != string(sc.UID) {
		// Despite the label mismatch the cluster needs to be propagated to state so that we can delete it to avoid a name collision.
		return &managerClusterState{
			Cluster: managerCluster,
		}, nil
	}

	// Sanity check.
	if len(managerCluster.ID) == 0 {
		return nil, fmt.Errorf("manager cluster is missing an ID")
	}

	var repairTaskStatuses map[string]scyllav1.RepairTaskStatus
	var backupTaskStatuses map[string]scyllav1.BackupTaskStatus

	var managerRepairTasks managerclient.TaskListItems
	managerRepairTasks, err = c.managerClient.ListTasks(ctx, managerCluster.ID, "repair", true, "", "")
	if err != nil {
		return nil, fmt.Errorf("can't list repair tasks registered with manager: %w", err)
	}

	repairTaskStatuses = make(map[string]scyllav1.RepairTaskStatus, len(managerRepairTasks.TaskListItemSlice))
	for _, managerRepairTask := range managerRepairTasks.TaskListItemSlice {
		var repairTaskStatus *scyllav1.RepairTaskStatus
		repairTaskStatus, err = NewRepairStatusFromManager(managerRepairTask)
		if err != nil {
			return nil, fmt.Errorf("can't get repair task status from manager task: %w", err)
		}
		repairTaskStatuses[repairTaskStatus.Name] = *repairTaskStatus
	}

	var managerBackupTasks managerclient.TaskListItems
	managerBackupTasks, err = c.managerClient.ListTasks(ctx, managerCluster.ID, "backup", true, "", "")
	if err != nil {
		return nil, fmt.Errorf("can't list backup tasks registered with manager: %w", err)
	}

	backupTaskStatuses = make(map[string]scyllav1.BackupTaskStatus, len(managerBackupTasks.TaskListItemSlice))
	for _, managerBackupTask := range managerBackupTasks.TaskListItemSlice {
		var backupTaskStatus *scyllav1.BackupTaskStatus
		backupTaskStatus, err = NewBackupStatusFromManager(managerBackupTask)
		if err != nil {
			return nil, fmt.Errorf("can't get backup task status from manager backup task: %w", err)
		}
		backupTaskStatuses[backupTaskStatus.Name] = *backupTaskStatus
	}

	return &managerClusterState{
		Cluster:     managerCluster,
		BackupTasks: backupTaskStatuses,
		RepairTasks: repairTaskStatuses,
	}, nil
}

func (c *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := c.scyllaLister.ScyllaClusters(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KRef(namespace, name))
		return nil
	}
	if err != nil {
		return err
	}

	state, err := c.getManagerClusterState(ctx, sc)
	if err != nil {
		return fmt.Errorf("can't get manager state for cluster %q: %w", naming.ObjRef(sc), err)
	}

	status := c.calculateStatus(sc, state)

	if sc.DeletionTimestamp != nil {
		return c.updateStatus(ctx, sc, status)
	}

	authToken, err := c.getAuthToken(sc)
	if err != nil {
		return fmt.Errorf("can't get auth token for cluster %q: %w", naming.ObjRef(sc), err)
	}

	actions, requeue, err := runSync(ctx, sc, authToken, state)
	if err != nil {
		return fmt.Errorf("can't run sync for cluster %q: %w", naming.ObjRef(sc), err)
	}

	var errs []error
	for _, a := range actions {
		klog.V(4).InfoS("Executing action", "action", a)
		err = a.Execute(ctx, c.managerClient, status)
		if err != nil {
			klog.ErrorS(err, "Failed to execute action", "action", a)
			errs = append(errs, fmt.Errorf("can't execute action: %w", err))
		} else {
			requeue = true
		}
	}

	err = c.updateStatus(ctx, sc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
		return utilerrors.NewAggregate(errs)
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	if requeue {
		c.queue.AddRateLimited(key)
		return nil
	}

	return nil
}
