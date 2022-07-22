package resourceapply

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
)

func NewServerAndClient(t *testing.T, handler http.Handler) (*httptest.Server, *mermaidclient.Client) {
	server := httptest.NewServer(handler)
	client, err := mermaidclient.NewClient(fmt.Sprintf("http://%s", server.Listener.Addr().String()), &http.Transport{})
	if err != nil {
		t.Fatalf("creating client failed: %v", err)
	}
	return server, &client
}

func TestApplyCluster(t *testing.T) {
	t.Parallel()

	testUUID := uuid.NewFromUint64(128, 153)
	newCluster := func() *mermaidclient.Cluster {
		return &mermaidclient.Cluster{
			AuthToken: "token",
			Host:      "host",
			Name:      "cluster",
		}
	}
	registeredCluster1 := func() *mermaidclient.Cluster {
		return &mermaidclient.Cluster{
			AuthToken: "token",
			Host:      "host",
			Name:      "cluster",
			ID:        testUUID.String(),
		}
	}
	registeredCluster2 := func() *mermaidclient.Cluster {
		return &mermaidclient.Cluster{
			AuthToken: "different-token",
			Host:      "host",
			Name:      "cluster",
			ID:        testUUID.String(),
		}
	}
	tt := []struct {
		name            string
		requiredCluster *mermaidclient.Cluster
		clusters        []*mermaidclient.Cluster
		handler         http.Handler
		expectedChange  bool
		expectedError   error
		expectedCluster *mermaidclient.Cluster
	}{
		{
			name:            "create cluster",
			requiredCluster: newCluster(),
			clusters:        nil,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", testUUID.String())
				w.WriteHeader(http.StatusCreated)
			}),
			expectedChange:  true,
			expectedError:   nil,
			expectedCluster: registeredCluster1(),
		},
		{
			name:            "updated cluster",
			requiredCluster: newCluster(),
			clusters:        []*mermaidclient.Cluster{registeredCluster2()},
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				render.Respond(w, r, registeredCluster1())
			}),
			expectedChange:  true,
			expectedError:   nil,
			expectedCluster: registeredCluster1(),
		},
		{
			name:            "cluster will not change",
			requiredCluster: newCluster(),
			clusters:        []*mermaidclient.Cluster{registeredCluster1()},
			handler:         nil,
			expectedChange:  false,
			expectedError:   nil,
			expectedCluster: registeredCluster1(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, client := NewServerAndClient(t, tc.handler)
			defer server.Close()

			c, changed, err := ApplyCluster(context.Background(), client, tc.requiredCluster, tc.clusters)
			if err != tc.expectedError {
				t.Fatalf("expected: %v, got: %v", tc.expectedError, err)
			}
			if changed != tc.expectedChange {
				t.Fatalf("expected: %v, got: %v", tc.expectedChange, changed)
			}
			if diff := cmp.Diff(c, tc.expectedCluster); diff != "" {
				t.Fatalf("Expected cluster is different from synced cluster exp: %v, sync: %v", tc.expectedCluster, c)
			}
		})
	}
}
