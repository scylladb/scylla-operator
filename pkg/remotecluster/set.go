// Copyright (c) 2026 ScyllaDB.

// Package remotecluster runs one controller-runtime cluster (cache, client, API reader) per remote Kubernetes cluster
// the operator manages, and starts, replaces and stops them as the RemoteKubernetesClusters and their kubeconfigs
// change. Controllers attach watches to every present and future cluster through OnCluster, and read and write a
// cluster through Cluster.
package remotecluster

import (
	"context"
	"fmt"
	"sync"

	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

// OnClusterFunc is called for every cluster of the set, present and future, once its cache is created and started,
// e.g. to attach the watches of a controller to it. The cluster is stopped when the context is done.
type OnClusterFunc func(ctx context.Context, name string, c cluster.Cluster) error

type entry struct {
	cluster    cluster.Cluster
	cancel     context.CancelFunc
	configHash string
}

// Set is the set of remote clusters. It implements remoteclient.DynamicClusterInterface, so the
// RemoteKubernetesCluster controller drives it the way it drives the remote typed clients.
type Set struct {
	// ctx bounds the lifetime of every cluster of the set.
	ctx          context.Context
	cacheOptions cache.Options

	mu        sync.Mutex
	clusters  map[string]*entry
	onCluster []OnClusterFunc
}

var _ remoteclient.DynamicClusterInterface = &Set{}

// New creates an empty set. Every cluster's cache is built with cacheOptions, e.g. to watch only selected objects;
// the scheme is the operator's.
func New(ctx context.Context, cacheOptions cache.Options) *Set {
	return &Set{
		ctx:          ctx,
		cacheOptions: cacheOptions,
		clusters:     map[string]*entry{},
	}
}

// OnCluster registers fn to run for every cluster of the set, now and whenever one is added or replaced.
func (s *Set) OnCluster(fn OnClusterFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onCluster = append(s.onCluster, fn)

	for name, e := range s.clusters {
		err := fn(s.ctx, name, e.cluster)
		if err != nil {
			return fmt.Errorf("can't run on cluster %q: %w", name, err)
		}
	}

	return nil
}

// Cluster returns the cluster of name, or an error if the set doesn't know it (yet).
func (s *Set) Cluster(name string) (cluster.Cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.clusters[name]
	if !ok {
		return nil, fmt.Errorf("cluster %q is not registered", name)
	}

	return e.cluster, nil
}

// UpdateCluster creates the cluster of name from the kubeconfig, replacing a previous one built from a different
// kubeconfig. The cluster is started in the background; the OnCluster functions run right away, and the watches they
// attach sync once the cache does.
func (s *Set) UpdateCluster(name string, config []byte) error {
	configHash, err := hash.HashBytes(config)
	if err != nil {
		return fmt.Errorf("can't hash config bytes: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if e, found := s.clusters[name]; found {
		if e.configHash == configHash {
			return nil
		}

		klog.V(2).InfoS("Remote cluster kubeconfig changed, replacing the cluster", "Cluster", name)
		e.cancel()
		delete(s.clusters, name)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
	if err != nil {
		return fmt.Errorf("can't create REST config from kubeconfig: %w", err)
	}

	c, err := newCluster(restConfig, s.cacheOptions)
	if err != nil {
		return fmt.Errorf("can't create cluster %q: %w", name, err)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	go func() {
		err := c.Start(ctx)
		if err != nil {
			klog.ErrorS(err, "Remote cluster stopped with an error", "Cluster", name)
		}
	}()

	s.clusters[name] = &entry{
		cluster:    c,
		cancel:     cancel,
		configHash: configHash,
	}

	var errs []error
	for _, fn := range s.onCluster {
		err := fn(ctx, name, c)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't run on cluster %q: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}

	klog.V(2).InfoS("Registered remote cluster", "Cluster", name)
	return nil
}

// DeleteCluster stops and forgets the cluster of name.
func (s *Set) DeleteCluster(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, found := s.clusters[name]
	if !found {
		return
	}

	e.cancel()
	delete(s.clusters, name)
	klog.V(2).InfoS("Unregistered remote cluster", "Cluster", name)
}

// HasSynced tells whether the caches of all registered clusters have synced.
func (s *Set) HasSynced() bool {
	s.mu.Lock()
	clusters := make([]cluster.Cluster, 0, len(s.clusters))
	for _, e := range s.clusters {
		clusters = append(clusters, e.cluster)
	}
	s.mu.Unlock()

	for _, c := range clusters {
		ctx, cancel := context.WithCancel(s.ctx)
		cancel()
		// A cancelled context makes WaitForCacheSync report the current state without waiting.
		if !c.GetCache().WaitForCacheSync(ctx) {
			return false
		}
	}

	return true
}

func newCluster(restConfig *rest.Config, cacheOptions cache.Options) (cluster.Cluster, error) {
	return cluster.New(restConfig, func(o *cluster.Options) {
		o.Scheme = scheme.Scheme
		o.Cache = cacheOptions
		o.Client = client.Options{
			Cache: &client.CacheOptions{
				EnableReadYourWritesConsistency: ptr.To(true),
			},
		}
	})
}
