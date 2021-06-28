package manager

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	helpers "github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) getAuthToken(sc *scyllav1.ScyllaCluster) (string, error) {
	secretName := naming.AgentAuthTokenSecretName(sc.Name)
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
		repairTasks []*RepairTask
		backupTasks []*BackupTask
	)

	if clusterID != "" {
		clusterFound := false
		for _, c := range clusters {
			if c.ID == clusterID {
				clusterFound = true
			}
		}

		if clusterFound {
			managerRepairTasks, err := c.managerClient.ListTasks(ctx, clusterID, "repair", true, "")
			if err != nil {
				return nil, err
			}

			repairTasks = make([]*RepairTask, 0, len(managerRepairTasks.ExtendedTaskSlice))
			for _, managerRepairTask := range managerRepairTasks.ExtendedTaskSlice {
				rt := &RepairTask{}
				if err := rt.FromManager(managerRepairTask); err != nil {
					return nil, err
				}
				repairTasks = append(repairTasks, rt)
			}

			managerBackupTasks, err := c.managerClient.ListTasks(ctx, clusterID, "backup", true, "")
			if err != nil {
				return nil, err
			}

			backupTasks = make([]*BackupTask, 0, len(managerBackupTasks.ExtendedTaskSlice))
			for _, managerBackupTask := range managerBackupTasks.ExtendedTaskSlice {
				bt := &BackupTask{}
				if err := bt.FromManager(managerBackupTask); err != nil {
					return nil, err
				}
				backupTasks = append(backupTasks, bt)
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
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	if sc.DeletionTimestamp != nil {
		return nil
	}

	authToken, err := c.getAuthToken(sc)
	if err != nil {
		return err
	}

	clusterID := ""
	if sc.Status.ManagerID != nil {
		clusterID = *sc.Status.ManagerID
	}
	managerState, err := c.getManagerState(ctx, clusterID)
	if err != nil {
		return err
	}

	actions, requeue, err := runSync(ctx, sc, authToken, managerState)
	if err != nil {
		return err
	}

	scCopy := sc.DeepCopy()

	var actionErrs []error
	for _, a := range actions {
		klog.V(4).InfoS("Executing action", "action", a)
		err = a.Execute(ctx, c.managerClient, &scCopy.Status)
		if err != nil {
			klog.ErrorS(err, "Failed to execute action", "action", a)
			actionErrs = append(actionErrs, err)
		}
	}

	// Update status if needed
	if !apiequality.Semantic.DeepEqual(scCopy.Status, sc.Status) {
		klog.V(4).InfoS("Updating cluster status", "new", scCopy.Status, "old", sc.Status)
		_, err := c.scyllaClient.ScyllaClusters(sc.Namespace).UpdateStatus(ctx, scCopy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	err = utilerrors.NewAggregate(actionErrs)
	if err != nil {
		return err
	}

	if requeue {
		c.queue.AddRateLimited(key)
		return nil
	}

	return nil
}
