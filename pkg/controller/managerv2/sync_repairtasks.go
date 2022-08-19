package managerv2

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (smc *Controller) syncRepairTasks(
	ctx context.Context,
	status *v1alpha1.ScyllaManagerStatus,
	sm *v1alpha1.ScyllaManager,
	client *managerclient.Client,
	tasks []*managerclient.Task,
	clusters []*managerclient.Cluster,
	managerReady bool,
) (*v1alpha1.ScyllaManagerStatus, error) {
	if !managerReady {
		return status, nil
	}

	var errs []error

	requiredTasks, err := MakeRepairTasks(sm, clusters)
	errs = append(errs, err)

	err = smc.pruneTasks(ctx, client, tasks, requiredTasks)
	errs = append(errs, err)

	for _, requiredTask := range requiredTasks {
		_, _, err = resourceapply.ApplyTask(ctx, client, requiredTask, tasks)
		errs = append(errs, err)
	}

	registeredRepairs, err := smc.getRepairTasks(ctx, client, clusters, managerReady)
	errs = append(errs, err)
	if err != nil {
		return status, utilerrors.NewAggregate(errs)
	}

	status = smc.calculateClustersRepairs(status, registeredRepairs, clusters)

	err = smc.scheduleNextStatusUpdate(sm, registeredRepairs)
	errs = append(errs, err)

	return status, utilerrors.NewAggregate(errs)
}
