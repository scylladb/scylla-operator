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

// Native protocol v5 ("modern framing") transport segments. The wire codec lives
// in internal/segment, which the record/replay dialers share; what remains here is
// the one thing that cannot move, because it needs this package's Compressor
// interface.

package gocql

import (
	"fmt"

	"github.com/gocql/gocql/internal/segment"
)

// asSegmentCompressor asserts that compressor supports native protocol v5 segment
// (de)compression, narrowing it to the interface the segment codec takes.
//
// The ClusterConfig validation already rejects a non-segment compressor on v5, so
// this is a defensive check whose error should never surface in practice. It stays
// in this package because it is the only part of the segment path that needs
// Compressor.Name, and naming the offending compressor is the whole value of the
// message; internal/segment deliberately requires only the two Append methods, so
// that the lz4 compressor — a separate module, which cannot import internal
// packages — keeps satisfying it structurally.
//
// A nil compressor is not a failure: it is the uncompressed layout, and yields a nil
// segment.Compressor, which is exactly how internal/segment spells that.
func asSegmentCompressor(compressor Compressor) (segment.Compressor, error) {
	if compressor == nil {
		return nil, nil
	}
	segComp, ok := compressor.(SegmentCompressor)
	if !ok {
		return nil, fmt.Errorf("gocql: compressor %q does not support protocol v5 segment compression", compressor.Name())
	}
	return segComp, nil
}

// resolveSegmentCompressor narrows the negotiated compressor once, into
// c.segCompressor, so the receive path does not repeat the assertion for every
// segment it reads.
//
// It must run after the COMPRESSION negotiation in startupCoordinator.startup — the
// last write to c.compressor, which clears it if the server refused the requested
// algorithm — and before the first segment is exchanged. Those two points are closer
// together than they look: an AUTHENTICATE response sets startupCompleted, so the
// AUTH_RESPONSE exchange that follows is already written and read as v5 segments, well
// before Conn.initFramerCache runs at the end of startupCoordinator.options. Resolving
// alongside the other snapshots taken there would leave an authenticated, compressed
// connection decoding its auth exchange with the uncompressed layout.
//
// Only v5+ resolves. Below it the segment codec is never reached, and a compressor
// without segment support — SnappyCompressor — is a valid configuration that must not
// be refused here.
//
// The error cannot fire on a connection that got this far: ClusterConfig.Validate
// rejects a compressor without segment support on v5 before anything is dialed. It is
// reported rather than ignored because the alternative is a connection that silently
// reads the wrong segment layout, and failing the handshake is where a misconfigured
// connection should die — not mid-stream, on a path whose callers can only turn it
// into a per-request error.
func (c *Conn) resolveSegmentCompressor() error {
	if c.version <= protoVersion4 {
		return nil
	}

	segComp, err := asSegmentCompressor(c.compressor)
	if err != nil {
		return err
	}
	c.segCompressor = segComp

	return nil
}
