package scylladbcluster

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
)

func WaitForFullQuorum(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) error {
	return nil
}
