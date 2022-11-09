// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	"context"
	"time"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllav2alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"k8s.io/klog/v2"
)

func RolloutTimeoutForScyllaCluster(sc *scyllav2alpha1.ScyllaCluster) time.Duration {
	return baseRolloutTimout + time.Duration(GetScyllaClusterTotalNodeCount(sc))*memberRolloutTimeout
}

func ContextForScyllaClusterRollout(parent context.Context, sc *scyllav2alpha1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForScyllaCluster(sc))
}

func GetScyllaClusterDatacenterNodeCount(sc *scyllav2alpha1.ScyllaCluster, dc scyllav2alpha1.Datacenter) int32 {
	var nodeCount int32
	if dc.NodesPerRack != nil {
		nodeCount += int32(len(dc.Racks)) * *dc.NodesPerRack
	}
	return nodeCount
}

func GetScyllaClusterTotalNodeCount(sc *scyllav2alpha1.ScyllaCluster) int32 {
	var totalNodeCount int32
	for _, dc := range sc.Spec.Datacenters {
		totalNodeCount += GetScyllaClusterDatacenterNodeCount(sc, dc)
	}
	return totalNodeCount
}

func IsScyllaClusterRolledOut(sc *scyllav2alpha1.ScyllaCluster) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sc.Status.Conditions, scyllav2alpha1.AvailableCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav2alpha1.ProgressingCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav2alpha1.DegradedCondition, sc.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	return true, nil
}

func WaitForScyllaClusterState(ctx context.Context, client scyllav2alpha1client.ScyllaV2alpha1Interface, namespace string, name string, options utils.WaitForStateOptions, condition func(sc *scyllav2alpha1.ScyllaCluster) (bool, error), additionalConditions ...func(sc *scyllav2alpha1.ScyllaCluster) (bool, error)) (*scyllav2alpha1.ScyllaCluster, error) {
	return utils.WaitForObjectState[*scyllav2alpha1.ScyllaCluster, *scyllav2alpha1.ScyllaClusterList](ctx, client.ScyllaClusters(namespace), name, options, condition, additionalConditions...)
}
