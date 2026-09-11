// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// The metav1 options given to the typed-client-shaped functions have to reach the API server whole: a StatefulSet
// recreated with a lost Orphan propagation policy takes its Pods down with it.
func TestOptionsReachTheClient(t *testing.T) {
	t.Parallel()

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "sts",
			UID:       types.UID("uid"),
		},
	}

	var gotDelete *metav1.DeleteOptions
	var gotCreate *metav1.CreateOptions
	var gotUpdate *metav1.UpdateOptions
	var gotPatch *metav1.PatchOptions
	c := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(sts).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				gotDelete = (&client.DeleteOptions{}).ApplyOptions(opts).AsDeleteOptions()
				return nil
			},
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				gotCreate = (&client.CreateOptions{}).ApplyOptions(opts).AsCreateOptions()
				return nil
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				gotUpdate = (&client.UpdateOptions{}).ApplyOptions(opts).AsUpdateOptions()
				return nil
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				gotPatch = (&client.PatchOptions{}).ApplyOptions(opts).AsPatchOptions()
				return nil
			},
		}).
		Build()

	ctx := t.Context()
	control := ApplyControl[appsv1.StatefulSet](ctx, c, "ns")

	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](5),
		Preconditions: &metav1.Preconditions{
			UID:             ptr.To(types.UID("uid")),
			ResourceVersion: ptr.To("42"),
		},
		PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan),
		DryRun:            []string{metav1.DryRunAll},
	}
	err := control.Delete(ctx, "sts", deleteOpts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&deleteOpts, gotDelete); diff != "" {
		t.Errorf("delete options differ (-want +got):\n%s", diff)
	}

	createOpts := metav1.CreateOptions{
		DryRun:          []string{metav1.DryRunAll},
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err = control.Create(ctx, sts.DeepCopy(), createOpts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&createOpts, gotCreate); diff != "" {
		t.Errorf("create options differ (-want +got):\n%s", diff)
	}

	updateOpts := metav1.UpdateOptions{
		DryRun:          []string{metav1.DryRunAll},
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err = control.Update(ctx, sts.DeepCopy(), updateOpts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&updateOpts, gotUpdate); diff != "" {
		t.Errorf("update options differ (-want +got):\n%s", diff)
	}

	patchOpts := metav1.PatchOptions{
		DryRun:          []string{metav1.DryRunAll},
		Force:           ptr.To(true),
		FieldManager:    "manager",
		FieldValidation: metav1.FieldValidationStrict,
	}
	_, err = PatchFunc[appsv1.StatefulSet](c, "ns")(ctx, "sts", types.MergePatchType, []byte(`{}`), patchOpts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&patchOpts, gotPatch); diff != "" {
		t.Errorf("patch options differ (-want +got):\n%s", diff)
	}
}
