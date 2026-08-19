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

// Native protocol v5 ("modern framing") transport segments. A segment is a
// self-describing, CRC-protected envelope that carries one or more complete CQL
// frames (self-contained) or a slice of a single large CQL frame split across
// several segments (non-self-contained). See the CQL native protocol v5 spec,
// section 2 ("Framing"), for the wire layout.

package gocql

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gocql/gocql/internal/crc"
)

const (
	crc24Size = 3
	crc32Size = 4
)

// segmentHeader is the decoded fixed-size header of a v5 transport segment.
//
// The header and the payload are read in two phases (readSegmentHeader then
// readSegmentPayload) so the caller can re-arm the read deadline between the
// possibly-idle wait for the header and the bounded read of the payload.
type segmentHeader struct {
	// payloadLen is the number of payload bytes on the wire that follow the
	// header (the post-compression size for compressed segments).
	payloadLen int
	// uncompressedLen is the size of the payload after decompression. It is 0
	// for uncompressed segments, and also 0 for compressed segments whose
	// payload is stored as-is because compression was not worth it.
	uncompressedLen int
	isSelfContained bool
}

// segmentScratch holds the buffers a segment payload is read into. Every inbound
// segment would otherwise allocate its wire payload, plus a second buffer for the
// decompressed bytes of a compressed segment — one or two allocations of up to
// maxSegmentPayloadSize (~128 KiB) each, per segment. A connection's receive path
// runs entirely on its serve() goroutine, so it can reuse one instance for every
// segment it reads.
//
// The consequence is that a payload returned by readSegmentPayload aliases these
// buffers and is only valid until the next segment is read with the same scratch.
// Every caller either copies the payload (reassembly) or fully consumes it (frame
// parsing copies the body into a pooled framer) before reading the next segment.
type segmentScratch struct {
	// wire holds the payload bytes as they arrive on the wire, which for a
	// compressed segment means the still-compressed bytes.
	wire []byte
	// decompressed holds the payload of a compressed segment after decompression.
	decompressed []byte
}

// wireBuf returns the wire buffer resized to exactly n bytes, ready to be read
// into, reallocating only when what it already holds is too small. n comes from a
// segment header, where both layouts carry the payload length in 17 bits, so it is
// inherently bounded by maxSegmentPayloadSize.
func (s *segmentScratch) wireBuf(n int) []byte {
	if cap(s.wire) < n {
		s.wire = make([]byte, n)
	}
	s.wire = s.wire[:n]
	return s.wire
}

// decompress decompresses src into the reusable decompressed buffer.
func (s *segmentScratch) decompress(segComp SegmentCompressor, src []byte, decompressedLen int) ([]byte, error) {
	if cap(s.decompressed) < decompressedLen {
		s.decompressed = make([]byte, 0, decompressedLen)
	}
	out, err := segComp.AppendDecompressed(s.decompressed[:0], src, uint32(decompressedLen))
	if err != nil {
		return nil, err
	}
	// Keep whatever buffer the compressor ended up using, so that a compressor
	// which had to grow it does not have to grow it again for the next segment.
	s.decompressed = out
	return out, nil
}

// readSegmentHeader reads and validates the fixed-size header of the next
// segment, consuming only the header bytes. When compressor is non-nil the
// compressed-segment layout is used, otherwise the uncompressed layout.
func readSegmentHeader(r io.Reader, compressor Compressor) (segmentHeader, error) {
	if compressor != nil {
		return readCompressedSegmentHeader(r)
	}
	return readUncompressedSegmentHeader(r)
}

// readSegmentPayload reads and verifies the payload and trailing CRC32 that
// follow a header previously read by readSegmentHeader, returning the
// reconstructed (decompressed, if applicable) payload bytes. The result aliases
// scratch; see segmentScratch.
func readSegmentPayload(r io.Reader, h segmentHeader, compressor Compressor, scratch *segmentScratch) ([]byte, error) {
	if compressor != nil {
		return readCompressedSegmentPayload(r, h, compressor, scratch)
	}
	return readUncompressedSegmentPayload(r, h, scratch)
}

func readUncompressedSegmentHeader(r io.Reader) (segmentHeader, error) {
	const headerSize = 3

	var header [headerSize + crc24Size]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return segmentHeader{}, fmt.Errorf("gocql: failed to read uncompressed frame, err: %w", err)
	}

	// Compute and verify the header CRC24
	computedHeaderCRC24 := crc.Crc24(header[:headerSize])
	readHeaderCRC24 := uint32(header[3]) | uint32(header[4])<<8 | uint32(header[5])<<16
	if computedHeaderCRC24 != readHeaderCRC24 {
		return segmentHeader{}, fmt.Errorf("gocql: crc24 mismatch in frame header, computed: %d, got: %d", computedHeaderCRC24, readHeaderCRC24)
	}

	headerInt := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16
	return segmentHeader{
		payloadLen:      int(headerInt & segmentPayloadLenMask),
		isSelfContained: (headerInt & (1 << 17)) != 0,
	}, nil
}

func readUncompressedSegmentPayload(r io.Reader, h segmentHeader, scratch *segmentScratch) ([]byte, error) {
	payload := scratch.wireBuf(h.payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("gocql: failed to read uncompressed frame payload, err: %w", err)
	}

	// Read and verify the payload CRC32
	var crcBuf [crc32Size]byte
	if _, err := io.ReadFull(r, crcBuf[:]); err != nil {
		return nil, fmt.Errorf("gocql: failed to read payload crc32, err: %w", err)
	}

	computedPayloadCRC32 := crc.Crc32(payload)
	readPayloadCRC32 := binary.LittleEndian.Uint32(crcBuf[:])
	if computedPayloadCRC32 != readPayloadCRC32 {
		return nil, fmt.Errorf("gocql: payload crc32 mismatch, computed: %d, got: %d", computedPayloadCRC32, readPayloadCRC32)
	}

	return payload, nil
}

func readCompressedSegmentHeader(r io.Reader) (segmentHeader, error) {
	const headerSize = 5

	var headerBuf [headerSize + crc24Size]byte
	if _, err := io.ReadFull(r, headerBuf[:]); err != nil {
		return segmentHeader{}, fmt.Errorf("gocql: failed to read compressed frame header, err: %w", err)
	}

	// Reading checksum from frame header
	readHeaderChecksum := uint32(headerBuf[5]) | uint32(headerBuf[6])<<8 | uint32(headerBuf[7])<<16
	if computedHeaderChecksum := crc.Crc24(headerBuf[:headerSize]); computedHeaderChecksum != readHeaderChecksum {
		return segmentHeader{}, fmt.Errorf("gocql: crc24 mismatch in frame header, read: %d, computed: %d", readHeaderChecksum, computedHeaderChecksum)
	}

	// First 17 bits - payload size after compression
	compressedLen := uint32(headerBuf[0]) | uint32(headerBuf[1])<<8 | uint32(headerBuf[2]&0x1)<<16

	// The next 17 bits - payload size before compression
	uncompressedLen := (uint32(headerBuf[2]) >> 1) | uint32(headerBuf[3])<<7 | uint32(headerBuf[4]&0b11)<<15

	// Both fields are extracted with a 17-bit mask, so each is inherently bounded
	// by maxSegmentPayloadSize (0x1FFFF, ~128 KiB). The payload allocations in
	// readCompressedSegmentPayload/readUncompressedSegmentPayload are therefore
	// bounded without an explicit check, and the int() conversions below are safe
	// on 32-bit platforms. TestReadCompressedSegmentHeader_LengthsBoundedTo17Bits
	// locks this invariant.

	return segmentHeader{
		payloadLen:      int(compressedLen),
		uncompressedLen: int(uncompressedLen),
		isSelfContained: (headerBuf[4] & 0b100) != 0,
	}, nil
}

// asSegmentCompressor asserts that compressor supports native protocol v5
// segment (de)compression. The ClusterConfig validation already rejects a
// non-segment compressor on v5, so this is a defensive check whose error should
// never surface in practice.
func asSegmentCompressor(compressor Compressor) (SegmentCompressor, error) {
	segComp, ok := compressor.(SegmentCompressor)
	if !ok {
		return nil, fmt.Errorf("gocql: compressor %q does not support protocol v5 segment compression", compressor.Name())
	}
	return segComp, nil
}

func readCompressedSegmentPayload(r io.Reader, h segmentHeader, compressor Compressor, scratch *segmentScratch) ([]byte, error) {
	compressedPayload := scratch.wireBuf(h.payloadLen)
	if _, err := io.ReadFull(r, compressedPayload); err != nil {
		return nil, fmt.Errorf("gocql: failed to read compressed frame payload, err: %w", err)
	}

	var crcBuf [crc32Size]byte
	if _, err := io.ReadFull(r, crcBuf[:]); err != nil {
		return nil, fmt.Errorf("gocql: failed to read payload crc32, err: %w", err)
	}

	// Ensuring if payload checksum matches
	readPayloadChecksum := binary.LittleEndian.Uint32(crcBuf[:])
	if computedPayloadChecksum := crc.Crc32(compressedPayload); readPayloadChecksum != computedPayloadChecksum {
		return nil, fmt.Errorf("gocql: crc32 mismatch in payload, read: %d, computed: %d", readPayloadChecksum, computedPayloadChecksum)
	}

	// An uncompressed length of 0 signals that the payload is stored as-is and
	// must not be decompressed (native_protocol_v5.spec 2.2).
	if h.uncompressedLen == 0 {
		return compressedPayload, nil
	}

	segComp, err := asSegmentCompressor(compressor)
	if err != nil {
		return nil, err
	}
	uncompressedPayload, err := scratch.decompress(segComp, compressedPayload, h.uncompressedLen)
	if err != nil {
		return nil, err
	}
	if len(uncompressedPayload) != h.uncompressedLen {
		return nil, fmt.Errorf("gocql: length mismatch after payload decoding, got %d, expected %d", len(uncompressedPayload), h.uncompressedLen)
	}

	return uncompressedPayload, nil
}

// appendUncompressedSegment encodes payload as one uncompressed transport segment
// (header, CRC24, payload, payload CRC32) directly into dst and returns the
// extended slice. On error the returned slice must not be used: the compressed
// variant can fail after having written into dst.
func appendUncompressedSegment(dst, payload []byte, isSelfContained bool) ([]byte, error) {
	const selfContainedBit = 1 << 17

	payloadLen := len(payload)
	if payloadLen > maxSegmentPayloadSize {
		return nil, fmt.Errorf("gocql: payload length (%d) exceeds maximum size of %d", payloadLen, maxSegmentPayloadSize)
	}

	// First 3 bytes: payload length and self-contained flag, as a single
	// little-endian integer.
	headerInt := uint32(payloadLen)
	if isSelfContained {
		headerInt |= selfContainedBit
	}
	var header [3]byte
	header[0] = byte(headerInt)
	header[1] = byte(headerInt >> 8)
	header[2] = byte(headerInt >> 16)

	// The next 3 bytes are the CRC24 of the header.
	checksum := crc.Crc24(header[:])
	dst = append(dst, header[0], header[1], header[2],
		byte(checksum), byte(checksum>>8), byte(checksum>>16))

	dst = append(dst, payload...)
	return binary.LittleEndian.AppendUint32(dst, crc.Crc32(payload)), nil
}

// appendCompressedSegment encodes payload as one compressed transport segment
// directly into dst and returns the extended slice. The compressed payload is
// written into dst before its length is known, so the header is reserved first
// and filled in afterwards. See appendUncompressedSegment for the error contract.
func appendCompressedSegment(dst, payload []byte, isSelfContained bool, compressor Compressor) ([]byte, error) {
	const (
		headerSize       = 5
		selfContainedBit = 1 << 34
	)

	uncompressedLen := len(payload)
	if uncompressedLen > maxSegmentPayloadSize {
		return nil, fmt.Errorf("gocql: payload length (%d) exceeds maximum size of %d", uncompressedLen, maxSegmentPayloadSize)
	}

	segComp, err := asSegmentCompressor(compressor)
	if err != nil {
		return nil, err
	}

	var reserved [headerSize + crc24Size]byte
	headerStart := len(dst)
	dst = append(dst, reserved[:]...)
	payloadStart := len(dst)

	dst, err = segComp.AppendCompressed(dst, payload)
	if err != nil {
		return nil, err
	}
	// SegmentCompressor requires appending to dst. A custom implementation that
	// instead returns only its own output would leave the reserved header out of
	// the returned slice; report that rather than slicing out of range below.
	if len(dst) < payloadStart {
		return nil, fmt.Errorf("gocql: compressor %q returned %d bytes, it must append to the %d bytes it was given",
			compressor.Name(), len(dst), payloadStart)
	}
	compressedLen := len(dst) - payloadStart

	// Fall back to sending the payload uncompressed when compression did not
	// shrink it, or (defensively) if a SegmentCompressor returns an empty result
	// for non-empty input. The built-in LZ4Compressor never returns empty given a
	// CompressBlockBound-sized buffer, but a segment with compressedLen==0 and a
	// nonzero uncompressedLen is undecodable by the peer, so guard against it for
	// arbitrary SegmentCompressor implementations.
	if compressedLen == 0 || uncompressedLen < compressedLen {
		// native_protocol_v5.spec
		// 2.2
		//  An uncompressed length of 0 signals that the compressed payload
		//  should be used as-is and not decompressed.
		dst = append(dst[:payloadStart], payload...)
		compressedLen = uncompressedLen
		uncompressedLen = 0
	}

	// Combine compressed and uncompressed lengths and set the self-contained flag
	// if needed. The value occupies 35 bits at most, so the 3 bytes PutUint64
	// writes past the 5-byte header are zero and are then overwritten by the CRC24.
	combined := uint64(compressedLen) | uint64(uncompressedLen)<<17
	if isSelfContained {
		combined |= selfContainedBit
	}
	header := dst[headerStart:payloadStart]
	binary.LittleEndian.PutUint64(header, combined)
	headerChecksum := crc.Crc24(header[:headerSize])
	header[headerSize] = byte(headerChecksum)
	header[headerSize+1] = byte(headerChecksum >> 8)
	header[headerSize+2] = byte(headerChecksum >> 16)

	return binary.LittleEndian.AppendUint32(dst, crc.Crc32(dst[payloadStart:])), nil
}
