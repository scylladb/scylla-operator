// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The controls are shaped for the controllerhelpers and resourceapply functions; these tests run them through those
// functions over a fake controller-runtime client, the way a controller would.

const testNamespace = "ns"

func newTestController() *scyllav1alpha1.ScyllaDBDatacenter {
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "sdc",
			UID:       types.UID("sdc-uid"),
		},
	}
}

func newControllerRef(controller *scyllav1alpha1.ScyllaDBDatacenter) metav1.OwnerReference {
	return *metav1.NewControllerRef(controller, scyllav1alpha1.ScyllaDBDatacenterGVK)
}

func TestGetObjectsControlClaimsObjects(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	controller := newTestController()
	selectorLabels := map[string]string{"app": "sdc"}
	foreignRef := newControllerRef(controller)
	foreignRef.UID = types.UID("someone-else")

	newService := func(name string, matching bool, ownerRefs ...metav1.OwnerReference) *corev1.Service {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       testNamespace,
				Name:            name,
				OwnerReferences: ownerRefs,
			},
		}
		if matching {
			svc.Labels = selectorLabels
		}
		return svc
	}
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
		controller,
		newService("owned", true, newControllerRef(controller)),
		newService("orphan", true),
		newService("foreign", true, foreignRef),
		newService("stale", false, newControllerRef(controller)),
	).Build()

	objects, err := controllerhelpers.GetObjects[*scyllav1alpha1.ScyllaDBDatacenter, *corev1.Service](
		ctx,
		controller,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		labels.SelectorFromSet(selectorLabels),
		GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, corev1.Service](ctx, c, c, testNamespace),
	)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(objects))
	for name := range objects {
		got = append(got, name)
	}
	if len(objects) != 2 || objects["owned"] == nil || objects["orphan"] == nil {
		t.Errorf("expected the owned and the adopted Service, got %v", got)
	}

	controllerUID := func(name string) types.UID {
		svc, err := Get[corev1.Service](ctx, c, testNamespace, name)
		if err != nil {
			t.Fatal(err)
		}
		ref := metav1.GetControllerOf(svc)
		if ref == nil {
			return ""
		}
		return ref.UID
	}
	if uid := controllerUID("orphan"); uid != controller.UID {
		t.Errorf("expected the orphan to be adopted, controller UID is %q", uid)
	}
	if uid := controllerUID("stale"); uid != "" {
		t.Errorf("expected the stale Service to be released, controller UID is %q", uid)
	}
	if uid := controllerUID("foreign"); uid != foreignRef.UID {
		t.Errorf("expected the foreign Service to be left alone, controller UID is %q", uid)
	}
}

func TestPruneControlDeletesUnrequiredObjects(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	newConfigMap := func(name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name, UID: types.UID(name + "-uid")},
		}
	}
	required := newConfigMap("required")
	unrequired := newConfigMap("unrequired")
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(required, unrequired).Build()

	err := controllerhelpers.Prune(
		ctx,
		[]*corev1.ConfigMap{required},
		map[string]*corev1.ConfigMap{required.Name: required, unrequired.Name: unrequired},
		PruneControl[corev1.ConfigMap](c, testNamespace),
		record.NewFakeRecorder(10),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Get[corev1.ConfigMap](ctx, c, testNamespace, required.Name)
	if err != nil {
		t.Errorf("expected the required ConfigMap to stay, got %v", err)
	}
	_, err = Get[corev1.ConfigMap](ctx, c, testNamespace, unrequired.Name)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected the unrequired ConfigMap to be deleted, got %v", err)
	}
}

func TestApplyControlAppliesThroughResourceApply(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	controller := newTestController()
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(controller).Build()
	control := ApplyControl[corev1.ConfigMap](ctx, c, testNamespace)
	recorder := record.NewFakeRecorder(10)
	required := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            "cm",
			OwnerReferences: []metav1.OwnerReference{newControllerRef(controller)},
		},
		Data: map[string]string{"key": "v1"},
	}

	_, changed, err := resourceapply.ApplyConfigMapWithControl(ctx, control, recorder, required, resourceapply.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected the first apply to create the ConfigMap")
	}

	_, changed, err = resourceapply.ApplyConfigMapWithControl(ctx, control, recorder, required, resourceapply.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected the second apply of the same ConfigMap to change nothing")
	}

	required.Data["key"] = "v2"
	applied, changed, err := resourceapply.ApplyConfigMapWithControl(ctx, control, recorder, required, resourceapply.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || applied.Data["key"] != "v2" {
		t.Errorf("expected the apply of changed data to update the ConfigMap, changed=%v data=%v", changed, applied.Data)
	}
}

func TestListersReadThroughTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	newPod := func(namespace, name string, labels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels}}
	}
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
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
