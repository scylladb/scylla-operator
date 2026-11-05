// Copyright (c) 2026 ScyllaDB.

package reconcilecritic

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/fake"
	fakerest "k8s.io/client-go/rest/fake"
)

func TestCollectTraces(t *testing.T) {
	t.Parallel()

	running := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "scylla-operator", Name: name, Labels: map[string]string{"app": "op"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}
	pending := running("op-pending")
	pending.Status.Phase = corev1.PodPending
	kubeClient := fake.NewClientset(running("op-leader"), running("op-follower"), pending)

	var paths []string
	proxy := &fakerest.RESTClient{
		NegotiatedSerializer: serializer.NewCodecFactory(runtime.NewScheme()).WithoutConversion(),
		Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			paths = append(paths, req.URL.Path)
			switch req.URL.Path {
			case "/namespaces/scylla-operator/pods/op-leader:8095/proxy/reconcile-traces/ns-a":
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(`{"id":1}` + "\n"))}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString("no trace"))}, nil
			}
		}),
	}

	dir := filepath.Join(t.TempDir(), TracesDirName)
	written, err := CollectTraces(context.Background(), kubeClient, proxy, CollectOptions{
		OperatorNamespace: "scylla-operator",
		OperatorSelector:  "app=op",
		Port:              8095,
	}, dir, "ns-a", TraceMeta{TestName: "a test", Failed: true, Source: "e2e"})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 2 {
		t.Errorf("expected the two running Pods to be asked, got %v", paths)
	}
	want := filepath.Join(dir, "ns-a.op-leader.jsonl")
	if len(written) != 1 || written[0] != want {
		t.Fatalf("expected only the leader's trace at %s, got %v", want, written)
	}
	b, err := os.ReadFile(want)
	if err != nil || string(b) != `{"id":1}`+"\n" {
		t.Errorf("unexpected trace: %q, %v", b, err)
	}

	meta, err := ReadTraceMeta(want)
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || meta.TestName != "a test" || !meta.Failed || meta.Namespace != "ns-a" {
		t.Errorf("unexpected meta: %+v", meta)
	}
}

func TestCollectTracesWritesNothingWithoutTraces(t *testing.T) {
	t.Parallel()

	kubeClient := fake.NewClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "scylla-operator", Name: "op", Labels: map[string]string{"app": "op"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	proxy := &fakerest.RESTClient{
		NegotiatedSerializer: serializer.NewCodecFactory(runtime.NewScheme()).WithoutConversion(),
		Client: fakerest.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
		}),
	}

	dir := filepath.Join(t.TempDir(), TracesDirName)
	written, err := CollectTraces(context.Background(), kubeClient, proxy, CollectOptions{OperatorNamespace: "scylla-operator", OperatorSelector: "app=op", Port: 8095}, dir, "ns-a", TraceMeta{})
	if err != nil || len(written) != 0 {
		t.Fatalf("expected nothing written, got %v, %v", written, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected no traces directory, got %v", err)
	}
}

func TestReadTraceMetaFindsTheEnvtestAndTheE2ELayout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, meta := range map[string]TraceMeta{
		"spec.meta.json": {TestName: "envtest spec", Source: "envtest"},
		"ns.meta.json":   {TestName: "e2e test", Source: "e2e"},
	} {
		if err := WriteTraceMeta(filepath.Join(dir, name), meta); err != nil {
			t.Fatal(err)
		}
	}

	for trace, want := range map[string]string{
		"spec.jsonl":   "envtest spec",
		"ns.pod.jsonl": "e2e test",
	} {
		meta, err := ReadTraceMeta(filepath.Join(dir, trace))
		if err != nil || meta == nil || meta.TestName != want {
			t.Errorf("%s: expected %q, got %+v, %v", trace, want, meta, err)
		}
	}

	meta, err := ReadTraceMeta(filepath.Join(dir, "other.jsonl"))
	if err != nil || meta != nil {
		t.Errorf("expected no meta, got %+v, %v", meta, err)
	}
}
