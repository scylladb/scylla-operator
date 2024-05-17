// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"sync"
	"time"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-manager/v3/pkg/util/logutil"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// ProviderFunc is a function that returns a Client for a given cluster.
type ProviderFunc func(ctx context.Context, clusterID uuid.UUID) (*Client, error)

type clientTTL struct {
	client *Client
	ttl    time.Time
}

// CachedProvider is a provider implementation that reuses clients.
type CachedProvider struct {
	inner    ProviderFunc
	validity time.Duration
	clients  map[uuid.UUID]clientTTL
	mu       sync.Mutex
	logger   log.Logger
}

func NewCachedProvider(f ProviderFunc, cacheInvalidationTimeout time.Duration, logger log.Logger) *CachedProvider {
	return &CachedProvider{
		inner:    f,
		validity: cacheInvalidationTimeout,
		clients:  make(map[uuid.UUID]clientTTL),
		logger:   logger.Named("cache-provider"),
	}
}

// Client is the cached ProviderFunc.
func (p *CachedProvider) Client(ctx context.Context, clusterID uuid.UUID) (*Client, error) {
	p.mu.Lock()
	c, ok := p.clients[clusterID]
	p.mu.Unlock()

	// Cache hit
	if ok {
		// Check if hosts did not change before returning
		changed, err := c.client.CheckHostsChanged(ctx)
		if err != nil {
			p.logger.Error(ctx, "Cannot check if hosts changed", "error", err)
		}
		if c.ttl.After(timeutc.Now()) && !changed && err == nil {
			return c.client, nil
		}
	}

	// If not found or hosts changed create a new one
	client, err := p.inner(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	c = clientTTL{
		client: client,
		ttl:    timeutc.Now().Add(p.validity),
	}

	p.mu.Lock()
	p.clients[clusterID] = c
	p.mu.Unlock()

	return c.client, nil
}

// Invalidate removes client for clusterID from cache.
func (p *CachedProvider) Invalidate(clusterID uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.clients, clusterID)
}

// Close removes all clients and closes them to clear up any resources.
func (p *CachedProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for clusterID, c := range p.clients {
		delete(p.clients, clusterID)
		logutil.LogOnError(context.Background(), p.logger, c.client.Close, "Couldn't close scylla client")
	}

	return nil
}
