// Copyright (c) 2024 ScyllaDB.

package cql

import (
	"bytes"
	"fmt"
)

const (
	headerLen = 9

	// OptionsFrame is a minimal OPTIONS CQL frame.
	// Ref: https://github.com/apache/cassandra/blob/f278f6774fc76465c182041e081982105c3e7dbb/doc/native_protocol_v4.spec
	OptionsFrame = `\x04\x00\x00\x00\x05\x00\x00\x00\x00`
)

type FrameParser struct {
	buf *bytes.Buffer
}

func NewFrameParser(buf *bytes.Buffer) *FrameParser {
	return &FrameParser{
		buf: buf,
	}
}

func (fp *FrameParser) SkipHeader() {
	_ = fp.readBytes(headerLen)
}

func (fp *FrameParser) readByte() byte {
	p, err := fp.buf.ReadByte()
	if err != nil {
		panic(fmt.Errorf("can't read byte from buffer: %w", err))
	}
	return p
}

func (fp *FrameParser) ReadShort() uint16 {
	return uint16(fp.readByte())<<8 | uint16(fp.readByte())
}

func (fp *FrameParser) ReadStringMultiMap() map[string][]string {
	n := fp.ReadShort()
	m := make(map[string][]string, n)
	for i := uint16(0); i < n; i++ {
		k := fp.ReadString()
		v := fp.ReadStringList()
		m[k] = v
	}
	return m
}

func (fp *FrameParser) readBytes(n int) []byte {
	p := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		p = append(p, fp.readByte())
	}

	return p
}

func (fp *FrameParser) ReadString() string {
	return string(fp.readBytes(int(fp.ReadShort())))
}

func (fp *FrameParser) ReadStringList() []string {
	n := fp.ReadShort()
	l := make([]string, 0, n)
	for i := uint16(0); i < n; i++ {
		l = append(l, fp.ReadString())
	}
	return l
}
