// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestObjectControl(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	control := NewObjectControl[corev1.ConfigMap](ctx, c, c)

	_, err := control.GetCached("ns", "cm")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected a not found error before the create, got %v", err)
	}

	created, err := control.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm"},
		Data:       map[string]string{"key": "created"},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	cached, err := control.GetCached("ns", "cm")
	if err != nil {
		t.Fatal(err)
	}
	if cached.Data["key"] != "created" {
		t.Errorf("expected the cached read to see the create, got %q", cached.Data["key"])
	}

	created.Data["key"] = "updated"
	_, err = control.Update(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	live, err := control.Get(ctx, "ns", "cm")
	if err != nil {
		t.Fatal(err)
	}
	if live.Data["key"] != "updated" {
		t.Errorf("expected the read to see the update, got %q", live.Data["key"])
	}

	err = control.Delete(ctx, "ns", "cm", metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = control.Get(ctx, "ns", "cm")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected a not found error after the delete, got %v", err)
	}
}

func TestObjectControl_GetReadsLive(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm"}}
	cached := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	live := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(cm).Build()
	control := NewObjectControl[corev1.ConfigMap](ctx, cached, live)

	_, err := control.GetCached("ns", "cm")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the cached read to miss, got %v", err)
	}

	got, err := control.Get(ctx, "ns", "cm")
	if err != nil {
		t.Fatalf("expected the live read to find the object the cache misses, got %v", err)
	}
	if got.Name != cm.Name {
		t.Errorf("expected %q, got %q", cm.Name, got.Name)
	}
}
