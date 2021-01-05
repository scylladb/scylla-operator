// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"time"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	RetryInterval time.Duration
	Timeout       time.Duration
	client.Client
}

func (c *Client) UpdateScyllaCluster(ctx context.Context, cluster *scyllav1.ScyllaCluster, opts ...client.UpdateOption) error {
	rv := cluster.ResourceVersion

	if err := c.Client.Update(ctx, cluster, opts...); err != nil {
		return err
	}

	return wait.Poll(c.RetryInterval, c.Timeout, func() (bool, error) {
		newCluster := &scyllav1.ScyllaCluster{}
		key := client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}
		err := c.Client.Get(ctx, key, newCluster)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, errors.Wrap(err, "get scylla cluster")
		}
		if rv != newCluster.ResourceVersion {
			return true, nil
		}
		return false, nil
	})
}

func (c *Client) Refresh(ctx context.Context, obj runtime.Object) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}
	return c.Client.Get(ctx, key, obj)
}
