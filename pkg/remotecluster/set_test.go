// Copyright (c) 2026 ScyllaDB.

package remotecluster

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

// newFakeAPIServer serves the discovery documents building a cluster needs, or 401 while unauthorized is set.
func newFakeAPIServer(t *testing.T, unauthorized *atomic.Bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if unauthorized.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"Unauthorized","reason":"Unauthorized","code":401}`))
			return
		}

		switch r.URL.Path {
		case "/api":
			_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
		case "/api/v1":
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","groupVersion":"v1","resources":[
				{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod","verbs":["get","list","watch"]},
				{"name":"secrets","singularName":"secret","namespaced":true,"kind":"Secret","verbs":["get","list","watch"]}
			]}`))
		case "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
		}
	}))
}

func kubeconfigFor(server string) []byte {
	return []byte(strings.ReplaceAll(`
apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: SERVER
contexts:
- name: c
  context: {cluster: c, user: u}
current-context: c
users:
- name: u
  user: {token: t}
`, "SERVER", server))
}

func TestSet_UnreachableClusterIsNotReadyUntilReachable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var unauthorized atomic.Bool
	unauthorized.Store(true)
	server := newFakeAPIServer(t, &unauthorized)
	defer server.Close()

	s := New(ctx, cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
				Label: labels.Everything(),
			},
		},
	})
	s.retryInterval = 50 * time.Millisecond

	var onClusterCalls atomic.Int32
	err := s.OnCluster(func(ctx context.Context, name string, c cluster.Cluster) error {
		onClusterCalls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpdateCluster("rkc", kubeconfigFor(server.URL))
	if err != nil {
		t.Fatalf("expected registering an unreachable cluster to succeed, got: %v", err)
	}

	// Wait for the first build attempt to fail and be reported.
	err = apimachineryutilwait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		_, err := s.Cluster("rkc")
		return err != nil && strings.Contains(err.Error(), "credentials"), nil
	})
	if err != nil {
		_, clusterErr := s.Cluster("rkc")
		t.Fatalf("expected the cluster to be reported not ready with the API server's error, got: %v", clusterErr)
	}
	if got := onClusterCalls.Load(); got != 0 {
		t.Fatalf("expected no OnCluster calls for an unreachable cluster, got %d", got)
	}

	unauthorized.Store(false)

	err = apimachineryutilwait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		_, err := s.Cluster("rkc")
		return err == nil, nil
	})
	if err != nil {
		_, clusterErr := s.Cluster("rkc")
		t.Fatalf("expected the cluster to become ready once reachable, got: %v", clusterErr)
	}
	if got := onClusterCalls.Load(); got != 1 {
		t.Fatalf("expected 1 OnCluster call once the cluster is ready, got %d", got)
	}

	// A later OnCluster runs on the ready cluster right away.
	err = s.OnCluster(func(ctx context.Context, name string, c cluster.Cluster) error {
		onClusterCalls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := onClusterCalls.Load(); got != 2 {
		t.Fatalf("expected 2 OnCluster calls, got %d", got)
	}

	s.DeleteCluster("rkc")
	_, err = s.Cluster("rkc")
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("expected a deleted cluster to be not registered, got: %v", err)
	}
}

func TestSet_UpdateClusterWithSameConfigIsNoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var unauthorized atomic.Bool
	server := newFakeAPIServer(t, &unauthorized)
	defer server.Close()

	s := New(ctx, cache.Options{})

	var onClusterCalls atomic.Int32
	err := s.OnCluster(func(ctx context.Context, name string, c cluster.Cluster) error {
		onClusterCalls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		err = s.UpdateCluster("rkc", kubeconfigFor(server.URL))
		if err != nil {
			t.Fatal(err)
		}
	}

	err = apimachineryutilwait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		_, err := s.Cluster("rkc")
		return err == nil, nil
	})
	if err != nil {
		t.Fatal(fmt.Errorf("expected the cluster to become ready: %w", err))
	}
	if got := onClusterCalls.Load(); got != 1 {
		t.Fatalf("expected 1 OnCluster call, got %d", got)
	}

	first, _ := s.Cluster("rkc")
	err = s.UpdateCluster("rkc", kubeconfigFor(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := s.Cluster("rkc")
	if first != second {
		t.Fatal("expected the same kubeconfig to keep the cluster")
	}
}
