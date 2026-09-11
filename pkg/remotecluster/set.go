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
	"time"

	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

const (
	// defaultRetryInterval is how often the set retries to build a cluster it couldn't reach.
	defaultRetryInterval = 10 * time.Second
)

// OnClusterFunc is called for every cluster of the set, present and future, once its cache is created and started,
// e.g. to attach the watches of a controller to it. The cluster is stopped when the context is done.
type OnClusterFunc func(ctx context.Context, name string, c cluster.Cluster) error

// entry is one remote cluster of the set. Building a cluster talks to its API server (the cache asks it which kinds
// are namespaced), so a cluster whose credentials or network don't work is retried in the background and is not
// ready until then.
type entry struct {
	configHash string
	cancel     context.CancelFunc

	// cluster is set once built and started; nil until then.
	cluster cluster.Cluster
	// err is the error of the last attempt to build the cluster, reported by Cluster while it is not ready.
	err error
}

// Set is the set of remote clusters. It implements remoteclient.DynamicClusterInterface, so the
// RemoteKubernetesCluster controller drives it the way it drives the remote typed clients.
type Set struct {
	// ctx bounds the lifetime of every cluster of the set.
	ctx           context.Context
	cacheOptions  cache.Options
	retryInterval time.Duration

	mu        sync.Mutex
	clusters  map[string]*entry
	onCluster []OnClusterFunc
}

var _ remoteclient.DynamicClusterInterface = &Set{}

// New creates an empty set. Every cluster's cache is built with cacheOptions, e.g. to watch only selected objects;
// the scheme is the operator's.
func New(ctx context.Context, cacheOptions cache.Options) *Set {
	return &Set{
		ctx:           ctx,
		cacheOptions:  cacheOptions,
		retryInterval: defaultRetryInterval,
		clusters:      map[string]*entry{},
	}
}

// OnCluster registers fn to run for every cluster of the set, now and whenever one is added or replaced.
func (s *Set) OnCluster(fn OnClusterFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onCluster = append(s.onCluster, fn)

	for name, e := range s.clusters {
		if e.cluster == nil {
			continue
		}

		err := fn(s.ctx, name, e.cluster)
		if err != nil {
			return fmt.Errorf("can't run on cluster %q: %w", name, err)
		}
	}

	return nil
}

// Cluster returns the cluster of name, or an error if the set doesn't know it or hasn't been able to build it (yet).
func (s *Set) Cluster(name string) (cluster.Cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.clusters[name]
	if !ok {
		return nil, fmt.Errorf("cluster %q is not registered", name)
	}

	if e.cluster == nil {
		if e.err != nil {
			return nil, fmt.Errorf("cluster %q is not ready: %w", name, e.err)
		}

		return nil, fmt.Errorf("cluster %q is not ready", name)
	}

	return e.cluster, nil
}

// UpdateCluster registers the cluster of name from the kubeconfig, replacing a previous one built from a different
// kubeconfig. The cluster is built and started in the background, retrying while its API server can't be reached;
// the OnCluster functions run once it is, and the watches they attach sync once the cache does. An unreachable
// cluster is not an error of the set: it is reported by Cluster until it is reachable.
func (s *Set) UpdateCluster(name string, config []byte) error {
	configHash, err := hash.HashBytes(config)
	if err != nil {
		return fmt.Errorf("can't hash config bytes: %w", err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
	if err != nil {
		return fmt.Errorf("can't create REST config from kubeconfig: %w", err)
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

	ctx, cancel := context.WithCancel(s.ctx)
	e := &entry{
		configHash: configHash,
		cancel:     cancel,
	}
	s.clusters[name] = e

	go s.build(ctx, name, e, restConfig)

	klog.V(2).InfoS("Registered remote cluster", "Cluster", name)
	return nil
}

// build creates and starts the cluster of e, retrying until it succeeds or ctx is done, and runs the OnCluster
// functions on it.
func (s *Set) build(ctx context.Context, name string, e *entry, restConfig *rest.Config) {
	var c cluster.Cluster
	err := apimachineryutilwait.PollUntilContextCancel(ctx, s.retryInterval, true, func(ctx context.Context) (bool, error) {
		var err error
		c, err = newCluster(restConfig, s.cacheOptions)
		if err != nil {
			klog.V(2).InfoS("Can't build remote cluster, will retry", "Cluster", name, "Error", err)
			s.mu.Lock()
			e.err = err
			s.mu.Unlock()
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		// The cluster was replaced or deleted while being built.
		return
	}

	go func() {
		err := c.Start(ctx)
		if err != nil {
			klog.ErrorS(err, "Remote cluster stopped with an error", "Cluster", name)
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil {
		return
	}

	e.cluster = c
	e.err = nil

	var errs []error
	for _, fn := range s.onCluster {
		err := fn(ctx, name, c)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't run on cluster %q: %w", name, err))
		}
	}
	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		klog.ErrorS(err, "Can't run functions on remote cluster", "Cluster", name)
	}

	klog.V(2).InfoS("Remote cluster is ready", "Cluster", name)
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
