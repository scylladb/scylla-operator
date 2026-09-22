// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// newTestScheme registers only the kinds the tests use, so that the tests don't lean on what the operator's scheme
// happens to hold.
func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	for _, install := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, scyllav1alpha1.Install} {
		if err := install(s); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// recordedOptions holds the metav1 options the fake client received, one per write verb.
type recordedOptions struct {
	delete *metav1.DeleteOptions
	create *metav1.CreateOptions
	update *metav1.UpdateOptions
	patch  *metav1.PatchOptions
}

// newOptionsRecordingClient returns a fake client holding objs that records the options of every write instead of
// performing it.
func newOptionsRecordingClient(t *testing.T, objs ...client.Object) (client.Client, *recordedOptions) {
	t.Helper()

	got := &recordedOptions{}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				got.delete = (&client.DeleteOptions{}).ApplyOptions(opts).AsDeleteOptions()
				return nil
			},
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				got.create = (&client.CreateOptions{}).ApplyOptions(opts).AsCreateOptions()
				return nil
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				got.update = (&client.UpdateOptions{}).ApplyOptions(opts).AsUpdateOptions()
				return nil
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				got.patch = (&client.PatchOptions{}).ApplyOptions(opts).AsPatchOptions()
				return nil
			},
		}).
		Build()
	return c, got
}

func newTestStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "sts",
			UID:       types.UID("uid"),
		},
	}
}

// The metav1 options given to the typed-client-shaped functions have to reach the API server whole: a StatefulSet
// recreated with a lost Orphan propagation policy takes its Pods down with it.
func TestDeleteOptionsReachTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sts := newTestStatefulSet()
	c, got := newOptionsRecordingClient(t, sts)

	opts := metav1.DeleteOptions{
		GracePeriodSeconds: new(int64(5)),
		Preconditions: &metav1.Preconditions{
			UID:             new(types.UID("uid")),
			ResourceVersion: new("42"),
		},
		OrphanDependents:  new(true),
		PropagationPolicy: new(metav1.DeletePropagationOrphan),
		DryRun:            []string{metav1.DryRunAll},
		IgnoreStoreReadErrorWithClusterBreakingPotential: new(true),
	}
	err := ApplyControl[appsv1.StatefulSet](ctx, c, testNamespace).Delete(ctx, sts.Name, opts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&opts, got.delete); diff != "" {
		t.Errorf("delete options differ (-want +got):\n%s", diff)
	}
}

func TestCreateOptionsReachTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c, got := newOptionsRecordingClient(t)

	opts := metav1.CreateOptions{
		DryRun:          []string{metav1.DryRunAll},
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err := ApplyControl[appsv1.StatefulSet](ctx, c, testNamespace).Create(ctx, newTestStatefulSet(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&opts, got.create); diff != "" {
		t.Errorf("create options differ (-want +got):\n%s", diff)
	}
}

func TestUpdateOptionsReachTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sts := newTestStatefulSet()
	c, got := newOptionsRecordingClient(t, sts)

	opts := metav1.UpdateOptions{
		DryRun:          []string{metav1.DryRunAll},
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err := ApplyControl[appsv1.StatefulSet](ctx, c, testNamespace).Update(ctx, sts.DeepCopy(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&opts, got.update); diff != "" {
		t.Errorf("update options differ (-want +got):\n%s", diff)
	}
}

func TestPatchOptionsReachTheClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sts := newTestStatefulSet()
	c, got := newOptionsRecordingClient(t, sts)

	opts := metav1.PatchOptions{
		DryRun:          []string{metav1.DryRunAll},
		Force:           new(true),
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err := PatchFunc[appsv1.StatefulSet](c, testNamespace)(ctx, sts.Name, types.MergePatchType, []byte(`{}`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&opts, got.patch); diff != "" {
		t.Errorf("patch options differ (-want +got):\n%s", diff)
	}
}

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
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(
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
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(required, unrequired).Build()

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

	controller := newTestController()
	required := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            "cm",
			OwnerReferences: []metav1.OwnerReference{newControllerRef(controller)},
		},
		Data: map[string]string{"key": "v1"},
	}
	// applied returns cm the way an earlier apply would have left it, with the hash annotation resourceapply compares.
	applied := func(cm *corev1.ConfigMap) *corev1.ConfigMap {
		if err := resourceapply.SetHashAnnotation(cm); err != nil {
			t.Fatal(err)
		}
		return cm
	}

	tt := []struct {
		name            string
		existing        *corev1.ConfigMap
		expectedCalls   []string
		expectedChanged bool
	}{
		{
			name:            "creates the ConfigMap when it is missing",
			existing:        nil,
			expectedCalls:   []string{"Get", "Create"},
			expectedChanged: true,
		},
		{
			name:            "leaves the ConfigMap unchanged when it is identical",
			existing:        applied(required.DeepCopy()),
			expectedCalls:   []string{"Get"},
			expectedChanged: false,
		},
		{
			name: "updates the ConfigMap when it differs",
			existing: applied(func() *corev1.ConfigMap {
				cm := required.DeepCopy()
				cm.Data = map[string]string{"key": "v0"}
				return cm
			}()),
			expectedCalls:   []string{"Get", "Update"},
			expectedChanged: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			// The fake client sets a resource version on the objects it is given, so a subtest gets its own copy.
			objects := []client.Object{controller.DeepCopy()}
			if tc.existing != nil {
				objects = append(objects, tc.existing)
			}
			var calls []string
			c := fake.NewClientBuilder().
				WithScheme(newTestScheme(t)).
				WithObjects(objects...).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						calls = append(calls, "Get")
						return c.Get(ctx, key, obj, opts...)
					},
					Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						calls = append(calls, "Create")
						return c.Create(ctx, obj, opts...)
					},
					Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						calls = append(calls, "Update")
						return c.Update(ctx, obj, opts...)
					},
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						calls = append(calls, "Delete")
						return c.Delete(ctx, obj, opts...)
					},
				}).
				Build()
			control := ApplyControl[corev1.ConfigMap](ctx, c, testNamespace)

			got, changed, err := resourceapply.ApplyConfigMapWithControl(ctx, control, record.NewFakeRecorder(10), required.DeepCopy(), resourceapply.ApplyOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expectedCalls, calls); diff != "" {
				t.Errorf("expected calls differ (-want +got):\n%s", diff)
			}
			if changed != tc.expectedChanged {
				t.Errorf("expected changed to be %v, got %v", tc.expectedChanged, changed)
			}
			if diff := cmp.Diff(required.Data, got.Data); diff != "" {
				t.Errorf("expected the applied data to match the required (-want +got):\n%s", diff)
			}
		})
	}
}
