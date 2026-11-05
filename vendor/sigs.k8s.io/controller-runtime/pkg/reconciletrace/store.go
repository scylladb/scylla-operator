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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// RecordTypeTruncated marks the end of a trace that hit its size limit; later records were dropped.
const RecordTypeTruncated RecordType = "Truncated"

// clusterScopedFileName holds the records of requests without a namespace. Namespace names can't start with an
// underscore, so it can't clash with a namespace's file.
const clusterScopedFileName = "_cluster"

var namespaceNameRegexp = regexp.MustCompile(`^(_cluster|[a-z0-9]([-a-z0-9]*[a-z0-9])?)$`)

// NamespacedFileStore is a Recorder writing the records of each namespace to a JSON lines file of its own, named
// after the namespace of the record's request, and serving them over HTTP. It is meant for tests running a controller
// in a cluster, so that each test can fetch the trace of its namespaces.
//
// Records attributed to no request are dropped. A namespace's file stops growing at the size limit; a final
// RecordTypeTruncated record marks it.
type NamespacedFileStore struct {
	dir      string
	maxBytes int64

	mu    sync.Mutex
	files map[string]*namespaceFile
}

type namespaceFile struct {
	f       *os.File
	w       *bufio.Writer
	written int64
	full    bool
	err     error
}

var _ Recorder = &NamespacedFileStore{}

// NewNamespacedFileStore creates the store in dir. maxBytesPerNamespace limits each namespace's file; 0 means no
// limit.
func NewNamespacedFileStore(dir string, maxBytesPerNamespace int64) (*NamespacedFileStore, error) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("can't create directory %q: %w", dir, err)
	}

	return &NamespacedFileStore{
		dir:      dir,
		maxBytes: maxBytesPerNamespace,
		files:    map[string]*namespaceFile{},
	}, nil
}

// namespaceOfRequest returns the namespace of a request formatted as namespace/name.
func namespaceOfRequest(request string) string {
	ns, _, found := strings.Cut(request, "/")
	if !found || ns == "" {
		return clusterScopedFileName
	}

	return ns
}

// Record implements Recorder.
func (s *NamespacedFileStore) Record(r Record) {
	if r.Request == "" {
		return
	}
	ns := namespaceOfRequest(r.Request)

	line, err := json.Marshal(r)
	if err != nil {
		return
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	nf, err := s.fileLocked(ns)
	if err != nil || nf.full || nf.err != nil {
		return
	}

	if s.maxBytes > 0 && nf.written+int64(len(line)) > s.maxBytes {
		nf.full = true
		marker, _ := json.Marshal(Record{ID: nextID(), Time: time.Now(), Type: RecordTypeTruncated, Request: r.Request})
		line = append(marker, '\n')
	}

	n, err := nf.w.Write(line)
	nf.written += int64(n)
	nf.err = err
}

func (s *NamespacedFileStore) fileLocked(ns string) (*namespaceFile, error) {
	nf, ok := s.files[ns]
	if ok {
		return nf, nil
	}

	f, err := os.OpenFile(filepath.Join(s.dir, ns+".jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	nf = &namespaceFile{f: f, w: bufio.NewWriter(f)}
	s.files[ns] = nf

	return nf, nil
}

// snapshot flushes the namespace's buffer and returns how many bytes of its file hold complete records.
func (s *NamespacedFileStore) snapshot(ns string) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nf, ok := s.files[ns]
	if !ok {
		return 0, false, nil
	}

	err := nf.w.Flush()
	if err != nil {
		return 0, true, err
	}

	return nf.written, true, nf.err
}

// Namespaces returns the namespaces with a trace.
func (s *NamespacedFileStore) Namespaces() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	namespaces := make([]string, 0, len(s.files))
	for ns := range s.files {
		namespaces = append(namespaces, ns)
	}
	slices.Sort(namespaces)

	return namespaces
}

// WriteTo writes the namespace's trace, as of now, to w. It returns false if the namespace has no trace.
func (s *NamespacedFileStore) WriteTo(ns string, w io.Writer) (bool, error) {
	size, found, err := s.snapshot(ns)
	if err != nil || !found {
		return found, err
	}

	f, err := os.Open(filepath.Join(s.dir, ns+".jsonl"))
	if err != nil {
		return true, err
	}
	defer f.Close()

	// Records written after the snapshot may be partially flushed; serve only the complete ones.
	_, err = io.Copy(w, io.NewSectionReader(f, 0, size))
	return true, err
}

// Close flushes and closes every file.
func (s *NamespacedFileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, nf := range s.files {
		errs = append(errs, nf.w.Flush(), nf.f.Close())
	}
	s.files = map[string]*namespaceFile{}

	return errors.Join(errs...)
}

// Handler serves the traces: GET {prefix}/ lists the namespaces with a trace as a JSON array and
// GET {prefix}/{namespace} returns a namespace's trace as JSON lines. The records of cluster-scoped requests are
// under the namespace "_cluster".
func (s *NamespacedFileStore) Handler(prefix string) http.Handler {
	prefix = strings.TrimSuffix(prefix, "/")

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix+"/{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Namespaces())
	})
	mux.HandleFunc("GET "+prefix+"/{namespace}", func(w http.ResponseWriter, r *http.Request) {
		ns := r.PathValue("namespace")
		if !namespaceNameRegexp.MatchString(ns) {
			http.Error(w, "invalid namespace", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		found, err := s.WriteTo(ns, w)
		switch {
		case !found:
			http.Error(w, "no trace", http.StatusNotFound)
		case err != nil:
			// The body may already be partially written; the error can only be reported to the server's log.
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return mux
}
