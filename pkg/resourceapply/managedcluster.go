package resourceapply

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"k8s.io/klog/v2"
)

func ApplyCluster(
	ctx context.Context,
	client *mermaidclient.Client,
	requiredCluster *mermaidclient.Cluster,
	managedClusters []*mermaidclient.Cluster,
) (*mermaidclient.Cluster, bool, error) {
	for _, mc := range managedClusters {
		if requiredCluster.Name == mc.Name {
			// The Cluster is already registered in manager.
			// It must have the same ID to be recognized during update.
			requiredCluster.ID = mc.ID
			if diffClusters(mc, requiredCluster) {
				err := client.UpdateCluster(ctx, requiredCluster)
				if err != nil {
					return requiredCluster, true, fmt.Errorf("updating cluster %s: %v", requiredCluster.Name, err)
				}
				klog.V(2).InfoS("updated cluster", "cluster", requiredCluster.Name)
				return requiredCluster, true, nil
			}
			return requiredCluster, false, nil
		}
	}

	id, err := client.CreateCluster(ctx, requiredCluster)
	requiredCluster.ID = id
	if err != nil {
		return requiredCluster, true, fmt.Errorf("creating cluster %s: %v", requiredCluster.Name, err)
	}

	klog.V(2).InfoS("created cluster", "cluster", requiredCluster.Name)
	return requiredCluster, true, nil
}

// diffClusters returns true when different.
func diffClusters(c1, c2 *mermaidclient.Cluster) bool {
	// TODO: check what fields are missing in manager response and create issue about it.
	// Ignore fields in comparison:
	// ID - unknown at this point (clusters that have not yet been registered have no ID)
	// Host - not set in responses from ScyllaManager
	// Password - not set in responses from ScyllaManager
	ignore := cmpopts.IgnoreFields(mermaidclient.Cluster{}, "ID", "Host", "Password")
	return cmp.Diff(c1, c2, ignore) != ""
}
