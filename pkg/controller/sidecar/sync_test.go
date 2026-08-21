// Copyright (c) 2026 ScyllaDB.

package sidecar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
)

func TestNodeIsScyllaDBClusterMember(t *testing.T) {
	t.Parallel()

	const hostID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	tt := []struct {
		name           string
		handler        http.HandlerFunc
		expectedMember bool
		expectedKnown  bool
		expectedErr    bool
	}{
		{
			name: "node owning normal tokens is a member",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/operation_mode":
					w.Write([]byte(`"NORMAL"`))
				case "/storage_service/host_id":
					w.Write([]byte(`[{"key":"10.0.0.1","value":"` + hostID + `"}]`))
				case "/storage_service/tokens/10.0.0.1":
					w.Write([]byte(`["-1","0","1"]`))
				default:
					t.Errorf("unexpected request to %q", r.URL.Path)
				}
			},
			expectedMember: true,
			expectedKnown:  true,
			expectedErr:    false,
		},
		{
			name: "node without normal tokens is known but not a member",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/operation_mode":
					w.Write([]byte(`"NORMAL"`))
				case "/storage_service/host_id":
					w.Write([]byte(`[{"key":"10.0.0.1","value":"` + hostID + `"}]`))
				case "/storage_service/tokens/10.0.0.1":
					w.Write([]byte(`[]`))
				default:
					t.Errorf("unexpected request to %q", r.URL.Path)
				}
			},
			expectedMember: false,
			expectedKnown:  true,
			expectedErr:    false,
		},
		{
			name: "node absent from the host ID map is undeterminable when node is not in normal operation mode",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/host_id":
					w.Write([]byte(`[{"key":"10.0.0.2","value":"different-host-id"}]`))
				case "/storage_service/operation_mode":
					w.Write([]byte(`"JOINING"`))
				default:
					t.Errorf("unexpected request to %q", r.URL.Path)
				}
			},
			expectedMember: false,
			expectedKnown:  false,
			expectedErr:    false,
		},
		{
			name: "node absent from the host ID map while normal is an error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/operation_mode":
					w.Write([]byte(`"NORMAL"`))
				case "/storage_service/host_id":
					w.Write([]byte(`[{"key":"10.0.0.2","value":"ffffffff-ffff-ffff-ffff-ffffffffffff"}]`))
				default:
					// The node tokens endpoint must not be reached when the expected HostID is absent.
					t.Errorf("unexpected request to %q", r.URL.Path)
				}
			},
			expectedMember: false,
			expectedKnown:  false,
			expectedErr:    true,
		},
		{
			name: "operation mode error is propagated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedMember: false,
			expectedKnown:  false,
			expectedErr:    true,
		},
		{
			name: "host ID map error is propagated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/operation_mode":
					w.Write([]byte(`"NORMAL"`))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			},
			expectedMember: false,
			expectedKnown:  false,
			expectedErr:    true,
		},
		{
			name: "node tokens error is propagated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/storage_service/operation_mode":
					w.Write([]byte(`"NORMAL"`))
				case "/storage_service/host_id":
					w.Write([]byte(`[{"key":"10.0.0.1","value":"` + hostID + `"}]`))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			},
			expectedMember: false,
			expectedKnown:  false,
			expectedErr:    true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tc.handler)
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

			isMember, isKnown, err := nodeIsScyllaDBClusterMember(context.Background(), client, u.Hostname(), hostID)
			if (err != nil) != tc.expectedErr {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}
			if isMember != tc.expectedMember {
				t.Errorf("expected isMember %v, got %v", tc.expectedMember, isMember)
			}
			if isKnown != tc.expectedKnown {
				t.Errorf("expected isKnown %v, got %v", tc.expectedKnown, isKnown)
			}
		})
	}
}
