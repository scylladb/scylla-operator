//go:build envtest

// Copyright (c) 2026 ScyllaDB.

package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	g "github.com/onsi/ginkgo/v2"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
)

// newFakeScyllaDBClientFactory returns a ScyllaDB client factory backed by a single httptest.Server serving the given
// handler. The server is torn down on spec cleanup.
func newFakeScyllaDBClientFactory(handler http.Handler) func() (*scyllaclient.Client, error) {
	g.GinkgoHelper()

	server := httptest.NewServer(handler)
	g.DeferCleanup(server.Close)

	return func() (*scyllaclient.Client, error) {
		parsedURL, err := url.Parse(server.URL)
		if err != nil {
			return nil, fmt.Errorf("can't parse server URL: %w", err)
		}

		return scyllaclient.NewClient(&scyllaclient.Config{
			Hosts:   []string{parsedURL.Hostname()},
			Port:    parsedURL.Port(),
			Scheme:  "http",
			Timeout: 5 * time.Second,
		})
	}
}

// newSwitchableFakeScyllaDBClientFactory returns a switcher and a ScyllaDB client factory, both backed by a single
// httptest.Server. The server serves firstHandler until SwitchToPhaseTwo is called on the switcher, and secondHandler
// afterwards. The server is torn down on spec cleanup.
func newSwitchableFakeScyllaDBClientFactory(firstHandler, secondHandler http.Handler) (*switchableHandler, func() (*scyllaclient.Client, error)) {
	g.GinkgoHelper()

	handler := &switchableHandler{
		first:            firstHandler,
		second:           secondHandler,
		phaseTwoServedCh: make(chan struct{}),
	}

	return handler, newFakeScyllaDBClientFactory(handler)
}

// switchableHandler serves the first handler until SwitchToPhaseTwo is called, and the second one afterwards. It backs
// a fake ScyllaDB API whose responses have to change mid-spec.
type switchableHandler struct {
	first  http.Handler
	second http.Handler

	inPhaseTwo       atomic.Bool
	phaseTwoOnce     sync.Once
	phaseTwoServedCh chan struct{}
}

func (h *switchableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.inPhaseTwo.Load() {
		h.first.ServeHTTP(w, r)
		return
	}

	h.phaseTwoOnce.Do(func() {
		close(h.phaseTwoServedCh)
	})

	h.second.ServeHTTP(w, r)
}

// SwitchToPhaseTwo makes the handler start serving the second handler.
func (h *switchableHandler) SwitchToPhaseTwo() {
	h.inPhaseTwo.Store(true)
}

// PhaseTwoServedCh returns a channel closed when the handler first serves a request in phase two. Waiting on it before
// asserting keeps a phase two assertion from passing against a phase one observation.
func (h *switchableHandler) PhaseTwoServedCh() <-chan struct{} {
	return h.phaseTwoServedCh
}

// encodeJSON writes v as the JSON response body, logging an encoding failure to the Ginkgo output.
func encodeJSON(w http.ResponseWriter, r *http.Request, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		g.GinkgoWriter.Printf("failed to encode response for %q: %v\n", r.URL.Path, err)
	}
}
