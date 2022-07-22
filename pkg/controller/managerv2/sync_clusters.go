package managerv2

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (smc *Controller) syncClusters(
	ctx context.Context,
	client *managerclient.Client,
	managedClusters []*managerclient.Cluster,
	scyllaClusters []*scyllav1.ScyllaCluster,
	status *v1alpha1.ScyllaManagerStatus,
	managerReady bool,
) (*v1alpha1.ScyllaManagerStatus, []*managerclient.Cluster, error) {
	if !managerReady {
		return smc.calculateClustersStatuses(status, scyllaClusters, nil), nil, nil
	}

	var errs []error

	requiredClusters, err := MakeClusters(scyllaClusters, smc.secretLister)
	errs = append(errs, err)

	err = smc.pruneClusters(ctx, client, managedClusters, requiredClusters)
	errs = append(errs, err)

	var syncedClusters []*managerclient.Cluster
	for _, requiredCluster := range requiredClusters {
		syncedCluster, _, err := resourceapply.ApplyCluster(ctx, client, requiredCluster, managedClusters)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		syncedClusters = append(syncedClusters, syncedCluster)
	}

	return smc.calculateClustersStatuses(status, scyllaClusters, syncedClusters), syncedClusters, utilerrors.NewAggregate(errs)
}

func (smc *Controller) pruneClusters(
	ctx context.Context,
	client *managerclient.Client,
	managedClusters, requiredClusters []*managerclient.Cluster,
) error {
	var errs []error

	for _, mc := range managedClusters {
		remove := true
		for _, rq := range requiredClusters {
			if mc.Name == rq.Name {
				remove = false
			}
		}

		if remove {
			err := client.DeleteCluster(ctx, mc.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("deleting cluster %s: %v", mc.Name, err))
			} else {
				klog.V(2).InfoS("deleted cluster", "cluster", mc.Name)
			}
		}
	}

	return utilerrors.NewAggregate(errs)
}
