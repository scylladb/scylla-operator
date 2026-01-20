//go:build integration
// +build integration

package gocql

// This file contains code to enable easy access to driver internals
// To be used only for integration test

import "fmt"

func (p *policyConnPool) MissingConnections() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := 0

	// close the pools
	for _, pool := range p.hostConnPools {
		missing := pool.GetShardCount() - pool.GetConnectionCount()
		if pool.IsClosed() {
			return 0, fmt.Errorf("pool for %s is closed", pool.host.HostID())
		}
		total += missing
	}
	return total, nil
}

func (s *Session) MissingConnections() (int, error) {
	if s.pool == nil {
		return 0, fmt.Errorf("pool is nil")
	}
	return s.pool.MissingConnections()
}

type ConnPickerIntegration interface {
	Pick(Token, ExecutableQuery) *Conn
	Put(*Conn) error
	Remove(conn *Conn)
	InFlight() int
	Size() (int, int)
	Close()
	CloseAllConnections()

	// NextShard returns the shardID to connect to.
	// nrShard specifies how many shards the host has.
	// If nrShards is zero, the caller shouldn't use shard-aware port.
	NextShard() (shardID, nrShards int)
}

func (p *scyllaConnPicker) CloseAllConnections() {
	p.nrConns = 0
	closeConns(p.conns...)
	for id := range p.conns {
		p.conns[id] = nil
	}
}

func (p *defaultConnPicker) CloseAllConnections() {
	closeConns(p.conns...)
	p.conns = p.conns[:0]
}

func (p *nopConnPicker) CloseAllConnections() {
}

func (pool *hostConnPool) CloseAllConnections() {
	if !pool.closed {
		return
	}
	pool.mu.Lock()
	println("Closing all connections in a pool")
	pool.connPicker.(ConnPickerIntegration).CloseAllConnections()
	println("Filling the pool")
	pool.mu.Unlock()
	pool.fill()
}

func (p *policyConnPool) CloseAllConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// close the pools
	for _, pool := range p.hostConnPools {
		pool.CloseAllConnections()
	}
}

func (s *Session) CloseAllConnections() {
	if s.pool != nil {
		s.pool.CloseAllConnections()
	}
}
