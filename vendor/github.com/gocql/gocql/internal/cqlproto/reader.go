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
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package cqlproto provides low-level utilities for reading and writing
// CQL binary protocol wire format data with zero allocations.
package cqlproto

import (
	"errors"
	"fmt"
)

// ErrEOF is a sentinel used internally to keep the hot path inlinable.
var ErrEOF = errors.New("unexpected end of data")

// Reader provides zero-allocation, zero-reflection reading of CQL wire
// format data. It processes length-prefixed values (the standard CQL binary
// protocol encoding for tuples, lists, maps, and UDTs).
//
// All Read* methods advance the internal cursor. If any read encounters
// insufficient data, the reader enters an error state and all subsequent
// reads become no-ops. Check Err() after a sequence of reads.
//
// Wire format conventions (CQL binary protocol v4+):
//   - [int]    = 4-byte big-endian signed int32 (used as length prefix)
//   - [bytes]  = [int n][n bytes] (n < 0 means null)
//   - bigint   = 8-byte big-endian signed int64
//   - int      = 4-byte big-endian signed int32
//   - uuid     = 16 bytes
//   - list/set = [int n][n elements], each element is [bytes]
//   - tuple    = concatenated [bytes] elements (count known from schema)
type Reader struct {
	err  error
	data []byte
}

// NewReader creates a Reader over the given byte slice.
func NewReader(data []byte) Reader {
	return Reader{data: data}
}

// Err returns the first error encountered during reading.
func (r *Reader) Err() error {
	return r.err
}

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int {
	return len(r.data)
}

// readN reads exactly n bytes without a length prefix.
func (r *Reader) readN(n int) []byte {
	if r.err != nil {
		return nil
	}
	if len(r.data) < n {
		r.err = ErrEOF
		return nil
	}
	p := r.data[:n]
	r.data = r.data[n:]
	return p
}

// ReadRawInt reads a raw 4-byte big-endian int32 (no length prefix).
func (r *Reader) ReadRawInt() int32 {
	p := r.readN(4)
	if p == nil {
		return 0
	}
	return int32(p[0])<<24 | int32(p[1])<<16 | int32(p[2])<<8 | int32(p[3])
}

// ReadRawBigInt reads a raw 8-byte big-endian int64 (no length prefix).
func (r *Reader) ReadRawBigInt() int64 {
	p := r.readN(8)
	if p == nil {
		return 0
	}
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 |
		int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

// ReadRawUUID reads a raw 16-byte UUID (no length prefix).
func (r *Reader) ReadRawUUID() [16]byte {
	var uuid [16]byte
	p := r.readN(16)
	if p != nil {
		copy(uuid[:], p)
	}
	return uuid
}

// ReadBytes reads a [bytes] value: 4-byte length prefix followed by payload.
// Returns nil for null values (length < 0).
func (r *Reader) ReadBytes() []byte {
	n := int(r.ReadRawInt())
	if r.err != nil {
		return nil
	}
	if n < 0 {
		return nil // CQL null
	}
	return r.readN(n)
}

// ReadInt reads a length-prefixed int32: [4-byte len][4-byte value].
// Returns 0 if the value is null (length < 0).
func (r *Reader) ReadInt() (int32, bool) {
	p := r.ReadBytes()
	if r.err != nil {
		return 0, false
	}
	if p == nil {
		return 0, false // CQL null
	}
	if len(p) != 4 {
		r.err = fmt.Errorf("unmarshal int: expected 4 bytes got %d", len(p))
		return 0, false
	}
	return int32(p[0])<<24 | int32(p[1])<<16 | int32(p[2])<<8 | int32(p[3]), true
}

// ReadBigInt reads a length-prefixed int64: [4-byte len][8-byte value].
// Returns 0 if the value is null (length < 0).
func (r *Reader) ReadBigInt() (int64, bool) {
	p := r.ReadBytes()
	if r.err != nil {
		return 0, false
	}
	if p == nil {
		return 0, false // CQL null
	}
	if len(p) != 8 {
		r.err = fmt.Errorf("unmarshal bigint: expected 8 bytes got %d", len(p))
		return 0, false
	}
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 |
		int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7]), true
}

// ReadUUID reads a length-prefixed UUID: [4-byte len][16-byte value].
// Returns zero UUID if the value is null (length < 0).
func (r *Reader) ReadUUID() ([16]byte, bool) {
	p := r.ReadBytes()
	if r.err != nil {
		return [16]byte{}, false
	}
	if p == nil {
		return [16]byte{}, false // CQL null
	}
	if len(p) != 16 {
		r.err = fmt.Errorf("unmarshal uuid: expected 16 bytes got %d", len(p))
		return [16]byte{}, false
	}
	var uuid [16]byte
	copy(uuid[:], p)
	return uuid, true
}

// ReadCollectionCount reads a list/set/map element count: 4-byte int32.
// This is the raw count at the start of a collection body (after the
// collection's own length prefix has been consumed).
func (r *Reader) ReadCollectionCount() int {
	return int(r.ReadRawInt())
}
