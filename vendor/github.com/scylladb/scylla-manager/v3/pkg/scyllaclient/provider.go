// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-manager/v3/pkg/util/logutil"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// ProviderFunc is a function that returns a Client for a given cluster.
type ProviderFunc func(ctx context.Context, clusterID uuid.UUID) (*Client, error)

type clientTTL struct {
	client   *Client
	ttl      time.Time // time after which client is invalid
	hostsTTL time.Time // time after which client hosts needs to be validated
}

const hostsValidity = 15 * time.Second

// isValid checks if client can be safely returned from cache.
// Client is invalid when it reaches the end of TTL, or when its hosts changed.
// In order to reduce API calls under mutex when creating many clients
// (e.g. when healthcheck svc runs pingREST for every node),
// checking for changed hosts is done only every hostsValidity.
func (c *clientTTL) isValid(ctx context.Context) (bool, error) {
	// Check client TTL
	if c.ttl.Before(timeutc.Now()) {
		return false, nil
	}
	// Check hosts TTL and refresh if they didn't change
	if c.hostsTTL.Before(timeutc.Now()) {
		changed, err := c.client.CheckHostsChanged(ctx)
		switch {
		case err != nil:
			return false, errors.Wrap(err, "check if client's hosts changed")
		case changed:
			return false, nil
		default:
			c.hostsTTL = timeutc.Now().Add(hostsValidity)
		}
	}
	return true, nil
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
	defer p.mu.Unlock()

	// Look for client in cache
	if c, ok := p.clients[clusterID]; ok {
		if valid, err := c.isValid(ctx); err != nil {
			p.logger.Error(ctx, "Cannot check client validity", "error", err)
		} else if valid {
			return c.client, nil
		}
	}

	// If not found or invalid, create a new one
	client, err := p.inner(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	c := clientTTL{
		client:   client,
		ttl:      timeutc.Now().Add(p.validity),
		hostsTTL: timeutc.Now().Add(hostsValidity),
	}

	p.clients[clusterID] = c
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
