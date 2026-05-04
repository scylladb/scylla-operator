/*
 * Copyright (C) 2026 ScyllaDB
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gocql

import (
	"sync"
	"sync/atomic"

	frm "github.com/gocql/gocql/internal/frame"
)

// framerPool owns one sync.Pool plus the adaptive buffer-sizing state for one
// framer usage class.
type framerPool struct {
	pool       sync.Pool
	bufAvgSize atomic.Int64
	enabled    atomic.Bool
}

// connFramers owns connection-scoped framer configuration and reader/writer pools.
type connFramers struct {
	readPool  framerPool
	writePool framerPool
	defaults  framerConfig
}

// framerConfig holds precomputed default framer parameters for the connection.
// Populated once during connection setup and used to initialize framers from the pool.
type framerConfig struct {
	compressor            Compressor
	flagLWT               int
	rateLimitingErrorCode int
	proto                 byte
	flags                 byte
	tabletsRoutingV1      bool
}

// framerBufEWMAWeight controls how quickly the exponential weighted moving average
// of framer buffer sizes adapts. A value of 8 means each sample contributes ~12.5%,
// so it takes roughly 8 samples to converge to a new steady state.
//
// Lower values (e.g., 4) adapt faster but are more sensitive to outliers.
// Higher values (e.g., 16) are more stable but adapt slower to workload changes.
// The value 8 was chosen as a reasonable balance for typical CQL query patterns.
const framerBufEWMAWeight = 8

// framerBufShrinkThreshold is the multiplier applied to the EWMA to decide when a
// framer's read buffer is too large relative to typical usage and should be shrunk.
const framerBufShrinkThreshold = 2

// maxReasonableBufferSize is a safety limit to prevent overflow in EWMA calculations
// and to catch pathological cases where buffers grow unreasonably large.
const maxReasonableBufferSize = 512 * 1024 * 1024 // 512MB

// maxCASRetries is the maximum number of CAS retries for updating EWMA.
// This prevents infinite loops under extreme contention.
const maxCASRetries = 100

// initFramerCache precomputes framer fields from cqlProtoExts so that
// per-query framer creation avoids repeated linear scans and allocations.
func (c *Conn) initFramerCache() {
	c.framers.initCache(c)
}

func (cf *connFramers) initCache(c *Conn) {
	cfg := framerConfig{
		compressor: c.compressor,
		proto:      c.version & protoVersionMask,
	}
	if c.compressor != nil {
		cfg.flags |= frm.FlagCompress
	}
	if c.version == protoVersion5 {
		cfg.flags |= frm.FlagBetaProtocol
	}
	if lwtExt := findCQLProtoExtByName(c.cqlProtoExts, lwtAddMetadataMarkKey); lwtExt != nil {
		if castedExt, ok := lwtExt.(*lwtAddMetadataMarkExt); ok {
			cfg.flagLWT = castedExt.lwtOptMetaBitMask
		} else {
			c.logger.Printf("gocql: failed to cast CQL protocol extension %s to %T", lwtAddMetadataMarkKey, lwtAddMetadataMarkExt{})
		}
	}
	if rateLimitErrorExt := findCQLProtoExtByName(c.cqlProtoExts, rateLimitError); rateLimitErrorExt != nil {
		if castedExt, ok := rateLimitErrorExt.(*rateLimitExt); ok {
			cfg.rateLimitingErrorCode = castedExt.rateLimitErrorCode
		} else {
			c.logger.Printf("gocql: failed to cast CQL protocol extension %s to %T", rateLimitError, rateLimitExt{})
		}
	}
	if tabletsExt := findCQLProtoExtByName(c.cqlProtoExts, tabletsRoutingV1); tabletsExt != nil {
		if _, ok := tabletsExt.(*tabletsRoutingV1Ext); ok {
			cfg.tabletsRoutingV1 = true
		} else {
			c.logger.Printf("gocql: failed to cast CQL protocol extension %s to %T", tabletsRoutingV1, tabletsRoutingV1Ext{})
		}
	}
	cf.defaults = cfg
	c.setTabletSupported(cfg.tabletsRoutingV1)
	cf.initPool(c)
}

func (cf *connFramers) initPool(c *Conn) {
	defaults := cf.defaults
	cf.readPool.init(defaults, func(f *framer) { c.releaseReadFramer(f) })
	cf.writePool.init(defaults, func(f *framer) { c.releaseWriteFramer(f) })
}

// getReadFramer returns a pooled framer for reading responses and events.
func (c *Conn) getReadFramer() *framer {
	return c.framers.getRead(c)
}

func (cf *connFramers) getRead(c *Conn) *framer {
	f := cf.readPool.get(c)
	f.released.Store(false)
	return f
}

// getWriteFramer returns a pooled framer for building outgoing requests.
func (c *Conn) getWriteFramer() *framer {
	return c.framers.getWrite(c)
}

func (cf *connFramers) getWrite(c *Conn) *framer {
	f := cf.writePool.get(c)
	f.released.Store(false)
	f.flags = cf.defaults.flags
	return f
}

// releaseReadFramer returns a response/event framer to the reader pool.
func (c *Conn) releaseReadFramer(f *framer) {
	c.framers.releaseRead(c, f)
}

func (cf *connFramers) releaseRead(c *Conn, f *framer) {
	if f == nil {
		return
	}
	if f.released.Swap(true) {
		return // already released
	}
	f.header = nil
	f.traceID = nil
	f.customPayload = nil
	if !cf.readPool.enabled.Load() {
		return
	}

	bufCap := int64(cap(f.readBuffer))
	newAvg, success := cf.readPool.updateAvg(c.logger, bufCap)
	if !success {
		cf.readPool.resetAndPut(f, false, 0)
		return
	}
	cf.readPool.resetAndPut(f, true, fpShrinkSize(bufCap, newAvg))
}

// releaseWriteFramer returns a request-builder framer to the writer pool.
func (c *Conn) releaseWriteFramer(f *framer) {
	c.framers.releaseWrite(f)
}

func (cf *connFramers) releaseWrite(f *framer) {
	if f == nil {
		return
	}
	if f.released.Swap(true) {
		return
	}
	f.header = nil
	f.traceID = nil
	f.customPayload = nil
	f.flags = cf.defaults.flags
	if !cf.writePool.enabled.Load() {
		return
	}
	bufCap := int64(cap(f.buf))
	newAvg, success := cf.writePool.updateAvg(nil, bufCap)
	if !success {
		cf.writePool.resetAndPut(f, false, 0)
		return
	}
	cf.writePool.resetAndPut(f, false, fpShrinkSize(bufCap, newAvg))
}

func (cf *connFramers) close() {
	cf.readPool.close()
	cf.writePool.close()
}

func (fp *framerPool) init(defaults framerConfig, release func(*framer)) {
	fp.bufAvgSize.Store(int64(defaultBufSize))
	fp.enabled.Store(true)
	fp.pool = sync.Pool{
		New: func() any {
			buf := make([]byte, defaultBufSize)
			f := &framer{
				buf:                   buf[:0],
				readBuffer:            buf,
				compressor:            defaults.compressor,
				proto:                 defaults.proto,
				flags:                 defaults.flags,
				flagLWT:               defaults.flagLWT,
				rateLimitingErrorCode: defaults.rateLimitingErrorCode,
				tabletsRoutingV1:      defaults.tabletsRoutingV1,
			}
			f.release = func() { release(f) }
			return f
		},
	}
}

func (fp *framerPool) get(c *Conn) *framer {
	if !fp.enabled.Load() {
		return newFramer(c.compressor, c.version)
	}
	return fp.pool.Get().(*framer)
}

func (fp *framerPool) put(f *framer) {
	if !fp.enabled.Load() {
		return
	}
	fp.pool.Put(f)
}

func (fp *framerPool) close() {
	fp.enabled.Store(false)
}

func (fp *framerPool) updateAvg(logger StdLogger, bufCap int64) (int64, bool) {
	if bufCap > maxReasonableBufferSize {
		bufCap = maxReasonableBufferSize
	}
	if bufCap < 0 {
		bufCap = defaultBufSize
	}

	for i := 0; i < maxCASRetries; i++ {
		avg := fp.bufAvgSize.Load()
		// EWMA update with upward-biased rounding: the +framerBufEWMAWeight/2 term
		// biases the integer division toward ceiling for all deltas. This means:
		// - When bufCap > avg (growth): the average increases slightly faster
		// - When bufCap < avg (shrink): the average decreases slightly slower
		// Both effects are intentional — favoring larger buffers reduces
		// reallocation churn at the cost of slightly more memory.
		// In practice, this means the steady-state EWMA settles ~framerBufEWMAWeight/2
		// bytes above the true average when tracking decreasing buffer sizes.
		newAvg := avg + (bufCap-avg+framerBufEWMAWeight/2)/framerBufEWMAWeight
		if fp.bufAvgSize.CompareAndSwap(avg, newAvg) {
			return newAvg, true
		}
	}

	if logger != nil {
		logger.Printf("gocql: EWMA update failed after %d retries, skipping shrink decision", maxCASRetries)
	}
	return fp.bufAvgSize.Load(), false
}

func fpShrinkSize(bufCap, newAvg int64) int64 {
	// If this framer's buffer is much larger than the running average,
	// reallocate it to prevent a single large query from permanently
	// bloating all pooled framers.
	if bufCap <= newAvg*framerBufShrinkThreshold {
		return 0
	}
	if newAvg < defaultBufSize {
		return defaultBufSize
	}
	return newAvg
}

func (fp *framerPool) resetAndPut(f *framer, alignBufWithReadBuffer bool, shrinkSize int64) {
	if shrinkSize > 0 {
		buf := make([]byte, shrinkSize)
		f.readBuffer = buf
		f.buf = buf[:0]
		fp.put(f)
		return
	}
	if alignBufWithReadBuffer {
		f.buf = f.readBuffer[:0]
	} else {
		f.buf = f.buf[:0]
	}
	fp.put(f)
}
