package managerv2

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (smc *Controller) syncBackupTasks(
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

	requiredTasks := MakeBackupTasks(sm, clusters)

	err := smc.pruneTasks(ctx, client, tasks, requiredTasks)
	errs = append(errs, err)

	for _, requiredTask := range requiredTasks {
		_, _, err = resourceapply.ApplyTask(ctx, client, requiredTask, tasks)
		errs = append(errs, err)
	}

	registeredBackups, err := smc.getBackupTasks(ctx, client, clusters, managerReady)
	errs = append(errs, err)
	if err != nil {
		return status, utilerrors.NewAggregate(errs)
	}

	status = smc.calculateClustersBackups(status, registeredBackups, clusters)

	err = smc.scheduleNextStatusUpdate(sm, registeredBackups)
	errs = append(errs, err)

	return status, utilerrors.NewAggregate(errs)
}

// CREATE KEYSPACE test WITH replication = {'class': 'SimpleStrategy', 'replication_factor' : 1};
// CREATE TABLE test.tab (pk bigint PRIMARY KEY, v1 bigint, v2 bigint);
// INSERT INTO test.tab (pk, v1, v2) VALUES (1, 2, 3);
