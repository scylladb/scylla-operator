// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestObjectControl(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	// The fake client is one store, so its reads trivially observe its writes.
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	control := NewObjectControl[corev1.ConfigMap](ctx, NewReadYourWritesClient(c))

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
