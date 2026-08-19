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
 * Copyright (c) 2012, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	frm "github.com/gocql/gocql/internal/frame"
)

type unsetColumn struct{}

// UnsetValue represents a value used in a query binding that will be ignored by Cassandra.
//
// By setting a field to the unset value Cassandra will ignore the write completely.
// The main advantage is the ability to keep the same prepared statement even when you don't
// want to update some fields, where before you needed to make another prepared statement.
//
// UnsetValue is only available when using the version 4 of the protocol.
var UnsetValue = unsetColumn{}

type namedValue struct {
	value any
	name  string
}

// NamedValue produce a value which will bind to the named parameter in a query
func NamedValue(name string, value any) any {
	return &namedValue{
		name:  name,
		value: value,
	}
}

const (
	protoDirectionMask = 0x80
	protoVersionMask   = 0x7F
	protoVersion1      = 0x01
	protoVersion2      = 0x02
	protoVersion3      = 0x03
	protoVersion4      = 0x04
	// protoVersion5 is the final (non-beta) native protocol v5: the envelope and
	// transport-segment layout implemented by segment.go.
	//
	// Requests are deliberately never marked with frm.FlagBetaProtocol. That flag
	// opts into whatever v5 dialect the server is currently developing, which for
	// Cassandra 3.11 is the *beta* v5 dialect — it does not use the final envelope
	// layout, so setting the flag makes such a server accept the handshake and
	// then fail every subsequent frame with a protocol error, instead of cleanly
	// rejecting the version. Apache removed the same automatic opt-in in
	// CASSGO-88 (https://issues.apache.org/jira/browse/CASSGO-88). Supporting a
	// beta dialect would need an explicit opt-in bound to that specific dialect.
	protoVersion5 = 0x05

	maxFrameSize = 256 * 1024 * 1024

	// maxSegmentPayloadSize is the largest payload a single v5 transport segment
	// may carry (2^17 - 1). Used as a bound check when building segments.
	maxSegmentPayloadSize = 0x1FFFF

	// segmentPayloadLenMask extracts the 17-bit payload-length field from a
	// decoded segment header. Numerically equal to maxSegmentPayloadSize, but
	// kept separate: one is a limit, the other is a bit mask, and conflating
	// them obscures why no explicit bound check is needed after masking.
	segmentPayloadLenMask = 0x1FFFF
)

// DEPRECATED use Consistency type, SerialConsistency is now an alias for backwards compatibility.
type SerialConsistency = Consistency

type Consistency uint16

const (
	Any         Consistency = 0x00
	One         Consistency = 0x01
	Two         Consistency = 0x02
	Three       Consistency = 0x03
	Quorum      Consistency = 0x04
	All         Consistency = 0x05
	LocalQuorum Consistency = 0x06
	EachQuorum  Consistency = 0x07
	Serial      Consistency = 0x08
	LocalSerial Consistency = 0x09
	LocalOne    Consistency = 0x0A
)

func (c Consistency) String() string {
	switch c {
	case Any:
		return "ANY"
	case One:
		return "ONE"
	case Two:
		return "TWO"
	case Three:
		return "THREE"
	case Quorum:
		return "QUORUM"
	case All:
		return "ALL"
	case LocalQuorum:
		return "LOCAL_QUORUM"
	case EachQuorum:
		return "EACH_QUORUM"
	case Serial:
		return "SERIAL"
	case LocalSerial:
		return "LOCAL_SERIAL"
	case LocalOne:
		return "LOCAL_ONE"
	default:
		return fmt.Sprintf("UNKNOWN_CONS_0x%x", uint16(c))
	}
}

func (c Consistency) IsSerial() bool {
	return c == Serial || c == LocalSerial
}

func (c Consistency) MarshalText() (text []byte, err error) {
	return []byte(c.String()), nil
}

func (c *Consistency) UnmarshalText(text []byte) error {
	switch string(text) {
	case "ANY":
		*c = Any
	case "ONE":
		*c = One
	case "TWO":
		*c = Two
	case "THREE":
		*c = Three
	case "QUORUM":
		*c = Quorum
	case "ALL":
		*c = All
	case "LOCAL_QUORUM":
		*c = LocalQuorum
	case "EACH_QUORUM":
		*c = EachQuorum
	case "SERIAL":
		*c = Serial
	case "LOCAL_SERIAL":
		*c = LocalSerial
	case "LOCAL_ONE":
		*c = LocalOne
	default:
		return fmt.Errorf("invalid consistency %q", string(text))
	}
	return nil
}

func ParseConsistency(s string) Consistency {
	var c Consistency
	if err := c.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		panic(err)
	}
	return c
}

// ParseConsistencyWrapper wraps gocql.ParseConsistency to provide an err
// return instead of a panic
func ParseConsistencyWrapper(s string) (consistency Consistency, err error) {
	err = consistency.UnmarshalText([]byte(strings.ToUpper(s)))
	return
}

const (
	apacheCassandraTypePrefix = "org.apache.cassandra.db.marshal."
)

var (
	ErrFrameTooBig       = errors.New("frame length is bigger than the maximum allowed")
	ErrReadHeaderTimeout = errors.New("unable to read frame header")
)

func readInt(p []byte) int32 {
	return int32(binary.BigEndian.Uint32(p[:4]))
}

const defaultBufSize = 128

type ObservedFrameHeader struct {
	// StartHeader is the time we started reading the frame header off the network connection.
	//
	// On protocol v5 the header arrives inside a transport segment, so this is
	// when the read of that segment began. Frames packed into one self-contained
	// segment therefore share a window: they did all arrive in the same read.
	Start time.Time
	// EndHeader is the time we finished reading the frame header off the network connection.
	//
	// On protocol v5 this is when the segment carrying the header had been read —
	// or, for a frame whose header is itself split across segments, when the last
	// segment needed to complete the 9 header bytes arrived. It is not extended to
	// cover the rest of a frame spanning further segments.
	End time.Time
	// Host is Host of the connection the frame header was read from.
	Host    *HostInfo
	Length  int32
	Stream  int16
	Version frm.ProtoVersion
	Flags   byte
	Opcode  frm.Op
}

func (f ObservedFrameHeader) String() string {
	return fmt.Sprintf("[observed header version=%s flags=0x%x stream=%d op=%s length=%d]", f.Version, f.Flags, f.Stream, f.Opcode, f.Length)
}

// FrameHeaderObserver is the interface implemented by frame observers / stat collectors.
//
// Experimental, this interface and use may change
type FrameHeaderObserver interface {
	// ObserveFrameHeader gets called on every received frame header.
	ObserveFrameHeader(context.Context, ObservedFrameHeader)
}

// framerInterface represents a frame reader/writer for the CQL protocol.
//
// Framers are pooled and reused. Any byte slices returned from frame parsing
// methods may be backed by pooled buffers that are reused after Release() is
// called. If data must outlive the framer, use readBytesCopy() instead of
// readBytes() when implementing parseFrame(), or copy returned byte slices
// before calling Release().
//
// After Release() is called, the framer and any slices derived from its
// buffers must not be accessed.
type framerInterface interface {
	ReadBytesInternal() ([]byte, error)
	GetCustomPayload() map[string][]byte
	GetHeaderWarnings() []string
	// Release returns the framer to its pool (if pooled).
	// Must be called when the framer is no longer needed.
	// Safe to call multiple times; subsequent calls are no-ops.
	Release()
}

const headSize = 9

// a framer is responsible for reading, writing and parsing frames on a single stream
type framer struct {
	compressor    Compressor
	header        *frm.FrameHeader
	customPayload map[string][]byte
	release       func()
	traceID       []byte
	readBuffer    []byte
	buf           []byte
	// wireBuf is the framer's second reusable byte buffer. prepareModernLayout
	// encodes the v5 transport segments into it and then swaps it with buf, so a
	// framer reused for consecutive v5 requests keeps both the raw-frame buffer
	// and the wire buffer alive instead of allocating a new one per request.
	wireBuf               []byte
	flagLWT               int
	rateLimitingErrorCode int
	flags                 byte
	proto                 byte
	tabletsRoutingV1      bool
	scyllaUseMetadataID   bool
	released              atomic.Bool
}

// defaultFramerFlags computes the default header flags a framer carries for the
// given compressor and negotiated protocol version. It is the single source of
// truth shared by newFramer and initCache so the startup/fallback path and the
// pooled framers cannot drift.
//
// Only FlagCompress is derived, and only below proto v5 (v5+ compresses at the
// segment layer, not via a frame-header flag). It depends on the negotiated
// compressor, so it must only be applied after startup: the server may reject the
// requested compression, which clears c.compressor. No version-derived flag is
// added — in particular FlagBetaProtocol is never set, see protoVersion5.
func defaultFramerFlags(compressor Compressor, version byte) byte {
	// Mask off the direction/reserved high bit: newFramer is called with the
	// unmasked version byte, and an unmasked byte must not defeat the v5 check and
	// re-enable frame-header compression on v5.
	if compressor != nil && version&protoVersionMask < protoVersion5 {
		return frm.FlagCompress
	}
	return 0
}

func newFramer(compressor Compressor, version byte) *framer {
	buf := make([]byte, defaultBufSize)
	f := &framer{
		buf:        buf[:0],
		readBuffer: buf,
	}
	flags := defaultFramerFlags(compressor, version)

	version &= protoVersionMask
	f.compressor = compressor
	f.proto = version
	f.flags = flags
	f.header = nil
	f.traceID = nil

	f.tabletsRoutingV1 = false
	f.scyllaUseMetadataID = false

	return f
}

// Release returns the framer to its pool. If the framer was not obtained
// from a pool (release is nil), this is a no-op.
//
// Conn.releaseFramer owns the released-state guard, so this method delegates
// directly to the release closure.
func (f *framer) Release() {
	if f.release != nil {
		f.release()
	}
}

type frame interface {
	Header() frm.FrameHeader
}

func readHeader(r io.Reader, p []byte) (head frm.FrameHeader, err error) {
	n, err := io.ReadFull(r, p[:headSize])
	if err != nil {
		// A timeout that consumed nothing is the benign idle wait for the next
		// frame, which serve() recovers from by simply reading again. A timeout that
		// consumed part of a header is not: the stream position is now unknown, so
		// resuming would mis-frame everything after it. Only the former is
		// normalised to ErrReadHeaderTimeout; the latter stays a plain error and
		// takes the connection down.
		//
		// errors.As rather than a bare type assertion, matching
		// Conn.readFirstSegmentHeader: the two timeout checks must stay in
		// agreement even if a caller in between starts wrapping the error.
		var netErr net.Error
		if n == 0 && errors.As(err, &netErr) && netErr.Timeout() {
			return frm.FrameHeader{}, fmt.Errorf("%w: %w", ErrReadHeaderTimeout, err)
		}
		return frm.FrameHeader{}, err
	}

	head.Version = frm.ProtoVersion(p[0])
	version := head.Version.Version()

	if version < protoVersion3 || version > protoVersion5 {
		return frm.FrameHeader{}, fmt.Errorf("gocql: unsupported protocol response version: %d", version)
	}

	head.Flags = p[1]

	head.Stream = int(int16(binary.BigEndian.Uint16(p[2:4])))
	head.Op = frm.Op(p[4])
	head.Length = int(readInt(p[5:]))

	// The length is a signed 32-bit field on the wire, so any value with the high
	// bit set arrives negative. Reject it here rather than further down in
	// readFrame: a header this broken means the stream position is no longer
	// trustworthy, and only an error out of readHeader closes the connection —
	// readFrame's error is handed to the waiting caller while serve() reads on.
	// recvSplitFrame applies the same bound to a reassembled frame.
	if head.Length < 0 {
		return frm.FrameHeader{}, fmt.Errorf("gocql: invalid frame body length: %d", head.Length)
	}

	return head, nil
}

// explicitly enables tracing for the framers outgoing requests
func (f *framer) trace() {
	f.flags |= frm.FlagTracing
}

// explicitly enables the custom payload flag
func (f *framer) payload() {
	f.flags |= frm.FlagCustomPayload
}

// reads a frame form the wire into the framers buffer
func (f *framer) readFrame(r io.Reader, head *frm.FrameHeader) error {
	// A negative length is rejected by readHeader, which is where it has to be
	// caught: there the error is fatal to the connection, whereas an error from
	// here is delivered to the waiting caller. Kept as a precondition check for
	// callers that synthesise a header.
	if head.Length < 0 {
		return fmt.Errorf("frame body length can not be less than 0: %d", head.Length)
	} else if head.Length > maxFrameSize {
		// need to free up the connection to be used again
		_, err := io.CopyN(io.Discard, r, int64(head.Length))
		if err != nil {
			// %w, not %v: a failed discard leaves the undiscarded remainder of the
			// body on the wire, so the caller has to be able to recognise the read
			// failure here (Conn.bodyReadDesyncedConn) and close the connection rather
			// than read the leftover bytes as the next frame header.
			return fmt.Errorf("error whilst trying to discard frame with invalid length: %w", err)
		}
		return ErrFrameTooBig
	}

	if cap(f.readBuffer) >= head.Length {
		f.buf = f.readBuffer[:head.Length]
	} else {
		f.readBuffer = make([]byte, head.Length)
		f.buf = f.readBuffer
	}

	n, err := io.ReadFull(r, f.buf)
	if err != nil {
		// %w, not %v: a partially read body leaves the rest of it on the wire, so
		// the connection is desynced and must be closed. Conn.bodyReadDesyncedConn
		// decides that by inspecting the wrapped error, which a %v-formatted error
		// would defeat — the connection would be reused and every later frame
		// mis-framed.
		return fmt.Errorf("unable to read frame body: read %d/%d bytes: %w", n, head.Length, err)
	}

	if f.proto < protoVersion5 && head.Flags&frm.FlagCompress == frm.FlagCompress {
		if f.compressor == nil {
			return NewErrProtocol("no compressor available with compressed frame body")
		}

		f.buf, err = f.compressor.Decode(f.buf)
		if err != nil {
			return err
		}
	}

	f.header = head
	return nil
}

// adoptFrameBody takes ownership of a frame body that is already fully in memory,
// instead of reading and copying it as readFrame does. It is used for a v5 frame
// reassembled from several transport segments (Conn.recvSplitFrame): that buffer
// is already exactly frame-sized, so copying it would mean holding the frame twice
// — 512 MiB for a maxFrameSize response.
//
// f.readBuffer is deliberately left pointing at the pooled buffer, so releasing
// the framer drops the adopted body instead of retaining an outsized buffer in the
// framer pool. Segment-level decompression has already happened, so unlike
// readFrame there is no frame-header compression flag to honour (it only exists
// below v5).
func (f *framer) adoptFrameBody(body []byte, head *frm.FrameHeader) error {
	if head.Length != len(body) {
		return fmt.Errorf("gocql: frame body length %d does not match the %d bytes reassembled", head.Length, len(body))
	}
	f.buf = body
	f.header = head
	return nil
}

func (f *framer) parseFrame() (frame frame, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()

	if f.header.Version.Request() {
		return nil, NewErrProtocol("got a request frame from server: %v", f.header.Version)
	}

	if f.header.Flags&frm.FlagTracing == frm.FlagTracing {
		f.readTrace()
	}

	if f.header.Flags&frm.FlagWarning == frm.FlagWarning {
		f.header.Warnings = f.readStringList()
	}

	if f.header.Flags&frm.FlagCustomPayload == frm.FlagCustomPayload {
		f.customPayload = f.readBytesMap()
	}

	// assumes that the frame body has been read into rbuf
	switch f.header.Op {
	case frm.OpError:
		frame = f.parseErrorFrame()
	case frm.OpReady:
		frame = f.parseReadyFrame()
	case frm.OpResult:
		frame, err = f.parseResultFrame()
	case frm.OpSupported:
		frame = f.parseSupportedFrame()
	case frm.OpAuthenticate:
		frame = f.parseAuthenticateFrame()
	case frm.OpAuthChallenge:
		frame = f.parseAuthChallengeFrame()
	case frm.OpAuthSuccess:
		frame = f.parseAuthSuccessFrame()
	case frm.OpEvent:
		frame = f.parseEventFrame()
	default:
		return nil, NewErrProtocol("unknown op in frame header: %s", f.header.Op)
	}

	return
}

func (f *framer) parseErrorFrame() frame {
	code := f.readInt()
	msg := f.readString()

	errD := frm.ErrorFrame{
		FrameHeader: *f.header,
		Code:        code,
		Message:     msg,
	}

	switch code {
	case ErrCodeUnavailable:
		cl := f.readConsistency()
		required := f.readInt()
		alive := f.readInt()
		return &RequestErrUnavailable{
			ErrorFrame:  errD,
			Consistency: cl,
			Required:    required,
			Alive:       alive,
		}
	case ErrCodeWriteTimeout:
		cl := f.readConsistency()
		received := f.readInt()
		blockfor := f.readInt()
		writeType := f.readString()
		return &RequestErrWriteTimeout{
			ErrorFrame:  errD,
			Consistency: cl,
			Received:    received,
			BlockFor:    blockfor,
			WriteType:   writeType,
		}
	case ErrCodeReadTimeout:
		cl := f.readConsistency()
		received := f.readInt()
		blockfor := f.readInt()
		dataPresent := f.readByte()
		return &RequestErrReadTimeout{
			ErrorFrame:  errD,
			Consistency: cl,
			Received:    received,
			BlockFor:    blockfor,
			DataPresent: dataPresent,
		}
	case ErrCodeAlreadyExists:
		ks := f.readString()
		table := f.readString()
		return &RequestErrAlreadyExists{
			ErrorFrame: errD,
			Keyspace:   ks,
			Table:      table,
		}
	case ErrCodeUnprepared:
		return &RequestErrUnprepared{
			ErrorFrame:  errD,
			StatementId: f.readShortBytesCopy(),
		}
	case ErrCodeReadFailure:
		res := &RequestErrReadFailure{
			ErrorFrame: errD,
		}
		res.Consistency = f.readConsistency()
		res.Received = f.readInt()
		res.BlockFor = f.readInt()
		if f.proto > protoVersion4 {
			res.ErrorMap = f.readErrorMap()
			res.NumFailures = len(res.ErrorMap)
		} else {
			res.NumFailures = f.readInt()
		}
		res.DataPresent = f.readByte() != 0

		return res
	case ErrCodeWriteFailure:
		res := &RequestErrWriteFailure{
			ErrorFrame: errD,
		}
		res.Consistency = f.readConsistency()
		res.Received = f.readInt()
		res.BlockFor = f.readInt()
		if f.proto > protoVersion4 {
			res.ErrorMap = f.readErrorMap()
			res.NumFailures = len(res.ErrorMap)
		} else {
			res.NumFailures = f.readInt()
		}
		res.WriteType = f.readString()
		return res
	case ErrCodeFunctionFailure:
		res := &RequestErrFunctionFailure{
			ErrorFrame: errD,
		}
		res.Keyspace = f.readString()
		res.Function = f.readString()
		res.ArgTypes = f.readStringList()
		return res

	case ErrCodeCDCWriteFailure:
		res := &RequestErrCDCWriteFailure{
			ErrorFrame: errD,
		}
		return res
	case ErrCodeCASWriteUnknown:
		res := &RequestErrCASWriteUnknown{
			ErrorFrame: errD,
		}
		res.Consistency = f.readConsistency()
		res.Received = f.readInt()
		res.BlockFor = f.readInt()
		return res
	case ErrCodeInvalid, ErrCodeBootstrapping, ErrCodeConfig, ErrCodeCredentials, ErrCodeOverloaded,
		ErrCodeProtocol, ErrCodeServer, ErrCodeSyntax, ErrCodeTruncate, ErrCodeUnauthorized:
		// TODO(zariel): we should have some distinct types for these errors
		return errD
	default:
		if f.rateLimitingErrorCode != 0 && code == f.rateLimitingErrorCode {
			res := &RequestErrRateLimitReached{
				ErrorFrame: errD,
			}
			res.OpType = OpType(f.readByte())
			res.RejectedByCoordinator = f.readByte() != 0
			return res
		} else {
			return &UnknownServerError{
				ErrorFrame: errD,
			}
		}
	}
}

func (f *framer) readErrorMap() (errMap ErrorMap) {
	errMap = make(ErrorMap)
	numErrs := f.readInt()
	for i := 0; i < numErrs; i++ {
		ip := f.readInetAdressOnly().String()
		errMap[ip] = f.readShort()
	}
	return
}

func (f *framer) writeHeader(flags byte, op frm.Op, stream int) {
	f.buf = append(f.buf[:0],
		f.proto, flags, byte(stream>>8), byte(stream),
		// pad out length
		byte(op), 0, 0, 0, 0,
	)
}

func (f *framer) setLength(length int) {
	f.buf[5] = byte(length >> 24)
	f.buf[6] = byte(length >> 16)
	f.buf[7] = byte(length >> 8)
	f.buf[8] = byte(length)
}

func (f *framer) finish() error {
	bufLen := len(f.buf)
	if bufLen > maxFrameSize {
		// huge app frame, lets remove it so it doesn't bloat the heap
		f.buf = make([]byte, defaultBufSize)
		return ErrFrameTooBig
	}

	if f.proto < protoVersion5 && f.buf[1]&frm.FlagCompress == frm.FlagCompress {
		if f.compressor == nil {
			panic("compress flag set with no compressor")
		}

		// TODO: only compress frames which are big enough
		compressed, err := f.compressor.Encode(f.buf[headSize:])
		if err != nil {
			return err
		}

		f.buf = append(f.buf[:headSize], compressed...)
		bufLen = len(f.buf)
	}
	length := bufLen - headSize
	f.setLength(length)

	return nil
}

func (f *framer) writeTo(w io.Writer) error {
	_, err := w.Write(f.buf)
	return err
}

func (f *framer) readTrace() {
	if len(f.buf) < 16 {
		panic(fmt.Errorf("not enough bytes in buffer to read trace uuid require 16 got: %d", len(f.buf)))
	}
	if len(f.traceID) != 16 {
		f.traceID = make([]byte, 16)
	}
	copy(f.traceID, f.buf[:16])
	f.buf = f.buf[16:]
}

func (f *framer) parseReadyFrame() frame {
	return &frm.ReadyFrame{
		FrameHeader: *f.header,
	}
}

// TODO: if we move the body buffer onto the frameHeader then we only need a single
// framer, and can move the methods onto the header.
func (f *framer) parseSupportedFrame() frame {
	return &frm.SupportedFrame{
		FrameHeader: *f.header,

		Supported: f.readStringMultiMap(),
	}
}

type writeStartupFrame struct {
	opts map[string]string
}

func (w writeStartupFrame) String() string {
	return fmt.Sprintf("[startup opts=%+v]", w.opts)
}

func (w *writeStartupFrame) buildFrame(f *framer, streamID int) error {
	f.writeHeader(f.flags&^frm.FlagCompress, frm.OpStartup, streamID)
	f.writeStringMap(w.opts)

	return f.finish()
}

type writePrepareFrame struct {
	customPayload map[string][]byte
	statement     string
	keyspace      string
}

func (w *writePrepareFrame) buildFrame(f *framer, streamID int) error {
	// Validate before writing anything into f.buf so an error never leaves a
	// partial frame in the reusable framer buffer.
	if err := f.validateV5Options(w.keyspace, nil); err != nil {
		return err
	}

	if len(w.customPayload) > 0 {
		f.payload()
	}
	f.writeHeader(f.flags, frm.OpPrepare, streamID)
	f.writeCustomPayload(&w.customPayload)
	f.writeLongString(w.statement)

	var flags uint32 = 0
	if w.keyspace != "" {
		flags |= frm.FlagWithPreparedKeyspace
	}
	if f.proto > protoVersion4 {
		f.writeUint(flags)
	}
	if w.keyspace != "" {
		f.writeString(w.keyspace)
	}

	return f.finish()
}

func (f *framer) readTypeInfo() TypeInfo {
	// TODO: factor this out so the same code paths can be used to parse custom
	// types and other types, as much of the logic will be duplicated.
	id := f.readShort()

	simple := NativeType{
		proto: f.proto,
		typ:   Type(id),
	}

	// Fast path for simple native types (through TypeDuration).
	if id > 0 && id <= uint16(TypeDuration) {
		return simple
	}

	if simple.typ == TypeCustom {
		simple.custom = f.readString()
		if cassType := getApacheCassandraType(simple.custom); cassType != TypeCustom {
			simple.typ = cassType
		}
	}

	switch simple.typ {
	case TypeTuple:
		n := f.readShort()
		tuple := TupleTypeInfo{
			NativeType: simple,
			Elems:      make([]TypeInfo, n),
		}

		for i := 0; i < int(n); i++ {
			tuple.Elems[i] = f.readTypeInfo()
		}

		return tuple

	case TypeUDT:
		udt := UDTTypeInfo{
			NativeType: simple,
		}
		udt.KeySpace = f.readString()
		udt.Name = f.readString()

		n := f.readShort()
		udt.Elements = make([]UDTField, n)
		for i := 0; i < int(n); i++ {
			field := &udt.Elements[i]
			field.Name = f.readString()
			field.Type = f.readTypeInfo()
		}

		return udt
	case TypeMap, TypeList, TypeSet:
		collection := CollectionType{
			NativeType: simple,
		}

		if simple.typ == TypeMap {
			collection.Key = f.readTypeInfo()
		}

		collection.Elem = f.readTypeInfo()

		return collection
	case TypeCustom:
		vectorTypePrefix := apacheCassandraTypePrefix + "VectorType"
		if strings.HasPrefix(simple.custom, vectorTypePrefix) {
			spec := strings.TrimPrefix(simple.custom, vectorTypePrefix)
			spec = spec[1 : len(spec)-1] // remove parenthesis
			idx := strings.LastIndex(spec, ",")
			typeStr := spec[:idx]
			dimStr := spec[idx+1:]
			subType := getCassandraLongType(strings.TrimSpace(typeStr), f.proto, nopLogger{})
			dim, _ := strconv.Atoi(strings.TrimSpace(dimStr))
			vector := VectorType{
				NativeType: simple,
				SubType:    subType,
				Dimensions: dim,
			}
			return vector
		}
	}

	return simple
}

type preparedMetadata struct {
	keyspace string
	table    string
	// proto v4+
	pkeyColumns []int
	resultMetadata
	// LWT query detected
	lwt bool
}

func (r preparedMetadata) String() string {
	return fmt.Sprintf("[prepared flags=0x%x pkey=%v paging_state=% X columns=%v col_count=%d actual_col_count=%d lwt=%t]",
		r.flags, r.pkeyColumns, r.pagingState, r.columns, r.colCount, r.actualColCount, r.lwt)
}

func (f *framer) parsePreparedMetadata() preparedMetadata {
	// TODO: deduplicate this from parseMetadata
	meta := preparedMetadata{}

	meta.flags = f.readInt()
	meta.colCount = f.readInt()
	if meta.colCount < 0 {
		panic(fmt.Errorf("received negative column count: %d", meta.colCount))
	}
	meta.actualColCount = meta.colCount

	if f.proto >= protoVersion4 {
		pkeyCount := f.readInt()
		// Like the colCount guard above, reject a negative count: make would panic
		// with a runtime error, which parseFrame's recover re-panics rather than
		// converting to an error. Unlike colCount — whose huge-value case is handled
		// by the make/append split further down — also reject a count larger than the
		// remaining buffer could supply: each pkey index is a short (2 bytes), so a
		// valid frame always has pkeyCount <= len(f.buf)/2. That bounds make() to the
		// actual frame size instead of a peer-declared count, so a small malformed
		// frame cannot force a large allocation.
		if pkeyCount < 0 || pkeyCount > len(f.buf)/2 {
			panic(fmt.Errorf("invalid partition key count %d (remaining %d bytes)", pkeyCount, len(f.buf)))
		}
		pkeys := make([]int, pkeyCount)
		for i := 0; i < pkeyCount; i++ {
			pkeys[i] = int(f.readShort())
		}
		meta.pkeyColumns = pkeys
	}

	meta.lwt = meta.flags&f.flagLWT == f.flagLWT

	if meta.flags&frm.FlagHasMorePages == frm.FlagHasMorePages {
		meta.pagingState = f.readBytesCopy()
	}

	if meta.flags&frm.FlagNoMetaData == frm.FlagNoMetaData {
		return meta
	}

	globalSpec := meta.flags&frm.FlagGlobalTableSpec == frm.FlagGlobalTableSpec
	if globalSpec {
		meta.keyspace = f.readString()
		meta.table = f.readString()
	}

	var cols []ColumnInfo
	readPerColumnSpec := !globalSpec
	var tracker keyspaceTableTracker
	if meta.colCount < 1000 {
		// preallocate columninfo to avoid excess copying
		cols = make([]ColumnInfo, meta.colCount)
		for i := 0; i < meta.colCount; i++ {
			col := &cols[i]
			keyspace, table := f.readColWithSpec(col, &meta.resultMetadata, globalSpec, meta.keyspace, meta.table, i, readPerColumnSpec)
			if readPerColumnSpec {
				tracker.track(i, keyspace, table)
			}
		}
	} else {
		// use append, huge number of columns usually indicates a corrupt frame or
		// just a huge row.
		for i := 0; i < meta.colCount; i++ {
			var col ColumnInfo
			keyspace, table := f.readColWithSpec(&col, &meta.resultMetadata, globalSpec, meta.keyspace, meta.table, i, readPerColumnSpec)
			if readPerColumnSpec {
				tracker.track(i, keyspace, table)
			}
			cols = append(cols, col)
		}
	}

	if !globalSpec && meta.colCount > 0 && tracker.allSame {
		meta.keyspace = tracker.keyspace
		meta.table = tracker.table
	}

	meta.columns = cols

	return meta
}

type resultMetadata struct {
	pagingState []byte
	// this is a count of the total number of columns which can be scanned,
	// it is at minimum len(columns) but may be larger, for instance when a column
	// is a UDT or tuple.
	columns        []ColumnInfo
	newMetadataID  []byte
	flags          int
	colCount       int
	actualColCount int
}

func (r *resultMetadata) morePages() bool {
	return r.flags&frm.FlagHasMorePages == frm.FlagHasMorePages
}

func (r *resultMetadata) noMetaData() bool {
	return r.flags&frm.FlagNoMetaData == frm.FlagNoMetaData
}

func (r resultMetadata) String() string {
	return fmt.Sprintf("[metadata flags=0x%x paging_state=% X columns=%v new_metadata_id=% X]", r.flags, r.pagingState, r.columns, r.newMetadataID)
}

// keyspaceTableTracker tracks whether all columns share the same keyspace/table.
type keyspaceTableTracker struct {
	keyspace string
	table    string
	allSame  bool
}

func (t *keyspaceTableTracker) track(colIndex int, keyspace, table string) {
	if colIndex == 0 {
		t.keyspace = keyspace
		t.table = table
		t.allSame = true
	} else if t.allSame && (keyspace != t.keyspace || table != t.table) {
		t.allSame = false
	}
}

func (f *framer) readColWithSpec(col *ColumnInfo, meta *resultMetadata, globalSpec bool, keyspace, table string, colIndex int, readPerColumnSpec bool) (string, string) {
	if readPerColumnSpec {
		// Per-column table spec encoding: read keyspace/table for this column.
		col.Keyspace = f.readString()
		col.Table = f.readString()
	} else {
		if !globalSpec && colIndex != 0 {
			// Skip per-column keyspace/table already read from column 0.
			f.skipString()
			f.skipString()
		}
		col.Keyspace = keyspace
		col.Table = table
	}

	col.Name = f.readString()
	col.TypeInfo = f.readTypeInfo()
	if tuple, ok := col.TypeInfo.(TupleTypeInfo); ok {
		// -1 because we already included the tuple column
		meta.actualColCount += len(tuple.Elems) - 1
	}

	return col.Keyspace, col.Table
}

func (f *framer) parseResultMetadata() resultMetadata {
	var meta resultMetadata

	meta.flags = f.readInt()
	meta.colCount = f.readInt()
	if meta.colCount < 0 {
		panic(fmt.Errorf("received negative column count: %d", meta.colCount))
	}
	meta.actualColCount = meta.colCount

	if meta.flags&frm.FlagHasMorePages == frm.FlagHasMorePages {
		meta.pagingState = f.readBytesCopy()
	}

	// The re-issue of a result metadata ID, reached from a RESULT/Rows whose
	// EXECUTE carried an ID the server found stale. It supersedes the one
	// RESULT/Prepared issued; see parseResultPrepared for the other half.
	//
	// Read after the paging state, matching Cassandra's encoder
	// (ResultSet$ResultMetadata$Codec.encode) and the v5 spec. See
	// TestParseResultMetadata_PagingStateBeforeNewMetadataID, which is the only
	// test that can distinguish the two orderings.
	if (f.proto > protoVersion4 || f.scyllaUseMetadataID) && meta.flags&frm.FlagMetaDataChanged == frm.FlagMetaDataChanged {
		meta.newMetadataID = f.readShortBytesCopy()
	}

	if meta.noMetaData() {
		return meta
	}

	globalSpec := meta.flags&frm.FlagGlobalTableSpec == frm.FlagGlobalTableSpec

	// Read keyspace/table once and reuse for all columns. ROWS results are
	// always single-table; when !globalSpec this consumes column 0's wire
	// values and readColWithSpec skips the rest via skipString().
	var keyspace, table string
	if globalSpec || meta.colCount > 0 {
		keyspace = f.readString()
		table = f.readString()
	}

	var cols []ColumnInfo
	if meta.colCount < 1000 {
		// preallocate columninfo to avoid excess copying
		cols = make([]ColumnInfo, meta.colCount)
		for i := 0; i < meta.colCount; i++ {
			f.readColWithSpec(&cols[i], &meta, globalSpec, keyspace, table, i, false)
		}

	} else {
		// use append, huge number of columns usually indicates a corrupt frame or
		// just a huge row.
		for i := 0; i < meta.colCount; i++ {
			var col ColumnInfo
			f.readColWithSpec(&col, &meta, globalSpec, keyspace, table, i, false)
			cols = append(cols, col)
		}
	}

	meta.columns = cols

	return meta
}

type resultVoidFrame struct {
	frm.FrameHeader
}

func (f *resultVoidFrame) String() string {
	return "[result_void]"
}

func (f *framer) parseResultFrame() (frame, error) {
	kind := f.readInt()

	switch kind {
	case frm.ResultKindVoid:
		return &resultVoidFrame{FrameHeader: *f.header}, nil
	case frm.ResultKindRows:
		return f.parseResultRows(), nil
	case frm.ResultKindKeyspace:
		return f.parseResultSetKeyspace(), nil
	case frm.ResultKindPrepared:
		return f.parseResultPrepared(), nil
	case frm.ResultKindSchemaChanged:
		return f.parseResultSchemaChange(), nil
	}

	return nil, NewErrProtocol("unknown result kind: %x", kind)
}

type resultRowsFrame struct {
	frm.FrameHeader

	meta resultMetadata
	// dont parse the rows here as we only need to do it once
	numRows int
}

func (f *resultRowsFrame) String() string {
	return fmt.Sprintf("[result_rows meta=%v]", f.meta)
}

func (f *framer) parseResultRows() frame {
	result := &resultRowsFrame{}
	result.meta = f.parseResultMetadata()

	result.numRows = f.readInt()
	if result.numRows < 0 {
		panic(fmt.Errorf("invalid row_count in result frame: %d", result.numRows))
	}

	return result
}

type resultKeyspaceFrame struct {
	keyspace string
	frm.FrameHeader
}

func (r *resultKeyspaceFrame) String() string {
	return fmt.Sprintf("[result_keyspace keyspace=%s]", r.keyspace)
}

func (f *framer) parseResultSetKeyspace() frame {
	return &resultKeyspaceFrame{
		FrameHeader: *f.header,
		keyspace:    f.readString(),
	}
}

type resultPreparedFrame struct {
	preparedID       []byte
	resultMetadataID []byte
	respMeta         resultMetadata
	frm.FrameHeader
	reqMeta preparedMetadata
}

// parseResultPrepared parses a RESULT/Prepared body:
//
//	<id>                 [short bytes]  prepared statement ID
//	<result_metadata_id> [short bytes]  v5, or v4 with SCYLLA_USE_METADATA_ID
//	<metadata>           bind variables and partition key indexes (request side)
//	<result_metadata>    the columns rows will carry (response side)
//
// The two metadata blocks describe opposite directions and are unrelated; only
// the second one has an ID, because only it can go stale without the driver
// noticing.
//
// The ID read here is the first one for this statement, issued alongside the
// metadata it identifies and echoed back by every later EXECUTE. A superseding
// ID arrives by a different route — newMetadataID inside a RESULT/Rows, behind
// METADATA_CHANGED — so that the server can repair a stale ID in the response it
// was already sending instead of making the driver re-prepare. Both land in
// preparedStatment.resultMetadataID.
//
// parseResultMetadata below is the same codec RESULT/Rows uses, as it is in
// Cassandra (ResultSet$ResultMetadata$Codec), so it reads newMetadataID whenever
// METADATA_CHANGED is set. Nothing sets it here: that flag says a previously
// issued ID is stale, which cannot apply to the response issuing the first one.
func (f *framer) parseResultPrepared() frame {
	frame := &resultPreparedFrame{
		FrameHeader: *f.header,
		preparedID:  f.readShortBytesCopy(),
	}

	if f.proto > protoVersion4 || f.scyllaUseMetadataID {
		frame.resultMetadataID = f.readShortBytesCopy()
	}

	frame.reqMeta = f.parsePreparedMetadata()
	frame.respMeta = f.parseResultMetadata()

	return frame
}

func (f *framer) parseResultSchemaChange() frame {
	change := f.readString()
	target := f.readString()

	// TODO: could just use a separate type for each target
	switch target {
	case "KEYSPACE":
		return &frm.SchemaChangeKeyspace{
			FrameHeader: *f.header,
			Change:      change,
			Keyspace:    f.readString(),
		}
	case "TABLE":
		return &frm.SchemaChangeTable{
			FrameHeader: *f.header,
			Change:      change,
			Keyspace:    f.readString(),
			Object:      f.readString(),
		}
	case "TYPE":
		return &frm.SchemaChangeType{
			FrameHeader: *f.header,
			Change:      change,
			Keyspace:    f.readString(),
			Object:      f.readString(),
		}
	case "FUNCTION":
		return &frm.SchemaChangeFunction{
			FrameHeader: *f.header,
			Change:      change,
			Keyspace:    f.readString(),
			Name:        f.readString(),
			Args:        f.readStringList(),
		}
	case "AGGREGATE":
		return &frm.SchemaChangeAggregate{
			FrameHeader: *f.header,
			Change:      change,
			Keyspace:    f.readString(),
			Name:        f.readString(),
			Args:        f.readStringList(),
		}
	default:
		panic(fmt.Errorf("gocql: unknown SCHEMA_CHANGE target: %q change: %q", target, change))
	}
}

func (f *framer) parseAuthenticateFrame() frame {
	return &frm.AuthenticateFrame{
		FrameHeader: *f.header,
		Class:       f.readString(),
	}
}

func (f *framer) parseAuthSuccessFrame() frame {
	return &frm.AuthSuccessFrame{
		FrameHeader: *f.header,
		Data:        f.readBytesCopy(),
	}
}

func (f *framer) parseAuthChallengeFrame() frame {
	return &frm.AuthChallengeFrame{
		FrameHeader: *f.header,
		Data:        f.readBytesCopy(),
	}
}

func (f *framer) parseEventFrame() frame {
	eventType := f.readString()

	switch eventType {
	case "TOPOLOGY_CHANGE":
		frame := &frm.TopologyChangeEventFrame{FrameHeader: *f.header}
		frame.Change = f.readString()
		frame.Host, frame.Port = f.readInet()

		return frame
	case "STATUS_CHANGE":
		frame := &frm.StatusChangeEventFrame{FrameHeader: *f.header}
		frame.Change = f.readString()
		frame.Host, frame.Port = f.readInet()

		return frame
	case "SCHEMA_CHANGE":
		// this should work for all versions
		return f.parseResultSchemaChange()
	case "CLIENT_ROUTES_CHANGE":
		return &frm.ClientRoutesChanged{
			FrameHeader:   *f.header,
			ChangeType:    f.readString(),
			ConnectionIDs: f.readStringList(),
			HostIDs:       f.readStringList(),
		}
	default:
		panic(fmt.Errorf("gocql: unknown event type: %q", eventType))
	}

}

type writeAuthResponseFrame struct {
	data []byte
}

func (a *writeAuthResponseFrame) String() string {
	return fmt.Sprintf("[auth_response data=%q]", a.data)
}

func (a *writeAuthResponseFrame) buildFrame(framer *framer, streamID int) error {
	return framer.writeAuthResponseFrame(streamID, a.data)
}

func (f *framer) writeAuthResponseFrame(streamID int, data []byte) error {
	f.writeHeader(f.flags, frm.OpAuthResponse, streamID)
	f.writeBytes(data)
	return f.finish()
}

type queryValues struct {
	name    string
	value   []byte
	isUnset bool
}

type queryParams struct {
	nowInSeconds          *int
	keyspace              string
	values                []queryValues
	pagingState           []byte
	pageSize              int
	defaultTimestampValue int64
	consistency           Consistency
	serialConsistency     Consistency
	skipMeta              bool
	defaultTimestamp      bool
}

func (q queryParams) String() string {
	return fmt.Sprintf("[query_params consistency=%v skip_meta=%v page_size=%d paging_state=%q serial_consistency=%v default_timestamp=%v values=%v keyspace=%s now_in_seconds=%v]",
		q.consistency, q.skipMeta, q.pageSize, q.pagingState, q.serialConsistency, q.defaultTimestamp, q.values, q.keyspace, q.nowInSeconds)
}

// validateV5Options rejects the request options that only exist from protocol v5
// onwards, and the one v5 option whose value has to fit the wire type. Callers
// pass nil for nowInSeconds when their frame has no such field.
//
// It is the single source of truth for these checks, shared by every writer that
// accepts them (QUERY/EXECUTE via writeQueryParams, BATCH, PREPARE), so the three
// cannot drift apart in what they reject or in what they say. Each buildFrame also
// calls it before writing any byte, so a rejected option cannot leave a partially
// serialised frame behind.
func (f *framer) validateV5Options(keyspace string, nowInSeconds *int) error {
	if keyspace != "" && f.proto < protoVersion5 {
		return fmt.Errorf("gocql: keyspace override can only be set with protocol v5 or higher, current protocol: %d", f.proto)
	}
	if nowInSeconds != nil {
		if f.proto < protoVersion5 {
			return fmt.Errorf("gocql: now_in_seconds can only be set with protocol v5 or higher, current protocol: %d", f.proto)
		}
		if v := *nowInSeconds; v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("gocql: nowInSeconds value %d overflows int32", v)
		}
	}
	return nil
}

func (f *framer) writeQueryParams(opts *queryParams) error {
	// Validated again here, not only in the callers' buildFrame: this function is
	// package-internal and nothing else would enforce the precondition.
	if err := f.validateV5Options(opts.keyspace, opts.nowInSeconds); err != nil {
		return err
	}

	f.writeConsistency(opts.consistency)

	var flags uint32
	names := false

	if len(opts.values) > 0 {
		flags |= frm.FlagValues
	}
	if opts.skipMeta {
		flags |= frm.FlagSkipMetaData
	}
	if opts.pageSize > 0 {
		flags |= frm.FlagPageSize
	}
	if len(opts.pagingState) > 0 {
		flags |= frm.FlagWithPagingState
	}
	if opts.serialConsistency > 0 {
		flags |= frm.FlagWithSerialConsistency
	}

	// protoV3 specific things
	if opts.defaultTimestamp {
		flags |= frm.FlagDefaultTimestamp
	}

	if len(opts.values) > 0 && opts.values[0].name != "" {
		flags |= frm.FlagWithNameValues
		names = true
	}

	if opts.keyspace != "" {
		flags |= frm.FlagWithKeyspace
	}

	if opts.nowInSeconds != nil {
		flags |= frm.FlagWithNowInSeconds
	}

	if f.proto > protoVersion4 {
		f.writeUint(flags)
	} else {
		f.writeByte(byte(flags))
	}

	if n := len(opts.values); n > 0 {
		f.writeShort(uint16(n))

		for i := 0; i < n; i++ {
			if names {
				f.writeString(opts.values[i].name)
			}
			if opts.values[i].isUnset {
				f.writeUnset()
			} else {
				f.writeBytes(opts.values[i].value)
			}
		}
	}

	if opts.pageSize > 0 {
		f.writeInt(int32(opts.pageSize))
	}

	if len(opts.pagingState) > 0 {
		f.writeBytes(opts.pagingState)
	}

	if opts.serialConsistency > 0 {
		f.writeConsistency(opts.serialConsistency)
	}

	if opts.defaultTimestamp {
		// timestamp in microseconds
		var ts int64
		if opts.defaultTimestampValue != 0 {
			ts = opts.defaultTimestampValue
		} else {
			ts = time.Now().UnixNano() / 1000
		}
		f.writeLong(ts)
	}

	if opts.keyspace != "" {
		f.writeString(opts.keyspace)
	}

	if opts.nowInSeconds != nil {
		// Bounds already validated at the top of this function.
		f.writeInt(int32(*opts.nowInSeconds))
	}
	return nil
}

type writeQueryFrame struct {
	customPayload map[string][]byte
	statement     string
	params        queryParams
}

func (w *writeQueryFrame) String() string {
	return fmt.Sprintf("[query statement=%q params=%v]", w.statement, w.params)
}

func (w *writeQueryFrame) buildFrame(framer *framer, streamID int) error {
	return framer.writeQueryFrame(streamID, w.statement, &w.params, w.customPayload)
}

func (f *framer) writeQueryFrame(streamID int, statement string, params *queryParams, customPayload map[string][]byte) error {
	// Validate before writing anything into f.buf, as the PREPARE and BATCH
	// builders do. writeQueryParams performs the same check, but by the time it
	// runs the header, the custom payload and the statement have already been
	// written, so its own "nothing was written yet" guarantee would not hold for
	// this path.
	if err := f.validateV5Options(params.keyspace, params.nowInSeconds); err != nil {
		return err
	}

	if len(customPayload) > 0 {
		f.payload()
	}
	f.writeHeader(f.flags, frm.OpQuery, streamID)
	f.writeCustomPayload(&customPayload)
	f.writeLongString(statement)
	if err := f.writeQueryParams(params); err != nil {
		return err
	}

	return f.finish()
}

type frameBuilder interface {
	buildFrame(framer *framer, streamID int) error
}

type frameWriterFunc func(framer *framer, streamID int) error

func (f frameWriterFunc) buildFrame(framer *framer, streamID int) error {
	return f(framer, streamID)
}

type writeExecuteFrame struct {
	customPayload    map[string][]byte
	preparedID       []byte
	resultMetadataID []byte
	params           queryParams
}

func (e *writeExecuteFrame) String() string {
	return fmt.Sprintf("[execute id=% X params=%v]", e.preparedID, &e.params)
}

func (e *writeExecuteFrame) buildFrame(fr *framer, streamID int) error {
	return fr.writeExecuteFrame(streamID, e.preparedID, e.resultMetadataID, &e.params, &e.customPayload)
}

func (f *framer) writeExecuteFrame(streamID int, preparedID, resultMetadataID []byte, params *queryParams, customPayload *map[string][]byte) error {
	// Validate first, as in writeQueryFrame: the prepared id (and, on v5, the
	// result metadata id) are written before writeQueryParams runs.
	if err := f.validateV5Options(params.keyspace, params.nowInSeconds); err != nil {
		return err
	}

	if len(*customPayload) > 0 {
		f.payload()
	}
	f.writeHeader(f.flags, frm.OpExecute, streamID)
	f.writeCustomPayload(customPayload)
	f.writeShortBytes(preparedID)

	if f.proto > protoVersion4 || f.scyllaUseMetadataID {
		f.writeShortBytes(resultMetadataID)
	}

	if err := f.writeQueryParams(params); err != nil {
		return err
	}

	return f.finish()
}

// TODO: can we replace BatchStatemt with batchStatement? As they prety much
// duplicate each other
type batchStatment struct {
	preparedID []byte
	statement  string
	values     []queryValues
}

type writeBatchFrame struct {
	customPayload         map[string][]byte
	nowInSeconds          *int
	keyspace              string
	statements            []batchStatment
	defaultTimestampValue int64
	consistency           Consistency
	serialConsistency     Consistency
	typ                   BatchType
	defaultTimestamp      bool
}

func (w *writeBatchFrame) buildFrame(framer *framer, streamID int) error {
	return framer.writeBatchFrame(streamID, w, w.customPayload)
}

func (f *framer) writeBatchFrame(streamID int, w *writeBatchFrame, customPayload map[string][]byte) error {
	// Validate everything that can fail BEFORE writing anything into f.buf, so
	// an error never leaves a partial frame in the reusable framer buffer.
	if err := f.validateV5Options(w.keyspace, w.nowInSeconds); err != nil {
		return err
	}

	// Named values are not supported in batches on any protocol version
	// (CASSANDRA-10246). Reject them up front, before any bytes are written,
	// so a rejected batch never leaves a partial frame in the reusable buffer.
	for i := range w.statements {
		for j := range w.statements[i].values {
			if w.statements[i].values[j].name != "" {
				return fmt.Errorf("gocql: named query values are not supported in batches, please see https://issues.apache.org/jira/browse/CASSANDRA-10246")
			}
		}
	}

	if len(customPayload) > 0 {
		f.payload()
	}
	f.writeHeader(f.flags, frm.OpBatch, streamID)
	f.writeCustomPayload(&customPayload)
	f.writeByte(byte(w.typ))

	n := len(w.statements)
	f.writeShort(uint16(n))

	var flags uint32

	for i := 0; i < n; i++ {
		b := &w.statements[i]
		if len(b.preparedID) == 0 {
			f.writeByte(0)
			f.writeLongString(b.statement)
		} else {
			f.writeByte(1)
			f.writeShortBytes(b.preparedID)
		}

		f.writeShort(uint16(len(b.values)))
		for j := range b.values {
			col := b.values[j]
			if col.isUnset {
				f.writeUnset()
			} else {
				f.writeBytes(col.value)
			}
		}
	}

	f.writeConsistency(w.consistency)

	if f.proto > protoVersion2 {
		if w.serialConsistency > 0 {
			flags |= frm.FlagWithSerialConsistency
		}
		if w.defaultTimestamp {
			flags |= frm.FlagDefaultTimestamp
		}
	}

	if w.keyspace != "" {
		flags |= frm.FlagWithKeyspace
	}

	if w.nowInSeconds != nil {
		flags |= frm.FlagWithNowInSeconds
	}

	if f.proto > protoVersion4 {
		f.writeUint(flags)
	} else {
		f.writeByte(byte(flags))
	}

	// serialConsistency and defaultTimestamp are only signalled by flags on
	// proto > v2, so their fields must only be written on proto > v2 as well;
	// otherwise the bytes would not be described by any flag and would corrupt
	// the frame. (In practice proto < v3 is unreachable: readHeader rejects
	// response versions below protoVersion3.)
	if f.proto > protoVersion2 {
		if w.serialConsistency > 0 {
			f.writeConsistency(w.serialConsistency)
		}

		if w.defaultTimestamp {
			var ts int64
			if w.defaultTimestampValue != 0 {
				ts = w.defaultTimestampValue
			} else {
				ts = time.Now().UnixNano() / 1000
			}
			f.writeLong(ts)
		}
	}

	if w.keyspace != "" {
		f.writeString(w.keyspace)
	}

	if w.nowInSeconds != nil {
		// Bounds already validated at the top of this function.
		f.writeInt(int32(*w.nowInSeconds))
	}

	return f.finish()
}

type writeOptionsFrame struct{}

func (w *writeOptionsFrame) buildFrame(framer *framer, streamID int) error {
	return framer.writeOptionsFrame(streamID, w)
}

func (f *framer) writeOptionsFrame(stream int, _ *writeOptionsFrame) error {
	f.writeHeader(f.flags&^frm.FlagCompress, frm.OpOptions, stream)
	return f.finish()
}

type writeRegisterFrame struct {
	events []string
}

func (w *writeRegisterFrame) buildFrame(framer *framer, streamID int) error {
	return framer.writeRegisterFrame(streamID, w)
}

func (f *framer) writeRegisterFrame(streamID int, w *writeRegisterFrame) error {
	f.writeHeader(f.flags, frm.OpRegister, streamID)
	f.writeStringList(w.events)

	return f.finish()
}

func (f *framer) readByte() byte {
	if len(f.buf) < 1 {
		panic(fmt.Errorf("not enough bytes in buffer to read byte require 1 got: %d", len(f.buf)))
	}

	b := f.buf[0]
	f.buf = f.buf[1:]
	return b
}

func (f *framer) readInt() (n int) {
	if len(f.buf) < 4 {
		panic(fmt.Errorf("not enough bytes in buffer to read int require 4 got: %d", len(f.buf)))
	}

	n = int(int32(binary.BigEndian.Uint32(f.buf[:4])))
	f.buf = f.buf[4:]
	return
}

func (f *framer) readShort() (n uint16) {
	if len(f.buf) < 2 {
		panic(fmt.Errorf("not enough bytes in buffer to read short require 2 got: %d", len(f.buf)))
	}
	n = binary.BigEndian.Uint16(f.buf[:2])
	f.buf = f.buf[2:]
	return
}

func (f *framer) readString() (s string) {
	size := f.readShort()

	if len(f.buf) < int(size) {
		panic(fmt.Errorf("not enough bytes in buffer to read string require %d got: %d", size, len(f.buf)))
	}

	s = string(f.buf[:size])
	f.buf = f.buf[size:]
	return
}

// skipString advances past a string without allocating.
func (f *framer) skipString() {
	size := f.readShort()

	if len(f.buf) < int(size) {
		panic(fmt.Errorf("not enough bytes in buffer to skip string, requires %d got %d", size, len(f.buf)))
	}

	f.buf = f.buf[size:]
}

func (f *framer) readLongString() (s string) {
	size := f.readInt()

	if len(f.buf) < size {
		panic(fmt.Errorf("not enough bytes in buffer to read long string require %d got: %d", size, len(f.buf)))
	}

	s = string(f.buf[:size])
	f.buf = f.buf[size:]
	return
}

func (f *framer) readStringList() []string {
	size := f.readShort()

	l := make([]string, size)
	for i := 0; i < int(size); i++ {
		l[i] = f.readString()
	}

	return l
}

func (f *framer) ReadBytesInternal() ([]byte, error) {
	size := f.readInt()
	if size < 0 {
		return nil, nil
	}

	if len(f.buf) < size {
		return nil, fmt.Errorf("not enough bytes in buffer to read bytes require %d got: %d", size, len(f.buf))
	}

	l := f.buf[:size]
	f.buf = f.buf[size:]

	return l, nil
}

func (f *framer) readBytesCopy() []byte {
	size := f.readInt()
	if size < 0 {
		return nil
	}

	if len(f.buf) < size {
		panic(fmt.Errorf("not enough bytes in buffer to read bytes require %d got: %d", size, len(f.buf)))
	}

	out := make([]byte, size)
	copy(out, f.buf[:size])
	f.buf = f.buf[size:]
	return out
}

func (f *framer) readShortBytesCopy() []byte {
	size := f.readShort()
	if len(f.buf) < int(size) {
		panic(fmt.Errorf("not enough bytes in buffer to read short bytes: require %d got %d", size, len(f.buf)))
	}

	out := make([]byte, size)
	copy(out, f.buf[:size])
	f.buf = f.buf[size:]

	return out
}

func (f *framer) readInetAdressOnly() net.IP {
	if len(f.buf) < 1 {
		panic(fmt.Errorf("not enough bytes in buffer to read inet size require %d got: %d", 1, len(f.buf)))
	}

	size := f.buf[0]
	f.buf = f.buf[1:]

	if !(size == 4 || size == 16) {
		panic(fmt.Errorf("invalid IP size: %d", size))
	}

	if len(f.buf) < int(size) {
		panic(fmt.Errorf("not enough bytes in buffer to read inet require %d got: %d", size, len(f.buf)))
	}

	ip := make(net.IP, size)
	copy(ip, f.buf[:size])
	f.buf = f.buf[size:]
	return ip
}

func (f *framer) readInet() (net.IP, int) {
	return f.readInetAdressOnly(), f.readInt()
}

func (f *framer) readConsistency() Consistency {
	return Consistency(f.readShort())
}

func (f *framer) readBytesMap() map[string][]byte {
	size := f.readShort()
	m := make(map[string][]byte, size)

	for i := 0; i < int(size); i++ {
		m[f.readString()] = f.readBytesCopy()
	}

	return m
}

func (f *framer) readStringMultiMap() map[string][]string {
	size := f.readShort()
	m := make(map[string][]string, size)

	for i := 0; i < int(size); i++ {
		k := f.readString()
		v := f.readStringList()
		m[k] = v
	}

	return m
}

func (f *framer) writeByte(b byte) {
	f.buf = append(f.buf, b)
}

func appendBytes(p []byte, d []byte) []byte {
	if d == nil {
		return appendIntNeg1(p)
	}
	p = appendInt(p, int32(len(d)))
	p = append(p, d...)
	return p
}

func appendShort(p []byte, n uint16) []byte {
	return append(p,
		byte(n>>8),
		byte(n),
	)
}

func appendInt(p []byte, n int32) []byte {
	return append(p, byte(n>>24),
		byte(n>>16),
		byte(n>>8),
		byte(n))
}

func appendIntNeg1(p []byte) []byte {
	return append(p, 255, 255, 255, 255)
}

func appendUint(p []byte, n uint32) []byte {
	return append(p, byte(n>>24),
		byte(n>>16),
		byte(n>>8),
		byte(n))
}

func appendLong(p []byte, n int64) []byte {
	return append(p,
		byte(n>>56),
		byte(n>>48),
		byte(n>>40),
		byte(n>>32),
		byte(n>>24),
		byte(n>>16),
		byte(n>>8),
		byte(n),
	)
}

func (f *framer) writeCustomPayload(customPayload *map[string][]byte) {
	if len(*customPayload) > 0 {
		if f.proto < protoVersion4 {
			panic("Custom payload is not supported with version V3 or less")
		}
		f.writeBytesMap(*customPayload)
	}
}

func (f *framer) GetCustomPayload() map[string][]byte {
	return f.customPayload
}

func (f *framer) GetHeaderWarnings() []string {
	return f.header.Warnings
}

// these are protocol level binary types
func (f *framer) writeInt(n int32) {
	f.buf = appendInt(f.buf, n)
}

func (f *framer) writeIntNeg1() {
	f.buf = appendIntNeg1(f.buf)
}

func (f *framer) writeIntNeg2() {
	f.buf = append(f.buf, 255, 255, 255, 254)
}

func (f *framer) writeUint(n uint32) {
	f.buf = appendUint(f.buf, n)
}

func (f *framer) writeShort(n uint16) {
	f.buf = appendShort(f.buf, n)
}

func (f *framer) writeLong(n int64) {
	f.buf = appendLong(f.buf, n)
}

func (f *framer) writeString(s string) {
	f.writeShort(uint16(len(s)))
	f.buf = append(f.buf, s...)
}

func (f *framer) writeLongString(s string) {
	f.writeInt(int32(len(s)))
	f.buf = append(f.buf, s...)
}

func (f *framer) writeStringList(l []string) {
	f.writeShort(uint16(len(l)))
	for _, s := range l {
		f.writeString(s)
	}
}

func (f *framer) writeUnset() {
	// Protocol version 4 specifies that bind variables do not require having a
	// value when executing a statement.   Bind variables without a value are
	// called 'unset'. The 'unset' bind variable is serialized as the int
	// value '-2' without following bytes.
	f.writeIntNeg2()
}

func (f *framer) writeBytes(p []byte) {
	// TODO: handle null case correctly,
	//     [bytes]        A [int] n, followed by n bytes if n >= 0. If n < 0,
	//					  no byte should follow and the value represented is `null`.
	if p == nil {
		f.writeIntNeg1()
	} else {
		f.writeInt(int32(len(p)))
		f.buf = append(f.buf, p...)
	}
}

func (f *framer) writeShortBytes(p []byte) {
	f.writeShort(uint16(len(p)))
	f.buf = append(f.buf, p...)
}

func (f *framer) writeConsistency(cons Consistency) {
	f.writeShort(uint16(cons))
}

func (f *framer) writeStringMap(m map[string]string) {
	f.writeShort(uint16(len(m)))
	for k, v := range m {
		f.writeString(k)
		f.writeString(v)
	}
}

func (f *framer) writeStringMultiMap(m map[string][]string) {
	f.writeShort(uint16(len(m)))
	for k, v := range m {
		f.writeString(k)
		f.writeStringList(v)
	}
}

func (f *framer) writeBytesMap(m map[string][]byte) {
	f.writeShort(uint16(len(m)))
	for k, v := range m {
		f.writeString(k)
		f.writeBytes(v)
	}
}

// prepareModernLayout rewrites the framer's buffer from a bare CQL frame into the
// v5 wire format: one transport segment if the frame fits in one, otherwise a
// chain of non-self-contained segments (see segment.go).
//
// Segment headers, payloads and CRCs are encoded straight into f.wireBuf, sized
// up front so appending never has to grow it, and nothing per-segment is
// allocated on the way. f.buf and f.wireBuf are then swapped, which both hands
// the caller the wire bytes in f.buf and keeps the raw-frame buffer alive as
// f.wireBuf for the next request on this framer.
func (f *framer) prepareModernLayout() error {
	// Ensure protocol version is V5 or higher
	if f.proto < protoVersion5 {
		return fmt.Errorf("gocql: modern layout is not supported with protocol version %d (requires v5+)", f.proto)
	}

	// Segment the frame via a local cursor rather than mutating f.buf as we go,
	// and only swap the buffers once the whole frame has been segmented
	// successfully, so that an error partway through leaves f.buf byte-for-byte
	// intact.
	src := f.buf
	wire := f.growWireBuf(segmentedFrameSize(len(src), f.compressor != nil))

	var err error
	selfContained := true

	// Process the buffer in chunks if it exceeds the max payload size
	for len(src) > maxSegmentPayloadSize {
		wire, err = f.appendSegment(wire, src[:maxSegmentPayloadSize], false)
		if err != nil {
			return err
		}

		src = src[maxSegmentPayloadSize:]
		selfContained = false
	}

	// Process the remaining buffer
	if wire, err = f.appendSegment(wire, src, selfContained); err != nil {
		return err
	}

	f.wireBuf, f.buf = f.buf, wire

	return nil
}

// appendSegment encodes payload as one transport segment appended to dst, in the
// layout matching the framer's compressor.
func (f *framer) appendSegment(dst, payload []byte, isSelfContained bool) ([]byte, error) {
	if f.compressor != nil {
		return appendCompressedSegment(dst, payload, isSelfContained, f.compressor)
	}
	return appendUncompressedSegment(dst, payload, isSelfContained)
}

// growWireBuf returns f.wireBuf emptied and with room for at least n bytes,
// reallocating only when what it already holds is too small.
func (f *framer) growWireBuf(n int) []byte {
	if cap(f.wireBuf) < n {
		f.wireBuf = make([]byte, 0, n)
	}
	return f.wireBuf[:0]
}

// segmentedFrameSize returns how many bytes a rawLen-byte CQL frame occupies once
// segmented, so the wire buffer can be sized before anything is encoded into it.
// For the compressed layout this is an upper bound rather than the exact size:
// compressed payloads are usually smaller, but a compressor may also return more
// bytes than it was given, so room for one segment's worth of expansion is added.
func segmentedFrameSize(rawLen int, compressed bool) int {
	const (
		// 3-byte header + CRC24 + payload CRC32.
		uncompressedSegmentOverhead = 3 + crc24Size + crc32Size
		// 5-byte header + CRC24 + payload CRC32.
		compressedSegmentOverhead = 5 + crc24Size + crc32Size
		// Room for a maximum-size payload growing under compression, matching
		// lz4's block bound (len + len/255 + 16). A compressor that expands more
		// than this is still handled correctly, it only makes the wire buffer grow
		// once.
		compressionSlack = maxSegmentPayloadSize/255 + 16
	)

	segments := rawLen/maxSegmentPayloadSize + 1
	if compressed {
		return rawLen + segments*compressedSegmentOverhead + compressionSlack
	}
	return rawLen + segments*uncompressedSegmentOverhead
}
