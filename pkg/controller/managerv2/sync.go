package managerv2

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var managerClientsCache = MakeClientsCache()

func (smc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaManager", "ScyllaManager", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaManager", "ScyllaManager", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sm, err := smc.scyllaManagerLister.ScyllaManagers(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaManager has been deleted", "ScyllaManager", key)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to list ScyllaManager, namespace %v, name %v: %v", namespace, name, err)
	}

	deploymentsMap, err := smc.getDeployments(ctx, sm)
	if err != nil {
		return err
	}

	serviceMap, err := smc.getServices(ctx, sm)
	if err != nil {
		return err
	}

	pdbMap, err := smc.getPDBs(ctx, sm)
	if err != nil {
		return err
	}

	secretMap, err := smc.getSecrets(ctx, sm)
	if err != nil {
		return err
	}

	configMaps, err := smc.getConfigMaps(ctx, sm)
	if err != nil {
		return err
	}

	scyllaClusters, err := smc.getScyllaClusters(sm)
	if err != nil {
		return err
	}

	managerReady := smc.isManagerReady(sm, deploymentsMap)

	client, err := managerClientsCache.Get(sm)
	if err != nil {
		return err
	}

	managedClusters, err := smc.getManagedClusters(ctx, client, managerReady)
	if err != nil {
		return err
	}

	managedRepairTasks, err := smc.getRepairTasks(ctx, client, managedClusters, managerReady)
	if err != nil {
		return err
	}

	managedBackupTasks, err := smc.getBackupTasks(ctx, client, managedClusters, managerReady)
	if err != nil {
		return err
	}

	var errs []error

	status := smc.calculateStatus(sm, deploymentsMap, scyllaClusters, managedClusters, managedRepairTasks, managedBackupTasks)

	if sm.DeletionTimestamp != nil {
		return smc.updateStatus(ctx, sm, status)
	}

	err = smc.syncSecrets(ctx, sm, secretMap)
	errs = append(errs, err)

	err = smc.syncConfigMaps(ctx, sm, configMaps)
	errs = append(errs, err)

	err = smc.syncPodDisruptionBudgets(ctx, sm, pdbMap)
	errs = append(errs, err)

	err = smc.syncDeployments(ctx, sm, deploymentsMap)
	errs = append(errs, err)

	err = smc.syncServices(ctx, sm, serviceMap)
	errs = append(errs, err)

	var syncedClusters []*managerclient.Cluster
	status, syncedClusters, err = smc.syncClusters(ctx, client, managedClusters, scyllaClusters, status, managerReady)
	errs = append(errs, err)

	status, err = smc.syncRepairTasks(ctx, status, sm, client, smc.reduceTasks(managedRepairTasks), syncedClusters, managerReady)
	errs = append(errs, err)

	status, err = smc.syncBackupTasks(ctx, status, sm, client, smc.reduceTasks(managedBackupTasks), syncedClusters, managerReady)
	errs = append(errs, err)

	err = smc.updateStatus(ctx, sm, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func (smc *Controller) getDeployments(ctx context.Context, sm *v1alpha1.ScyllaManager) (map[string]*v1.Deployment, error) {
	deployments, err := smc.deploymentLister.Deployments(sm.Namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't deployments: %v", err)
	}

	selector := labels.SelectorFromSet(naming.ManagerLabels(sm))

	canAdoptFunc := func() error {
		fresh, err := smc.scyllaManagerLister.ScyllaManagers(sm.Namespace).Get(sm.Name)
		if err != nil {
			return err
		}

		if fresh.UID != sm.UID {
			return fmt.Errorf("original ScyllaManager %v/%v is gone: got uid %v, wanted %v", sm.Namespace, sm.Name, fresh.UID, sm.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sm.Namespace, sm.Name, sm.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewDeploymentControllerRefManager(
		ctx,
		sm,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealDeploymentControl{
			KubeClient: smc.kubeClient,
			Recorder:   smc.eventRecorder,
		},
	)
	return cm.ClaimDeployments(deployments)
}

func (smc *Controller) getServices(ctx context.Context, sm *v1alpha1.ScyllaManager) (map[string]*corev1.Service, error) {
	// List all Services to find even those that no longer match our selector.
	// They will be orphaned in ClaimServices().
	services, err := smc.serviceLister.Services(sm.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(naming.ManagerLabels(sm))

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Services.
	canAdoptFunc := func() error {
		fresh, err := smc.scyllaManagerLister.ScyllaManagers(sm.Namespace).Get(sm.Name)
		if err != nil {
			return err
		}

		if fresh.UID != sm.UID {
			return fmt.Errorf("original ScyllaManager %v/%v is gone: got uid %v, wanted %v", sm.Namespace, sm.Name, fresh.UID, sm.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sm.Namespace, sm.Name, sm.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceControllerRefManager(
		ctx,
		sm,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceControl{
			KubeClient: smc.kubeClient,
			Recorder:   smc.eventRecorder,
		},
	)
	return cm.ClaimServices(services)
}

func (smc *Controller) getSecrets(ctx context.Context, sm *v1alpha1.ScyllaManager) (map[string]*corev1.Secret, error) {
	// List all Secrets to find even those that no longer match our selector.
	// They will be orphaned in ClaimSecrets().
	secrets, err := smc.secretLister.Secrets(sm.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(naming.ManagerLabels(sm))

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Secrets.
	canAdoptFunc := func() error {
		fresh, err := smc.scyllaManagerLister.ScyllaManagers(sm.Namespace).Get(sm.Name)
		if err != nil {
			return err
		}

		if fresh.UID != sm.UID {
			return fmt.Errorf("original ScyllaManager %v/%v is gone: got uid %v, wanted %v", sm.Namespace, sm.Name, fresh.UID, sm.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sm.Namespace, sm.Name, sm.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewSecretControllerRefManager(
		ctx,
		sm,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealSecretControl{
			KubeClient: smc.kubeClient,
			Recorder:   smc.eventRecorder,
		},
	)
	return cm.ClaimSecrets(secrets)
}

func (smc *Controller) getPDBs(ctx context.Context, sm *v1alpha1.ScyllaManager) (map[string]*policyv1.PodDisruptionBudget, error) {
	// List all Pdbs to find even those that no longer match our selector.
	// They will be orphaned in ClaimPdbs().
	pdbs, err := smc.pdbLister.PodDisruptionBudgets(sm.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(naming.ManagerLabels(sm))

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pdbs.
	canAdoptFunc := func() error {
		fresh, err := smc.scyllaManagerLister.ScyllaManagers(sm.Namespace).Get(sm.Name)
		if err != nil {
			return err
		}

		if fresh.UID != sm.UID {
			return fmt.Errorf("original ScyllaManager %v/%v is gone: got uid %v, wanted %v", sm.Namespace, sm.Name, fresh.UID, sm.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sm.Namespace, sm.Name, sm.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewPodDisruptionBudgetControllerRefManager(
		ctx,
		sm,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealPodDisruptionBudgetControl{
			KubeClient: smc.kubeClient,
			Recorder:   smc.eventRecorder,
		},
	)
	return cm.ClaimPodDisruptionBudgets(pdbs)
}

func (smc *Controller) getConfigMaps(ctx context.Context, sm *v1alpha1.ScyllaManager) (map[string]*corev1.ConfigMap, error) {
	cfgMaps, err := smc.configMapLister.ConfigMaps(sm.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(naming.ManagerLabels(sm))

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing ConfigMaps.
	canAdoptFunc := func() error {
		fresh, err := smc.scyllaManagerLister.ScyllaManagers(sm.Namespace).Get(sm.Name)
		if err != nil {
			return err
		}

		if fresh.UID != sm.UID {
			return fmt.Errorf("original ScyllaManager %v/%v is gone: got uid %v, wanted %v", sm.Namespace, sm.Name, fresh.UID, sm.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sm.Namespace, sm.Name, sm.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		sm,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: smc.kubeClient,
			Recorder:   smc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(cfgMaps)
}

func (smc *Controller) getManagedClusters(
	ctx context.Context,
	client *managerclient.Client,
	managerReady bool,
) ([]*managerclient.Cluster, error) {
	if !managerReady {
		return nil, nil
	}

	return client.ListClusters(ctx)
}

func (smc *Controller) getScyllaClusters(sm *v1alpha1.ScyllaManager) ([]*scyllav1.ScyllaCluster, error) {
	selector := labels.SelectorFromSet(sm.Spec.ScyllaClusterSelector.MatchLabels)
	return smc.scyllaClusterLister.ScyllaClusters(sm.Namespace).List(selector)
}

func (smc *Controller) isManagerReady(sm *v1alpha1.ScyllaManager, deployments map[string]*v1.Deployment) bool {
	// There has to be a manager deployment to talk with.
	if deployment, ok := deployments[sm.Name]; ok {
		return deployment.Status.ReadyReplicas > 0
	}

	return false
}

func (smc *Controller) getTasks(
	ctx context.Context,
	client *managerclient.Client,
	clusters []*managerclient.Cluster,
	managerReady bool,
	typ string,
) ([]*managerclient.ExtendedTask, error) {
	if !managerReady {
		return nil, nil
	}

	tasks := make([]*managerclient.ExtendedTask, 0)
	for _, cluster := range clusters {
		l, err := client.ListTasks(ctx, cluster.ID, typ, true, "")
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, l.ExtendedTaskSlice...)
	}

	return tasks, nil

}

func (smc *Controller) getRepairTasks(
	ctx context.Context,
	client *managerclient.Client,
	clusters []*managerclient.Cluster,
	managerReady bool,
) ([]*managerclient.ExtendedTask, error) {
	return smc.getTasks(ctx, client, clusters, managerReady, naming.RepairTask)
}

func (smc *Controller) getBackupTasks(
	ctx context.Context,
	client *managerclient.Client,
	clusters []*managerclient.Cluster,
	managerReady bool,
) ([]*managerclient.ExtendedTask, error) {
	return smc.getTasks(ctx, client, clusters, managerReady, naming.BackupTask)
}

func (smc *Controller) reduceTasks(tasks []*managerclient.ExtendedTask) []*managerclient.Task {
	res := make([]*managerclient.Task, 0, len(tasks))
	for _, task := range tasks {
		res = append(res, &managerclient.Task{
			ID:         task.ID,
			ClusterID:  task.ClusterID,
			Name:       task.Name,
			Properties: task.Properties,
			Schedule:   task.Schedule,
			Type:       task.Type,
			Enabled:    task.Enabled,
			Tags:       task.Tags,
		})
	}
	return res
}

func (smc *Controller) pruneTasks(
	ctx context.Context,
	client *managerclient.Client,
	tasks, requiredTasks []*managerclient.Task,
) error {
	var errs []error

	for _, task := range tasks {
		remove := true
		for _, requiredTask := range requiredTasks {
			if task.Name == requiredTask.Name && task.ClusterID == requiredTask.ClusterID && task.Type == requiredTask.Type {
				remove = false
			}
		}

		if remove {
			err := client.DeleteTask(ctx, task.ClusterID, task.Type, uuid.MustParse(task.ID))
			if err != nil {
				errs = append(errs, fmt.Errorf("deleting task %v of type %v, clusterID %v, %v", task.Name, task.Type, task.ClusterID, err))
			} else {
				klog.V(2).InfoS("deleted task", "type", task.Type, "task", task.Name, "clusterID", task.ClusterID)
			}
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (smc *Controller) scheduleNextStatusUpdate(sm *v1alpha1.ScyllaManager, tasks []*managerclient.ExtendedTask) error {
	key, err := keyFunc(sm)
	if err != nil {
		return err
	}

	duration := time.Duration(0)
	for _, task := range tasks {
		d, ok := smc.nextTaskStatusUpdate(sm, task)
		if ok {
			if d < duration || duration == 0 {
				duration = d
			}
		}
	}

	if duration > 0 {
		smc.queue.AddAfter(key, duration)
		klog.V(2).InfoS("Added key to refresh tasks statuses", "key", key, "duration[s]", duration.Seconds())
	}
	return nil
}

// TODO: What to do with suspended tasks?
func (smc *Controller) nextTaskStatusUpdate(sm *v1alpha1.ScyllaManager, t *managerclient.ExtendedTask) (time.Duration, bool) {
	if !t.Enabled {
		return time.Duration(0), false
	}

	if t.NextActivation != nil {
		return time.Time(*t.NextActivation).Sub(time.Now()), true
	}

	if v1alpha1.TaskStatusType(t.Status) != v1alpha1.TaskStatusDone {
		return sm.Spec.TaskStatusRefreshTime, true
	}

	return time.Duration(0), false
}
