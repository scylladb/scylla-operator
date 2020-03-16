// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"sync"

	"github.com/scylladb/mermaid/uuid"
)

// ProviderFunc is a function that returns a Client for a given cluster.
type ProviderFunc func(ctx context.Context, clusterID uuid.UUID) (*Client, error)

// CachedProvider is a provider implementation that reuses clients.
type CachedProvider struct {
	inner   ProviderFunc
	clients map[uuid.UUID]*Client
	mu      sync.Mutex
}

func NewCachedProvider(f ProviderFunc) *CachedProvider {
	return &CachedProvider{
		inner:   f,
		clients: make(map[uuid.UUID]*Client),
	}
}

// Client is the cached ProviderFunc.
func (p *CachedProvider) Client(ctx context.Context, clusterID uuid.UUID) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Cache hit
	c, ok := p.clients[clusterID]
	if ok {
		return c, nil
	}

	// Create new
	c, err := p.inner(ctx, clusterID)
	if err != nil {
		return c, err
	}

	p.clients[clusterID] = c

	return c, nil
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
		c.Close()
	}

	return nil
}
