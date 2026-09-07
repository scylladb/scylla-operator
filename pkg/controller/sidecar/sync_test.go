// Copyright (c) 2026 ScyllaDB.

package sidecar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
)

func TestGetScyllaDBClusterMembership(t *testing.T) {
	t.Parallel()

	const (
		localIP = "10.0.0.1"
		hostID  = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	)

	tt := []struct {
		name               string
		fake               fakeScyllaDBTokenMetadata
		expectedMembership scyllaDBClusterMembership
		expectedErr        bool
	}{
		{
			name: "node owning normal tokens is a member",
			fake: fakeScyllaDBTokenMetadata{
				operationMode: scyllaclient.OperationalModeNormal,
				ipToHostIDMap: map[string]string{localIP: hostID},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
			},
			expectedMembership: scyllaDBClusterMembershipMember,
		},
		{
			// A vnode decommission streams while in LEAVING. The node still owns its tokens and its peers still observe it.
			name: "leaving node owning normal tokens is a member",
			fake: fakeScyllaDBTokenMetadata{
				operationMode: scyllaclient.OperationalModeLeaving,
				ipToHostIDMap: map[string]string{localIP: hostID},
				nodeTokens:    map[string][]string{localIP: {"-1", "0", "1"}},
			},
			expectedMembership: scyllaDBClusterMembershipMember,
		},
		{
			// BOOTSTRAP is a mode without special handling, which falls through to the token metadata.
			name: "node in a mode without special handling is looked up in the token metadata",
			fake: fakeScyllaDBTokenMetadata{
				operationMode: scyllaclient.OperationalModeBootstrap,
				ipToHostIDMap: map[string]string{localIP: hostID},
				nodeTokens:    map[string][]string{localIP: {}},
			},
			expectedMembership: scyllaDBClusterMembershipNotMember,
		},
		{
			name: "node without normal tokens is not a member",
			fake: fakeScyllaDBTokenMetadata{
				operationMode: scyllaclient.OperationalModeNormal,
				ipToHostIDMap: map[string]string{localIP: hostID},
				nodeTokens:    map[string][]string{localIP: {}},
			},
			expectedMembership: scyllaDBClusterMembershipNotMember,
		},
		{
			// The node tokens endpoint must not be reached, it reports an empty token list for an address the cluster
			// doesn't know, which is indistinguishable from a bootstrapping node. Reaching it fails the request instead.
			name: "node absent from the host ID map is undeterminable",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:  scyllaclient.OperationalModeNormal,
				ipToHostIDMap:  map[string]string{"10.0.0.2": "ffffffff-ffff-ffff-ffff-ffffffffffff"},
				failNodeTokens: true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
		},
		{
			// The token metadata endpoints must not be reached: the gossiper is stopped and the host ID map fails.
			name: "decommissioned node is not a member without consulting the token metadata",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:     scyllaclient.OperationalModeDecommissioned,
				failIPToHostIDMap: true,
				failNodeTokens:    true,
			},
			expectedMembership: scyllaDBClusterMembershipNotMember,
		},
		{
			// Ditto, but the node still owns its tokens and is restarted eventually, so the last observation stands.
			name: "drained node is undeterminable without consulting the token metadata",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:     scyllaclient.OperationalModeDrained,
				failIPToHostIDMap: true,
				failNodeTokens:    true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
		},
		{
			// Ditto, the gossiper hasn't started yet.
			name: "starting node is undeterminable without consulting the token metadata",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:     scyllaclient.OperationalModeStarting,
				failIPToHostIDMap: true,
				failNodeTokens:    true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
		},
		{
			name: "operation mode error is propagated",
			fake: fakeScyllaDBTokenMetadata{
				failOperationMode: true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
			expectedErr:        true,
		},
		{
			name: "host ID map error is propagated",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:     scyllaclient.OperationalModeNormal,
				failIPToHostIDMap: true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
			expectedErr:        true,
		},
		{
			name: "node tokens error is propagated",
			fake: fakeScyllaDBTokenMetadata{
				operationMode:  scyllaclient.OperationalModeNormal,
				ipToHostIDMap:  map[string]string{localIP: hostID},
				failNodeTokens: true,
			},
			expectedMembership: scyllaDBClusterMembershipUnknown,
			expectedErr:        true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(newFakeScyllaDBTokenMetadataHandler(t, tc.fake))
			t.Cleanup(server.Close)

			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}

			config := scyllaclient.DefaultConfig("", u.Hostname())
			config.Scheme = "http"
			config.Port = u.Port()

			client, err := scyllaclient.NewClient(config)
			if err != nil {
				t.Fatal(err)
			}

			membership, err := getScyllaDBClusterMembership(context.Background(), client, u.Hostname(), hostID)
			if (err != nil) != tc.expectedErr {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}
			if membership != tc.expectedMembership {
				t.Errorf("expected membership %v, got %v", tc.expectedMembership, membership)
			}
		})
	}
}

// fakeScyllaDBTokenMetadata describes the ScyllaDB API responses served by newFakeScyllaDBTokenMetadataHandler.
type fakeScyllaDBTokenMetadata struct {
	// operationMode is the node's operational mode.
	operationMode scyllaclient.OperationalMode
	// failOperationMode makes the operation mode request fail.
	failOperationMode bool
	// ipToHostIDMap is the cluster's IP to host ID mapping.
	ipToHostIDMap map[string]string
	// failIPToHostIDMap makes the host ID mapping request fail.
	failIPToHostIDMap bool
	// nodeTokens maps an endpoint to the normal tokens it owns. An endpoint absent from the map is served as 404.
	nodeTokens map[string][]string
	// failNodeTokens makes the per-endpoint tokens request fail.
	failNodeTokens bool
}

// newFakeScyllaDBTokenMetadataHandler returns a handler serving the given fake ScyllaDB API responses, failing the test
// on any other request.
func newFakeScyllaDBTokenMetadataHandler(t *testing.T, fake fakeScyllaDBTokenMetadata) http.HandlerFunc {
	encode := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(v); err != nil {
			t.Errorf("can't encode response: %v", err)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/storage_service/operation_mode":
			if fake.failOperationMode {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			encode(w, string(fake.operationMode))

		case r.URL.Path == "/storage_service/host_id":
			if fake.failIPToHostIDMap {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var mapping []map[string]string
			for ip, hostID := range fake.ipToHostIDMap {
				mapping = append(mapping, map[string]string{"key": ip, "value": hostID})
			}
			encode(w, mapping)

		case strings.HasPrefix(r.URL.Path, "/storage_service/tokens/"):
			if fake.failNodeTokens {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			tokens, ok := fake.nodeTokens[strings.TrimPrefix(r.URL.Path, "/storage_service/tokens/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			encode(w, tokens)

		default:
			t.Errorf("unexpected request to %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}
}
