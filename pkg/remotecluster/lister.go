// Copyright (c) 2026 ScyllaDB.

package remotecluster

import (
	"context"

	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClusterLister returns a lister that resolves a cluster of the set by name and reads through that cluster's client
// under ctx: the shape the ScyllaDBCluster controller's functions take for remote objects. A cluster the set doesn't
// know yields a lister whose every read fails with that error, so the caller reports it instead of treating the
// cluster as empty.
func NewClusterLister[L any](ctx context.Context, set *Set, newLister func(context.Context, client.Reader) L) remotelister.GenericClusterLister[L] {
	return remotelister.NewGenericClusterLister(func(name string) L {
		c, err := set.Cluster(name)
		if err != nil {
			return newLister(ctx, unknownClusterReader{err: err})
		}

		return newLister(ctx, c.GetClient())
	})
}

// unknownClusterReader fails every read with the error of the cluster lookup.
type unknownClusterReader struct {
	err error
}

var _ client.Reader = unknownClusterReader{}

func (r unknownClusterReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return r.err
}

func (r unknownClusterReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return r.err
}
