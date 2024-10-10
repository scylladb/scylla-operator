package manager

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
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

func (c *Controller) getManagerState(ctx context.Context, clusterID string) (*state, error) {
	clusters, err := c.managerClient.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	var (
		repairTasks map[string]scyllav1.RepairTaskStatus
		backupTasks map[string]scyllav1.BackupTaskStatus
	)

	if clusterID != "" {
		clusterFound := false
		for _, c := range clusters {
			if c.ID == clusterID {
				clusterFound = true
			}
		}

		if clusterFound {
			managerRepairTasks, err := c.managerClient.ListTasks(ctx, clusterID, "repair", true, "", "")
			if err != nil {
				return nil, err
			}

			repairTasks = make(map[string]scyllav1.RepairTaskStatus, len(managerRepairTasks.TaskListItemSlice))
			for _, managerRepairTask := range managerRepairTasks.TaskListItemSlice {
				rts, err := NewRepairStatusFromManager(managerRepairTask)
				if err != nil {
					return nil, err
				}
				repairTasks[rts.Name] = *rts
			}

			managerBackupTasks, err := c.managerClient.ListTasks(ctx, clusterID, "backup", true, "", "")
			if err != nil {
				return nil, err
			}

			backupTasks = make(map[string]scyllav1.BackupTaskStatus, len(managerBackupTasks.TaskListItemSlice))
			for _, managerBackupTask := range managerBackupTasks.TaskListItemSlice {
				bts, err := NewBackupStatusFromManager(managerBackupTask)
				if err != nil {
					return nil, err
				}
				backupTasks[bts.Name] = *bts
			}
		}
	}

	return &state{
		Clusters:    clusters,
		BackupTasks: backupTasks,
		RepairTasks: repairTasks,
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

	clusterID := ""
	if sc.Status.ManagerID != nil {
		clusterID = *sc.Status.ManagerID
	}

	managerState, err := c.getManagerState(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("can't get manager state: %w", err)
	}

	status := c.calculateStatus(sc, managerState)

	if sc.DeletionTimestamp != nil {
		return c.updateStatus(ctx, sc, status)
	}

	authToken, err := c.getAuthToken(sc)
	if err != nil {
		return fmt.Errorf("can't get auth token: %w", err)
	}

	actions, requeue, err := runSync(ctx, sc, authToken, managerState)
	if err != nil {
		return fmt.Errorf("can't run sync: %w", err)
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
