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

package frame

import (
	"fmt"
	"net"
)

const (
	protoDirectionMask = 0x80
	protoVersionMask   = 0x7F
	protoVersion1      = 0x01
	protoVersion2      = 0x02
	protoVersion3      = 0x03
	protoVersion4      = 0x04
	protoVersion5      = 0x05

	maxFrameSize = 256 * 1024 * 1024
)

const (
	// result kind
	ResultKindVoid          = 1
	ResultKindRows          = 2
	ResultKindKeyspace      = 3
	ResultKindPrepared      = 4
	ResultKindSchemaChanged = 5

	// rows flags
	FlagGlobalTableSpec int = 0x01
	FlagHasMorePages    int = 0x02
	FlagNoMetaData      int = 0x04

	// query flags
	FlagValues                byte = 0x01
	FlagSkipMetaData          byte = 0x02
	FlagPageSize              byte = 0x04
	FlagWithPagingState       byte = 0x08
	FlagWithSerialConsistency byte = 0x10
	FlagDefaultTimestamp      byte = 0x20
	FlagWithNameValues        byte = 0x40
	FlagWithKeyspace          byte = 0x80

	// prepare flags
	FlagWithPreparedKeyspace uint32 = 0x01

	// header flags
	FlagCompress      byte = 0x01
	FlagTracing       byte = 0x02
	FlagCustomPayload byte = 0x04
	FlagWarning       byte = 0x08
	FlagBetaProtocol  byte = 0x10
)

type ProtoVersion byte

func (p ProtoVersion) Request() bool {
	return p&protoDirectionMask == 0x00
}

func (p ProtoVersion) Response() bool {
	return p&protoDirectionMask == 0x80
}

func (p ProtoVersion) Version() byte {
	return byte(p) & protoVersionMask
}

func (p ProtoVersion) String() string {
	dir := "REQ"
	if p.Response() {
		dir = "RESP"
	}

	return fmt.Sprintf("[version=%d direction=%s]", p.Version(), dir)
}

type Op byte

const (
	// header ops
	OpError         Op = 0x00
	OpStartup       Op = 0x01
	OpReady         Op = 0x02
	OpAuthenticate  Op = 0x03
	OpOptions       Op = 0x05
	OpSupported     Op = 0x06
	OpQuery         Op = 0x07
	OpResult        Op = 0x08
	OpPrepare       Op = 0x09
	OpExecute       Op = 0x0A
	OpRegister      Op = 0x0B
	OpEvent         Op = 0x0C
	OpBatch         Op = 0x0D
	OpAuthChallenge Op = 0x0E
	OpAuthResponse  Op = 0x0F
	OpAuthSuccess   Op = 0x10
)

func (f Op) String() string {
	switch f {
	case OpError:
		return "ERROR"
	case OpStartup:
		return "STARTUP"
	case OpReady:
		return "READY"
	case OpAuthenticate:
		return "AUTHENTICATE"
	case OpOptions:
		return "OPTIONS"
	case OpSupported:
		return "SUPPORTED"
	case OpQuery:
		return "QUERY"
	case OpResult:
		return "RESULT"
	case OpPrepare:
		return "PREPARE"
	case OpExecute:
		return "EXECUTE"
	case OpRegister:
		return "REGISTER"
	case OpEvent:
		return "EVENT"
	case OpBatch:
		return "BATCH"
	case OpAuthChallenge:
		return "AUTH_CHALLENGE"
	case OpAuthResponse:
		return "AUTH_RESPONSE"
	case OpAuthSuccess:
		return "AUTH_SUCCESS"
	default:
		return fmt.Sprintf("UNKNOWN_OP_%d", f)
	}
}

type FrameHeader struct {
	Warnings []string
	Stream   int
	Length   int
	Version  ProtoVersion
	Flags    byte
	Op       Op
}

func (f FrameHeader) String() string {
	return fmt.Sprintf("[header version=%s flags=0x%x stream=%d op=%s length=%d]", f.Version, f.Flags, f.Stream, f.Op, f.Length)
}

func (f FrameHeader) Header() FrameHeader {
	return f
}

type Frame interface {
	Header() FrameHeader
}

type ReadyFrame struct {
	FrameHeader
}

type SupportedFrame struct {
	Supported map[string][]string
	FrameHeader
}

type SchemaChangeKeyspace struct {
	Change   string
	Keyspace string
	FrameHeader
}

func (f SchemaChangeKeyspace) String() string {
	return fmt.Sprintf("[event schema_change_keyspace change=%q keyspace=%q]", f.Change, f.Keyspace)
}

type SchemaChangeTable struct {
	Change   string
	Keyspace string
	Object   string
	FrameHeader
}

func (f SchemaChangeTable) String() string {
	return fmt.Sprintf("[event schema_change change=%q keyspace=%q object=%q]", f.Change, f.Keyspace, f.Object)
}

type SchemaChangeType struct {
	Change   string
	Keyspace string
	Object   string
	FrameHeader
}

type SchemaChangeFunction struct {
	Change   string
	Keyspace string
	Name     string
	Args     []string
	FrameHeader
}

type SchemaChangeAggregate struct {
	Change   string
	Keyspace string
	Name     string
	Args     []string
	FrameHeader
}

type ClientRoutesChanged struct {
	ChangeType    string
	ConnectionIDs []string
	HostIDs       []string
	FrameHeader
}

type AuthenticateFrame struct {
	Class string
	FrameHeader
}

func (a *AuthenticateFrame) String() string {
	return fmt.Sprintf("[authenticate class=%q]", a.Class)
}

type AuthSuccessFrame struct {
	Data []byte
	FrameHeader
}

func (a *AuthSuccessFrame) String() string {
	return fmt.Sprintf("[auth_success data=%q]", a.Data)
}

type AuthChallengeFrame struct {
	Data []byte
	FrameHeader
}

func (a *AuthChallengeFrame) String() string {
	return fmt.Sprintf("[auth_challenge data=%q]", a.Data)
}

type StatusChangeEventFrame struct {
	Change string
	Host   net.IP
	FrameHeader
	Port int
}

func (t StatusChangeEventFrame) String() string {
	return fmt.Sprintf("[status_change change=%s host=%v port=%v]", t.Change, t.Host, t.Port)
}

// essentially the same as statusChange
type TopologyChangeEventFrame struct {
	Change string
	Host   net.IP
	FrameHeader
	Port int
}

func (t TopologyChangeEventFrame) String() string {
	return fmt.Sprintf("[topology_change change=%s host=%v port=%v]", t.Change, t.Host, t.Port)
}

type ErrorFrame struct {
	Message string
	FrameHeader
	Code int
}

func (e ErrorFrame) GetCode() int {
	return e.Code
}

func (e ErrorFrame) GetMessage() string {
	return e.Message
}

func (e ErrorFrame) Error() string {
	return e.GetMessage()
}

func (e ErrorFrame) String() string {
	return fmt.Sprintf("[error code=%x message=%q]", e.Code, e.Message)
}
