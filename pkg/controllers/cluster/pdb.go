// Copyright (C) 2021 ScyllaDB

package cluster

import (
	"context"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/util/resourceapply"
)

func (cc ClusterReconciler) syncPodDisruptionBudget(ctx context.Context, cluster *scyllav1.ScyllaCluster) error {
	pdb := resource.MakePodDisruptionBudget(cluster)
	if err := resourceapply.ApplyPodDisruptionBudget(ctx, cc.Recorder, cc.Client, pdb); err != nil {
		return errors.Wrap(err, "apply pod disruption budget")
	}

	return nil
}
