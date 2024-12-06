// Copyright (c) 2024 ScyllaDB.

package client

import (
	"fmt"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/util/hash"
	"k8s.io/klog/v2"
)

type ClusterClientInterface[CT any] interface {
	// Cluster returns a client of type CT bound to the given cluster.
	Cluster(name string) (CT, error)
}

type DynamicClusterInterface interface {
	// UpdateCluster updates the cluster under the given name with the provided configuration.
	UpdateCluster(name string, config []byte) error
	// DeleteCluster deletes data associated with provided cluster name.
	DeleteCluster(name string)
}

func NewClusterClient[CT any](makeClient func(config []byte) (CT, error)) *ClusterClient[CT] {
	rc := &ClusterClient[CT]{
		makeClient:    makeClient,
		lock:          sync.RWMutex{},
		clustersCache: make(map[string]clusterInfo[CT]),
	}

	return rc
}

type clusterInfo[CT any] struct {
	configHash string
	client     CT
}

type ClusterClient[CT any] struct {
	makeClient func(config []byte) (CT, error)

	lock          sync.RWMutex
	clustersCache map[string]clusterInfo[CT]
}

var _ DynamicClusterInterface = &ClusterClient[interface{}]{}

func (c *ClusterClient[CT]) UpdateCluster(cluster string, config []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	configHash, err := hash.HashBytes(config)
	if err != nil {
		return fmt.Errorf("can't hash cluster config: %w", err)
	}

	ci, ok := c.clustersCache[cluster]
	if ok && configHash == ci.configHash {
		return nil
	}

	klog.V(4).InfoS("Updating cluster client", "cluster", cluster)

	clusterClient, err := c.makeClient(config)
	if err != nil {
		return fmt.Errorf("can't create cluster %q client: %w", cluster, err)
	}
	c.clustersCache[cluster] = clusterInfo[CT]{
		configHash: configHash,
		client:     clusterClient,
	}

	return nil
}

func (c *ClusterClient[CT]) DeleteCluster(cluster string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	klog.V(4).InfoS("Removing cluster client", "cluster", cluster)
	delete(c.clustersCache, cluster)
}

var _ ClusterClientInterface[interface{}] = &ClusterClient[interface{}]{}

func (c *ClusterClient[CT]) Cluster(cluster string) (CT, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	ci, ok := c.clustersCache[cluster]
	if !ok {
		return *new(CT), fmt.Errorf("client for %q cluster not found", cluster)
	}
	return ci.client, nil
}
