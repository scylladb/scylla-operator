// Copyright (C) 2017 ScyllaDB

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
)

type ScyllaFake struct {
	snapshotsTaken  map[string]*strset.Set
	drainRequests   int
	operationalMode scyllaclient.OperationalMode
	keyspaces       []string
	mut             sync.Mutex

	server *httptest.Server
}

func NewScyllaFake(operationalMode scyllaclient.OperationalMode, keyspaces []string) *ScyllaFake {
	return &ScyllaFake{
		operationalMode: operationalMode,
		snapshotsTaken:  make(map[string]*strset.Set),
		keyspaces:       keyspaces,
	}
}

func (s *ScyllaFake) Start() string {
	s.server = httptest.NewServer(http.HandlerFunc(s.handler))
	return s.server.Listener.Addr().String()
}

func (s *ScyllaFake) handler(w http.ResponseWriter, r *http.Request) {
	s.mut.Lock()
	defer s.mut.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/storage_service/snapshots" && r.Method == http.MethodGet:
		type snapshot struct {
			Key string `json:"key"`
		}
		resp := []snapshot{}
		for tag := range s.snapshotsTaken {
			resp = append(resp, snapshot{Key: tag})
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case r.URL.Path == "/storage_service/snapshots" && r.Method == http.MethodPost:
		kn := r.URL.Query().Get("kn")
		tag := r.URL.Query().Get("tag")
		if _, ok := s.snapshotsTaken[tag]; !ok {
			s.snapshotsTaken[tag] = strset.New()
		}
		s.snapshotsTaken[tag].Add(kn)
	case r.URL.Path == "/storage_service/snapshots" && r.Method == http.MethodDelete:
		tag := r.URL.Query().Get("tag")
		delete(s.snapshotsTaken, tag)
	case r.URL.Path == "/storage_service/keyspaces" && r.Method == http.MethodGet:
		if err := json.NewEncoder(w).Encode(s.keyspaces); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case r.URL.Path == "/storage_service/drain" && r.Method == http.MethodPost:
		s.drainRequests++
	case r.URL.Path == "/storage_service/operation_mode" && r.Method == http.MethodGet:
		fmt.Fprintf(w, "%q", s.operationalMode)
	}
}

func (s *ScyllaFake) SetOperationalMode(mode scyllaclient.OperationalMode) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.operationalMode = mode
}

func (s *ScyllaFake) DrainRequests() int {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.drainRequests
}

func (s *ScyllaFake) KeyspaceSnapshots() []string {
	s.mut.Lock()
	defer s.mut.Unlock()
	var keyspaceSnapshots = []string{}
	for _, snapshots := range s.snapshotsTaken {
		keyspaceSnapshots = append(keyspaceSnapshots, snapshots.List()...)
	}
	return keyspaceSnapshots
}

func (s *ScyllaFake) Close() {
	s.server.Close()
}
