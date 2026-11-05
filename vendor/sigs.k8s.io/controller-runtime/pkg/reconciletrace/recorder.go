/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciletrace

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

// Recorder receives the records of a trace. Record is called from the controller's workers and event handlers, so
// it must be safe for concurrent use and must not block for long: it runs on the reconcile path.
type Recorder interface {
	Record(r Record)
}

// RecorderFunc adapts a function to a Recorder.
type RecorderFunc func(r Record)

// Record implements Recorder.
func (f RecorderFunc) Record(r Record) {
	f(r)
}

// MultiRecorder records to every one of its recorders, in order.
type MultiRecorder []Recorder

// Record implements Recorder.
func (m MultiRecorder) Record(r Record) {
	for _, rec := range m {
		rec.Record(r)
	}
}

// MemoryRecorder keeps the records in memory.
type MemoryRecorder struct {
	mu      sync.Mutex
	records []Record
}

// NewMemoryRecorder creates an empty MemoryRecorder.
func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{}
}

// Record implements Recorder.
func (m *MemoryRecorder) Record(r Record) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records = append(m.records, r)
}

// Records returns a copy of the records, ordered by ID.
func (m *MemoryRecorder) Records() []Record {
	m.mu.Lock()
	records := slices.Clone(m.records)
	m.mu.Unlock()

	slices.SortFunc(records, func(a, b Record) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return records
}

// JSONLRecorder writes the records as JSON lines. Records arrive concurrently, so lines are not strictly ordered by
// ID; ReadJSONL sorts them.
type JSONLRecorder struct {
	mu  sync.Mutex
	w   *bufio.Writer
	enc *json.Encoder
	err error
}

// NewJSONLRecorder creates a JSONLRecorder writing to w. Call Flush before reading what it wrote.
func NewJSONLRecorder(w io.Writer) *JSONLRecorder {
	bw := bufio.NewWriter(w)

	return &JSONLRecorder{
		w:   bw,
		enc: json.NewEncoder(bw),
	}
}

// Record implements Recorder. The first error is kept and returned by Flush; later records are dropped.
func (j *JSONLRecorder) Record(r Record) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.err != nil {
		return
	}

	j.err = j.enc.Encode(r)
}

// Flush writes out the buffered records.
func (j *JSONLRecorder) Flush() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	return errors.Join(j.err, j.w.Flush())
}

// ReadJSONL reads the records written by a JSONLRecorder, ordered by ID.
func ReadJSONL(r io.Reader) ([]Record, error) {
	var records []Record

	dec := json.NewDecoder(r)
	for {
		var rec Record
		err := dec.Decode(&rec)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("can't decode record %d: %w", len(records)+1, err)
		}
		records = append(records, rec)
	}

	slices.SortFunc(records, func(a, b Record) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return records, nil
}
