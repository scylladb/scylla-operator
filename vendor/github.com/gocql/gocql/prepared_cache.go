/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
/*
 * Content before git sha 34fdeebefcbf183ed7f916f931aa0586fdaa1b40
 * Copyright (c) 2016, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"bytes"
	"sync"

	"github.com/gocql/gocql/internal/lru"
)

const defaultMaxPreparedStmts = 1000

// stmtCacheKey is a composite key for the prepared statement cache.
// A struct avoids the allocation and collision risk of concatenating
// (hostID, keyspace, statement) into a single string key.
type stmtCacheKey struct {
	keyspace  string
	statement string
	hostID    UUID
}

// preparedLRU is the prepared statement cache
type preparedLRU struct {
	lru *lru.Cache[stmtCacheKey]
	mu  sync.Mutex
}

func (p *preparedLRU) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.lru.Len() > 0 {
		p.lru.RemoveOldest()
	}
}

func (p *preparedLRU) add(key stmtCacheKey, val *inflightPrepare) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lru.Add(key, val)
}

func (p *preparedLRU) remove(key stmtCacheKey) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lru.Remove(key)
}

// updateMetadataIfSame atomically replaces the cache entry for key with val, but
// only when the currently cached entry is still the exact prepared statement
// identified by expect (pointer identity of its preparedStatment, and its done
// channel already closed). It returns true if the replacement happened.
//
// Pointer identity — not the prepared id — is the generation token: a concurrent
// eviction+reprepare for the same statement installs a new *inflightPrepare with
// a freshly allocated *preparedStatment, so even though the prepared id bytes are
// typically identical across reprepares, the pointer differs and this stale
// refresh is correctly skipped.
//
// This makes the METADATA_CHANGED metadata refresh a single locked operation:
// the presence/identity check and the replacement cannot be interleaved with a
// concurrent eviction (which would otherwise be resurrected) or with a newer or
// still-in-flight prepare installed for the same key (which would otherwise be
// clobbered).
func (p *preparedLRU) updateMetadataIfSame(key stmtCacheKey, expect *preparedStatment, val *inflightPrepare) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	cur, ok := p.lru.Get(key)
	if !ok {
		return false
	}
	ifp, ok := cur.(*inflightPrepare)
	if !ok {
		return false
	}

	select {
	case <-ifp.done:
		if ifp.preparedStatment != nil && ifp.preparedStatment == expect {
			p.lru.Add(key, val)
			return true
		}
	default:
		// still in-flight — leave the newer prepare alone
	}
	return false
}

func (p *preparedLRU) execIfMissing(key stmtCacheKey, fn func(cache *lru.Cache[stmtCacheKey]) *inflightPrepare) (*inflightPrepare, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	val, ok := p.lru.Get(key)
	if ok {
		return val.(*inflightPrepare), true
	}

	return fn(p.lru), false
}

// keyFor constructs a zero-allocation composite cache key from the given
// components. The returned struct references the original strings without
// copying, so no heap allocation occurs.
func (p *preparedLRU) keyFor(hostID UUID, keyspace, statement string) stmtCacheKey {
	return stmtCacheKey{
		hostID:    hostID,
		keyspace:  keyspace,
		statement: statement,
	}
}

func (p *preparedLRU) evictPreparedID(key stmtCacheKey, id []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	val, ok := p.lru.Get(key)
	if !ok {
		return
	}

	ifp, ok := val.(*inflightPrepare)
	if !ok {
		return
	}

	select {
	case <-ifp.done:
		// preparedStatment is nil when the prepare failed. prepareStatement removes
		// such an entry from the cache before closing done, so it should not be
		// reachable from here — but that is an ordering nothing enforces, and the
		// check costs less than the panic. updateMetadataIfSame guards the same way.
		if ifp.preparedStatment != nil && bytes.Equal(id, ifp.preparedStatment.id) {
			p.lru.Remove(key)
		}
	default:
	}
}
