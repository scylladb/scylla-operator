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

// Package segment implements native protocol v5 ("modern framing") transport
// segments. A segment is a self-describing, CRC-protected envelope that carries
// one or more complete CQL frames (self-contained) or a slice of a single large
// CQL frame split across several segments (non-self-contained). See the CQL
// native protocol v5 spec, section 2 ("Framing"), for the wire layout.
//
// The codec lives here, apart from the driver, because two packages decode the
// same wire format for different reasons: the connection read path, which reads
// a header and a payload in two phases so it can re-arm its read deadline in
// between, and the record/replay dialers, which are handed arbitrary slices of a
// byte stream and have to buffer until a segment is whole. Sharing the codec is
// what keeps the 17-bit field extraction, the CRC24/CRC32 checks, the
// store-as-is rule for incompressible payloads and the split threshold from
// being implemented twice and drifting.
//
// Errors are prefixed "gocql:" rather than with this package's name: they reach
// driver users, for whom this package is an implementation detail.
package segment

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gocql/gocql/internal/crc"
)

const (
	// Crc24Size is the width of a segment header's CRC24 checksum.
	Crc24Size = 3
	// Crc32Size is the width of a segment payload's CRC32 checksum.
	Crc32Size = 4

	// UncompressedHeaderSize is the total size of an uncompressed segment's
	// header: a 3-byte length-and-flag field plus its CRC24.
	UncompressedHeaderSize = 3 + Crc24Size
	// CompressedHeaderSize is the total size of a compressed segment's header: a
	// 5-byte field carrying both lengths and the flag, plus its CRC24.
	CompressedHeaderSize = 5 + Crc24Size

	// MaxPayloadSize is the largest payload a single transport segment may carry
	// (2^17 - 1). Used as a bound check when building segments.
	MaxPayloadSize = 0x1FFFF

	// PayloadLenMask extracts the 17-bit payload-length field from a decoded
	// segment header. Numerically equal to MaxPayloadSize, but kept separate: one
	// is a limit, the other is a bit mask, and conflating them obscures why no
	// explicit bound check is needed after masking.
	PayloadLenMask = 0x1FFFF
)

// Compressor compresses and decompresses segment payloads.
//
// It is deliberately a structural interface rather than a reference to the
// driver's SegmentCompressor: this package cannot import the root package, and
// the lz4 compressor lives in a separate module that cannot import internal
// packages either. Both satisfy this implicitly, and must keep doing so.
//
// A nil Compressor means the uncompressed segment layout. The driver's
// Compressor.Name is not needed here — the one place that wants it for an error
// message is the narrowing in the root package.
type Compressor interface {
	// AppendCompressed compresses src and appends the compressed bytes to dst.
	AppendCompressed(dst, src []byte) ([]byte, error)

	// AppendDecompressed decompresses src (whose decompressed size is supplied
	// out-of-band as decompressedLength) and appends the result to dst.
	AppendDecompressed(dst, src []byte, decompressedLength uint32) ([]byte, error)
}

// Header is the decoded fixed-size header of a transport segment.
//
// The header and the payload are read in two phases (ReadHeader then ReadPayload)
// so a caller can re-arm a read deadline between the possibly-idle wait for the
// header and the bounded read of the payload.
type Header struct {
	// PayloadLen is the number of payload bytes on the wire that follow the
	// header (the post-compression size for compressed segments).
	PayloadLen int
	// UncompressedLen is the size of the payload after decompression. It is 0
	// for uncompressed segments, and also 0 for compressed segments whose
	// payload is stored as-is because compression was not worth it.
	UncompressedLen int
	IsSelfContained bool
}

// Scratch holds the buffers a segment payload is read into. Every inbound
// segment would otherwise allocate its wire payload, plus a second buffer for the
// decompressed bytes of a compressed segment — one or two allocations of up to
// MaxPayloadSize (~128 KiB) each, per segment. A connection's receive path runs
// entirely on its serve() goroutine, so it can reuse one instance for every
// segment it reads.
//
// The consequence is that a payload returned by ReadPayload aliases these
// buffers and is only valid until the next segment is read with the same Scratch.
// Every caller either copies the payload (reassembly) or fully consumes it (frame
// parsing copies the body into a pooled framer) before reading the next segment.
type Scratch struct {
	// wire holds the payload bytes as they arrive on the wire, which for a
	// compressed segment means the still-compressed bytes.
	wire []byte
	// decompressed holds the payload of a compressed segment after decompression.
	decompressed []byte
}

// wireBuf returns the wire buffer resized to exactly n bytes, ready to be read
// into, reallocating only when what it already holds is too small. n comes from a
// segment header, where both layouts carry the payload length in 17 bits, so it is
// inherently bounded by MaxPayloadSize.
func (s *Scratch) wireBuf(n int) []byte {
	if cap(s.wire) < n {
		s.wire = make([]byte, n)
	}
	s.wire = s.wire[:n]
	return s.wire
}

// decompress decompresses src into the reusable decompressed buffer.
func (s *Scratch) decompress(comp Compressor, src []byte, decompressedLen int) ([]byte, error) {
	if cap(s.decompressed) < decompressedLen {
		s.decompressed = make([]byte, 0, decompressedLen)
	}
	out, err := comp.AppendDecompressed(s.decompressed[:0], src, uint32(decompressedLen))
	if err != nil {
		return nil, err
	}
	// Keep whatever buffer the compressor ended up using, so that a compressor
	// which had to grow it does not have to grow it again for the next segment.
	s.decompressed = out
	return out, nil
}

// HeaderSize returns how many bytes ReadHeader consumes for the given layout. A
// caller that has to buffer a whole header before it can decode one — as the
// record/replay dialers do, being handed arbitrary slices of a byte stream — needs
// this before it has a Header to look at.
func HeaderSize(compressed bool) int {
	if compressed {
		return CompressedHeaderSize
	}
	return UncompressedHeaderSize
}

// ReadHeader reads and validates the fixed-size header of the next segment,
// consuming only the header bytes.
//
// It takes a bool rather than a Compressor because that is all a header depends on:
// which of the two layouts is on the wire. Only the payload needs the codec itself.
func ReadHeader(r io.Reader, compressed bool) (Header, error) {
	if compressed {
		return ReadCompressedHeader(r)
	}
	return ReadUncompressedHeader(r)
}

// ReadPayload reads and verifies the payload and trailing CRC32 that follow a
// header previously read by ReadHeader, returning the reconstructed (decompressed,
// if applicable) payload bytes. The result aliases scratch; see Scratch.
func ReadPayload(r io.Reader, h Header, comp Compressor, scratch *Scratch) ([]byte, error) {
	if comp != nil {
		return ReadCompressedPayload(r, h, comp, scratch)
	}
	return ReadUncompressedPayload(r, h, scratch)
}

// ReadUncompressedHeader decodes an uncompressed segment header.
func ReadUncompressedHeader(r io.Reader) (Header, error) {
	const headerSize = 3

	var header [headerSize + Crc24Size]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Header{}, fmt.Errorf("gocql: failed to read uncompressed frame, err: %w", err)
	}

	// Compute and verify the header CRC24
	computedHeaderCRC24 := crc.Crc24(header[:headerSize])
	readHeaderCRC24 := uint32(header[3]) | uint32(header[4])<<8 | uint32(header[5])<<16
	if computedHeaderCRC24 != readHeaderCRC24 {
		return Header{}, fmt.Errorf("gocql: crc24 mismatch in frame header, computed: %d, got: %d", computedHeaderCRC24, readHeaderCRC24)
	}

	headerInt := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16
	return Header{
		PayloadLen:      int(headerInt & PayloadLenMask),
		IsSelfContained: (headerInt & (1 << 17)) != 0,
	}, nil
}

// ReadUncompressedPayload reads the payload of an uncompressed segment and
// verifies its CRC32. The result aliases scratch.
func ReadUncompressedPayload(r io.Reader, h Header, scratch *Scratch) ([]byte, error) {
	payload := scratch.wireBuf(h.PayloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("gocql: failed to read uncompressed frame payload, err: %w", err)
	}

	// Read and verify the payload CRC32
	var crcBuf [Crc32Size]byte
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

// ReadCompressedHeader decodes a compressed segment header.
func ReadCompressedHeader(r io.Reader) (Header, error) {
	const headerSize = 5

	var headerBuf [headerSize + Crc24Size]byte
	if _, err := io.ReadFull(r, headerBuf[:]); err != nil {
		return Header{}, fmt.Errorf("gocql: failed to read compressed frame header, err: %w", err)
	}

	// Reading checksum from frame header
	readHeaderChecksum := uint32(headerBuf[5]) | uint32(headerBuf[6])<<8 | uint32(headerBuf[7])<<16
	if computedHeaderChecksum := crc.Crc24(headerBuf[:headerSize]); computedHeaderChecksum != readHeaderChecksum {
		return Header{}, fmt.Errorf("gocql: crc24 mismatch in frame header, read: %d, computed: %d", readHeaderChecksum, computedHeaderChecksum)
	}

	// First 17 bits - payload size after compression
	compressedLen := uint32(headerBuf[0]) | uint32(headerBuf[1])<<8 | uint32(headerBuf[2]&0x1)<<16

	// The next 17 bits - payload size before compression
	uncompressedLen := (uint32(headerBuf[2]) >> 1) | uint32(headerBuf[3])<<7 | uint32(headerBuf[4]&0b11)<<15

	// Both fields are extracted with a 17-bit mask, so each is inherently bounded
	// by MaxPayloadSize (0x1FFFF, ~128 KiB). The payload allocations in
	// ReadCompressedPayload/ReadUncompressedPayload are therefore bounded without
	// an explicit check, and the int() conversions below are safe on 32-bit
	// platforms. TestReadCompressedHeaderLengthsBoundedTo17Bits locks this invariant.

	return Header{
		PayloadLen:      int(compressedLen),
		UncompressedLen: int(uncompressedLen),
		IsSelfContained: (headerBuf[4] & 0b100) != 0,
	}, nil
}

// ReadCompressedPayload reads the payload of a compressed segment, verifies its
// CRC32 and decompresses it. The result aliases scratch.
func ReadCompressedPayload(r io.Reader, h Header, comp Compressor, scratch *Scratch) ([]byte, error) {
	compressedPayload := scratch.wireBuf(h.PayloadLen)
	if _, err := io.ReadFull(r, compressedPayload); err != nil {
		return nil, fmt.Errorf("gocql: failed to read compressed frame payload, err: %w", err)
	}

	var crcBuf [Crc32Size]byte
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
	if h.UncompressedLen == 0 {
		return compressedPayload, nil
	}

	uncompressedPayload, err := scratch.decompress(comp, compressedPayload, h.UncompressedLen)
	if err != nil {
		return nil, err
	}
	if len(uncompressedPayload) != h.UncompressedLen {
		return nil, fmt.Errorf("gocql: length mismatch after payload decoding, got %d, expected %d", len(uncompressedPayload), h.UncompressedLen)
	}

	return uncompressedPayload, nil
}

// Append encodes payload as one transport segment appended to dst, in the layout
// matching comp. On error the returned slice must not be used: the compressed
// variant can fail after having written into dst.
func Append(dst, payload []byte, isSelfContained bool, comp Compressor) ([]byte, error) {
	if comp != nil {
		return AppendCompressed(dst, payload, isSelfContained, comp)
	}
	return AppendUncompressed(dst, payload, isSelfContained)
}

// AppendUncompressed encodes payload as one uncompressed transport segment
// (header, CRC24, payload, payload CRC32) directly into dst and returns the
// extended slice.
func AppendUncompressed(dst, payload []byte, isSelfContained bool) ([]byte, error) {
	const selfContainedBit = 1 << 17

	payloadLen := len(payload)
	if payloadLen > MaxPayloadSize {
		return nil, fmt.Errorf("gocql: payload length (%d) exceeds maximum size of %d", payloadLen, MaxPayloadSize)
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

// AppendCompressed encodes payload as one compressed transport segment directly
// into dst and returns the extended slice. The compressed payload is written into
// dst before its length is known, so the header is reserved first and filled in
// afterwards. See AppendUncompressed for the error contract.
func AppendCompressed(dst, payload []byte, isSelfContained bool, comp Compressor) ([]byte, error) {
	const (
		headerSize       = 5
		selfContainedBit = 1 << 34
	)

	uncompressedLen := len(payload)
	if uncompressedLen > MaxPayloadSize {
		return nil, fmt.Errorf("gocql: payload length (%d) exceeds maximum size of %d", uncompressedLen, MaxPayloadSize)
	}

	var reserved [headerSize + Crc24Size]byte
	headerStart := len(dst)
	dst = append(dst, reserved[:]...)
	payloadStart := len(dst)

	var err error
	dst, err = comp.AppendCompressed(dst, payload)
	if err != nil {
		return nil, err
	}
	// Compressor requires appending to dst. A custom implementation that instead
	// returns only its own output would leave the reserved header out of the
	// returned slice; report that rather than slicing out of range below. The
	// compressor is not named, because this interface deliberately does not require
	// a Name method — the root package names it when rejecting the wrong type.
	if len(dst) < payloadStart {
		return nil, fmt.Errorf("gocql: segment compressor returned %d bytes, it must append to the %d bytes it was given", len(dst), payloadStart)
	}
	compressedLen := len(dst) - payloadStart

	// Fall back to sending the payload uncompressed when compression did not
	// shrink it, or (defensively) if a Compressor returns an empty result for
	// non-empty input. The built-in LZ4Compressor never returns empty given a
	// CompressBlockBound-sized buffer, but a segment with compressedLen==0 and a
	// nonzero uncompressedLen is undecodable by the peer, so guard against it for
	// arbitrary Compressor implementations.
	//
	// This is also why compressedLen needs no bound of its own: a compressor that
	// expanded the payload past MaxPayloadSize lands here, and comes out with
	// compressedLen == uncompressedLen, which was bounded on entry.
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

// EncodedSize returns how many bytes a rawLen-byte CQL frame occupies once
// segmented, so a wire buffer can be sized before anything is encoded into it.
// For the compressed layout this is an upper bound rather than the exact size:
// compressed payloads are usually smaller, but a compressor may also return more
// bytes than it was given, so room for one segment's worth of expansion is added.
func EncodedSize(rawLen int, compressed bool) int {
	const (
		// 3-byte header + CRC24 + payload CRC32.
		uncompressedSegmentOverhead = UncompressedHeaderSize + Crc32Size
		// 5-byte header + CRC24 + payload CRC32.
		compressedSegmentOverhead = CompressedHeaderSize + Crc32Size
		// Room for a maximum-size payload growing under compression, matching
		// lz4's block bound (len + len/255 + 16). A compressor that expands more
		// than this is still handled correctly, it only makes the wire buffer grow
		// once.
		compressionSlack = MaxPayloadSize/255 + 16
	)

	segments := rawLen/MaxPayloadSize + 1
	if compressed {
		return rawLen + segments*compressedSegmentOverhead + compressionSlack
	}
	return rawLen + segments*uncompressedSegmentOverhead
}
