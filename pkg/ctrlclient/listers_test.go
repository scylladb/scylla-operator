// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestListersReadThroughTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	newPod := func(namespace, name string, labels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels}}
	}
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(
		newPod(testNamespace, "a", map[string]string{"app": "x"}),
		newPod(testNamespace, "b", map[string]string{"app": "y"}),
		newPod("other", "c", map[string]string{"app": "x"}),
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "s"}},
	).Build()

	pods := NewPodLister(ctx, c)
	pod, err := pods.Pods(testNamespace).Get("a")
	if err != nil || pod.Name != "a" {
		t.Errorf("expected to get Pod a, got %v, %v", pod, err)
	}
	namespaced, err := pods.Pods(testNamespace).List(labels.SelectorFromSet(map[string]string{"app": "x"}))
	if err != nil || len(namespaced) != 1 || namespaced[0].Name != "a" {
		t.Errorf("expected the namespaced list to hold Pod a only, got %v, %v", namespaced, err)
	}
	all, err := pods.List(labels.SelectorFromSet(map[string]string{"app": "x"}))
	if err != nil || len(all) != 2 {
		t.Errorf("expected the cluster-wide list to hold 2 Pods, got %d, %v", len(all), err)
	}

	secret, err := NewSecretLister(ctx, c).Secrets(testNamespace).Get("s")
	if err != nil || secret.Name != "s" {
		t.Errorf("expected to get Secret s, got %v, %v", secret, err)
	}
	_, err = NewSecretLister(ctx, c).Secrets(testNamespace).Get("missing")
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected a not found error, got %v", err)
	}
}
