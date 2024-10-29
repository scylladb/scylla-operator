// Copyright (c) 2024 ScyllaDB.

package cql

import (
	"bytes"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
)

func TestFrameParserReadShort(t *testing.T) {
	tt := []struct {
		name           string
		buffer         []byte
		expectedShort  uint16
		expectedBuffer []byte
	}{
		{
			name: "consumes two bytes and returns short number",
			buffer: []byte{
				0x01, 0x00,
				0x02, 0x00,
				0x03, 0x00,
				0x04, 0x00,
			},
			expectedShort: 256,
			expectedBuffer: []byte{
				0x02, 0x00,
				0x03, 0x00,
				0x04, 0x00,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.buffer)
			fp := NewFrameParser(buf)
			gotShort := fp.ReadShort()
			if gotShort != tc.expectedShort {
				t.Errorf("got %v short, expected %v", gotShort, tc.expectedShort)
			}
			if !equality.Semantic.DeepEqual(buf.Bytes(), tc.expectedBuffer) {
				t.Errorf("got %v buffer, expected %v", buf, tc.expectedBuffer)
			}
		})
	}
}

func TestFrameParserSkipHeader(t *testing.T) {
	tt := []struct {
		name           string
		buffer         []byte
		expectedBuffer []byte
	}{
		{
			name:           "consumes first 9 bytes",
			buffer:         []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x10},
			expectedBuffer: []byte{0x10},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.buffer)
			fp := NewFrameParser(buf)
			fp.SkipHeader()
			if !equality.Semantic.DeepEqual(buf.Bytes(), tc.expectedBuffer) {
				t.Errorf("got %v buffer, expected %v", buf, tc.expectedBuffer)
			}
		})
	}
}

func TestFrameParserReadString(t *testing.T) {
	tt := []struct {
		name           string
		buffer         []byte
		expectedString string
		expectedBuffer []byte
	}{
		{
			name: "consumes bytes from buffer by reading string length (uint16) and content",
			buffer: []byte{
				0x00, 0x05, // 5 - string length
				0x68, 0x65, 0x6c, 0x6c, 0x6f, // hello
			},
			expectedString: "hello",
			expectedBuffer: []byte{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.buffer)
			fp := NewFrameParser(buf)
			gotString := fp.ReadString()
			if gotString != tc.expectedString {
				t.Errorf("got %v string, expected %v", gotString, tc.expectedString)
			}
			if !equality.Semantic.DeepEqual(buf.Bytes(), tc.expectedBuffer) {
				t.Errorf("got %v buffer, expected %v", buf, tc.expectedBuffer)
			}
		})
	}
}

func TestFrameParserReadStringList(t *testing.T) {
	tt := []struct {
		name            string
		buffer          []byte
		expectedStrings []string
		expectedBuffer  []byte
	}{
		{
			name: "consumes bytes from buffer by reading string list length and strings",
			buffer: []byte{
				0x00, 0x02, // 2 - slice length
				0x00, 0x05, // 5 - string length
				0x68, 0x65, 0x6c, 0x6c, 0x6f, // hello
				0x00, 0x05, // 5 - string length
				0x77, 0x6f, 0x72, 0x6c, 0x64, // world
			},
			expectedStrings: []string{"hello", "world"},
			expectedBuffer:  []byte{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.buffer)
			fp := NewFrameParser(buf)
			gotString := fp.ReadStringList()
			if !equality.Semantic.DeepEqual(gotString, tc.expectedStrings) {
				t.Errorf("got %v strings, expected %v", gotString, tc.expectedStrings)
			}
			if !equality.Semantic.DeepEqual(buf.Bytes(), tc.expectedBuffer) {
				t.Errorf("got %v buffer, expected %v", buf, tc.expectedBuffer)
			}
		})
	}
}

func TestFrameParserReadStringMultiMap(t *testing.T) {
	tt := []struct {
		name             string
		buffer           []byte
		expectedMultiMap map[string][]string
		expectedBuffer   []byte
	}{
		{
			name: "consumes bytes from buffer by reading string list length and strings",
			buffer: []byte{
				0x00, 0x01, // 1 - map elements
				0x00, 0x03, // 3 - key length
				0x66, 0x6f, 0x6f, // foo
				0x00, 0x02, // 2 - slice length
				0x00, 0x05, // 5 - string length
				0x68, 0x65, 0x6c, 0x6c, 0x6f, // hello
				0x00, 0x05, // 5 - string length
				0x77, 0x6f, 0x72, 0x6c, 0x64, // world
			},
			expectedMultiMap: map[string][]string{
				"foo": {"hello", "world"},
			},
			expectedBuffer: []byte{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.buffer)
			fp := NewFrameParser(buf)
			gotMultiMap := fp.ReadStringMultiMap()
			if !equality.Semantic.DeepEqual(gotMultiMap, tc.expectedMultiMap) {
				t.Errorf("got %v multimap, expected %v", gotMultiMap, tc.expectedMultiMap)
			}
			if !equality.Semantic.DeepEqual(buf.Bytes(), tc.expectedBuffer) {
				t.Errorf("got %v buffer, expected %v", buf, tc.expectedBuffer)
			}
		})
	}
}
