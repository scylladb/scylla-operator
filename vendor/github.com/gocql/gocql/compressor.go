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
	"fmt"

	"github.com/klauspost/compress/s2"

	frm "github.com/gocql/gocql/internal/frame"
)

// Compressor is the interface that must be implemented by frame compressors.
//
// Encode compresses data and returns the compressed bytes; Decode reverses it.
// Any protocol-required framing (e.g. LZ4's big-endian uncompressed-length
// prefix expected by Cassandra) is the compressor's responsibility.
//
// Decode must bound what it allocates: it must reject a decompressed length above
// 256 MiB. The frame reader bounds the compressed body it reads, but only Decode
// knows how far that body expands, so a compressor that trusts a length carried
// inside the compressed data lets a short corrupt frame drive an arbitrarily large
// allocation. 256 MiB is the largest frame body the driver accepts, reused here as
// a deliberate ceiling on expansion -- the protocol bounds only the compressed
// side, so this is stricter than the wire requires. Both built-in compressors
// enforce it; a third-party implementation is expected to do the same.
type Compressor interface {
	Name() string
	Encode(data []byte) ([]byte, error)
	Decode(data []byte) ([]byte, error)
}

// SegmentCompressor is an optional capability interface for compressors that
// support native protocol v5 segment compression. The v5 transport carries the
// uncompressed payload length out-of-band in the segment header, so — unlike
// Encode/Decode — no length prefix is embedded in the compressed bytes.
//
// A Compressor that does not also implement SegmentCompressor cannot be used
// with ProtoVersion >= 5; the driver rejects such a configuration up front (see
// the ProtoVersion validation in the cluster config). Both methods append to
// dst and return the extended slice.
type SegmentCompressor interface {
	// AppendCompressed compresses src and appends the compressed bytes to dst.
	AppendCompressed(dst, src []byte) ([]byte, error)

	// AppendDecompressed decompresses src (whose decompressed size is supplied
	// out-of-band as decompressedLength) and appends the result to dst.
	AppendDecompressed(dst, src []byte, decompressedLength uint32) ([]byte, error)
}

// SnappyCompressor implements the Compressor interface and can be used to
// compress incoming and outgoing frames. It uses the S2 compression algorithm,
// which is compatible with snappy and aims for high throughput.
//
// SnappyCompressor deliberately does not implement SegmentCompressor: the
// native protocol v5 spec allows only lz4 for segment compression, so
// SnappyCompressor cannot be used with ProtoVersion >= 5. Such a configuration
// is rejected up front by the cluster config validation. v5 is also not
// auto-negotiated (discoverProtocol caps at v4), so this only affects users who
// explicitly set ProtoVersion: 5.
type SnappyCompressor struct{}

func (s SnappyCompressor) Name() string {
	return "snappy"
}

func (s SnappyCompressor) Encode(data []byte) ([]byte, error) {
	return s2.EncodeSnappy(nil, data), nil
}

func (s SnappyCompressor) Decode(data []byte) ([]byte, error) {
	// s2.Decode allocates the decoded block from the length varint at the head of
	// data -- a length the peer declared, which s2 itself only refuses above 4 GiB.
	// The frame reader bounds the *compressed* body it accepts against
	// frm.MaxFrameSize, but nothing bounds what that body expands into, so a few
	// corrupt header bytes on a short frame are enough to ask for a multi-gigabyte
	// allocation.
	//
	// frm.MaxFrameSize is reused as the ceiling, but that is a driver-side trade
	// rather than a protocol fact. At v4 the header length, and the server's
	// native_transport_max_frame_size, both bound the *compressed* body, so a
	// legitimate frame may carry a body within that limit which decodes above it.
	// Refusing those gives up a case nobody has reported -- a response compressing
	// to under 256 MiB and expanding past it -- in exchange for bounding an
	// allocation whose size the peer controls. LZ4Compressor makes the same trade
	// with maxDecompressedSize in the lz4 module.
	//
	// s2.DecodedLen reads the same varint without allocating, so this costs a few
	// bytes of parsing rather than a second pass.
	//
	// The 32-bit hazard lz4's comment cites does not apply here: above MaxInt32 s2
	// already returns ErrTooLarge rather than panicking in makeslice.
	n, err := s2.DecodedLen(data)
	if err != nil {
		return nil, err
	}
	if n > frm.MaxFrameSize {
		return nil, fmt.Errorf("gocql: snappy uncompressed length %d exceeds maximum of %d", n, frm.MaxFrameSize)
	}

	return s2.Decode(nil, data)
}
