// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"strings"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// newUnstartedCache builds a controller-runtime cache that never talks to an API server: the REST mappings are
// static, and an informer requested from an unstarted cache is only registered, not started.
func newUnstartedCache(t *testing.T, options cache.Options) cache.Cache {
	t.Helper()

	mapper := meta.NewDefaultRESTMapper(nil)
	for _, gvk := range []schema.GroupVersionKind{
		corev1.SchemeGroupVersion.WithKind("Pod"),
		corev1.SchemeGroupVersion.WithKind("Secret"),
	} {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}

	options.Scheme = scheme.Scheme
	options.Mapper = mapper
	c, err := cache.New(&rest.Config{Host: "https://cache.invalid"}, options)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func TestInformerFor(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := newUnstartedCache(t, cache.Options{})

	pods, err := InformerFor(ctx, c, &corev1.Pod{}, corev1listers.NewPodLister)
	if err != nil {
		t.Fatal(err)
	}

	// The lister reads the informer's own indexer.
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}}
	err = pods.Informer().GetIndexer().Add(pod)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pods.Lister().Pods("ns").Get("pod")
	if err != nil {
		t.Fatal(err)
	}
	if got != pod {
		t.Errorf("expected the lister to return the object from the informer's indexer, got %v", got)
	}

	// Requesting a kind again hands out the informer the cache already has: one informer per kind in the process,
	// however many controllers take it.
	podsAgain, err := InformerFor(ctx, c, &corev1.Pod{}, corev1listers.NewPodLister)
	if err != nil {
		t.Fatal(err)
	}
	if podsAgain.Informer() != pods.Informer() {
		t.Error("expected the second request for Pods to return the same informer")
	}

	secrets, err := InformerFor(ctx, c, &corev1.Secret{}, corev1listers.NewSecretLister)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.Informer() == pods.Informer() {
		t.Error("expected Secrets and Pods to have informers of their own")
	}
}

// A namespace-restricted cache wraps its informers in controller-runtime's own multi-namespace type, which the typed
// listers can't be built over. The bridge refuses it instead of failing later at the first read.
func TestInformerFor_namespacedCache(t *testing.T) {
	t.Parallel()

	c := newUnstartedCache(t, cache.Options{
		DefaultNamespaces: map[string]cache.Config{"ns": {}},
	})

	_, err := InformerFor(t.Context(), c, &corev1.Pod{}, corev1listers.NewPodLister)
	if err == nil || !strings.Contains(err.Error(), "not a client-go SharedIndexInformer") {
		t.Fatalf("expected the bridge to refuse the multi-namespace informer, got %v", err)
	}
}

func TestInformerFor_unknownKind(t *testing.T) {
	t.Parallel()

	c := newUnstartedCache(t, cache.Options{})

	// ConfigMap is in the scheme but has no REST mapping in this cache, like a kind whose CRD is not installed.
	_, err := InformerFor(t.Context(), c, &corev1.ConfigMap{}, corev1listers.NewConfigMapLister)
	if err == nil {
		t.Fatal("expected an error for a kind the cache can't map")
	}
}
