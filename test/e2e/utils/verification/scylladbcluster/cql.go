package scylladbcluster

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WaitForFullQuorum(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) error {
	allHostIDs, err := utilsv1alpha1.GetHostIDsForScyllaDBCluster(ctx, rkcClusterMap, sc)
	if err != nil {
		return fmt.Errorf("can't get host IDs for ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	sortedAllHostIDs := slices.Concat(slices.Collect(maps.Values(allHostIDs))...)
	slices.Sort(sortedAllHostIDs)

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		remoteNamespaceName, err := naming.RemoteNamespaceName(sc, &dc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get remote namespace name for DC %q of ScyllaDBCluster %q: %w", dc.Name, naming.ObjRef(sc), err))
			continue
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		err = utilsv1alpha1.WaitForFullQuorum(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc, sortedAllHostIDs)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return fmt.Errorf("can't wait for scylla nodes to reach status consistency: %w", err)
	}

	framework.Infof("ScyllaDB nodes have reached status consistency.")

	return nil
}
