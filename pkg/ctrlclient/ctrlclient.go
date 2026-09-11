// Copyright (c) 2026 ScyllaDB.

// Package ctrlclient adapts a controller-runtime client to the function-shaped interfaces the controllers and their
// helpers (resourceapply, controllerhelpers) take, so that a controller reading and writing through the manager's
// client keeps using them unchanged. Every helper is generic over the object type and needs no per-kind code.
//
// The type parameters follow one convention: T is the object struct (e.g. corev1.Pod) and PT its pointer type, which
// is what the controllers pass around. Callers spell out T only; PT is inferred.
package ctrlclient

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Object constrains PT to the pointer to T that is a Kubernetes object.
type Object[T any] interface {
	*T
	kubeinterfaces.ObjectInterface
}

// Get reads the named object through c.
func Get[T any, PT Object[T]](ctx context.Context, c client.Reader, namespace, name string) (PT, error) {
	obj := PT(new(T))
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// List reads the objects of PT's kind in namespace matching selector through c. The list type is resolved from the
// operator's scheme.
func List[T any, PT Object[T]](ctx context.Context, c client.Reader, namespace string, selector labels.Selector) ([]PT, error) {
	list, err := newList[T, PT]()
	if err != nil {
		return nil, err
	}

	err = c.List(ctx, list, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return nil, err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("can't extract items from %T: %w", list, err)
	}

	res := make([]PT, 0, len(items))
	for _, item := range items {
		obj, ok := item.(PT)
		if !ok {
			return nil, fmt.Errorf("list %T holds %T, expected %T", list, item, obj)
		}
		res = append(res, obj)
	}

	return res, nil
}

// ListFunc returns a lister-shaped List over c for namespace.
func ListFunc[T any, PT Object[T]](ctx context.Context, c client.Reader, namespace string) func(labels.Selector) ([]PT, error) {
	return func(selector labels.Selector) ([]PT, error) {
		return List[T, PT](ctx, c, namespace, selector)
	}
}

// GetFunc returns a typed-client-shaped Get over c for namespace. Give it the manager's API reader for a live read.
func GetFunc[T any, PT Object[T]](c client.Reader, namespace string) func(ctx context.Context, name string, opts metav1.GetOptions) (PT, error) {
	return func(ctx context.Context, name string, opts metav1.GetOptions) (PT, error) {
		obj := PT(new(T))
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj, getOptions(opts))
		if err != nil {
			return nil, err
		}

		return obj, nil
	}
}

// PatchFunc returns a typed-client-shaped Patch over c for namespace.
func PatchFunc[T any, PT Object[T]](c client.Client, namespace string) func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (PT, error) {
	return func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (PT, error) {
		obj := PT(new(T))
		obj.SetNamespace(namespace)
		obj.SetName(name)

		var err error
		switch len(subresources) {
		case 0:
			err = c.Patch(ctx, obj, client.RawPatch(pt, data), patchOptions(opts))
		case 1:
			err = c.SubResource(subresources[0]).Patch(ctx, obj, client.RawPatch(pt, data), &client.SubResourcePatchOptions{PatchOptions: *patchOptions(opts)})
		default:
			return nil, fmt.Errorf("can't patch more than one subresource at a time, got %v", subresources)
		}
		if err != nil {
			return nil, err
		}

		return obj, nil
	}
}

// DeleteFunc returns a typed-client-shaped Delete over c for namespace.
func DeleteFunc[T any, PT Object[T]](c client.Client, namespace string) func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		obj := PT(new(T))
		obj.SetNamespace(namespace)
		obj.SetName(name)

		return c.Delete(ctx, obj, deleteOptions(opts))
	}
}

// ApplyControl returns the resourceapply control over c for namespace: cached reads and writes through the same
// client, so that with a read-your-writes client the apply sees what it wrote.
func ApplyControl[T any, PT Object[T]](ctx context.Context, c client.Client, namespace string) resourceapply.ApplyControlFuncs[PT] {
	return resourceapply.ApplyControlFuncs[PT]{
		GetCachedFunc: func(name string) (PT, error) {
			return Get[T, PT](ctx, c, namespace, name)
		},
		CreateFunc: func(ctx context.Context, obj PT, opts metav1.CreateOptions) (PT, error) {
			err := c.Create(ctx, obj, createOptions(opts))
			if err != nil {
				return nil, err
			}

			return obj, nil
		},
		UpdateFunc: func(ctx context.Context, obj PT, opts metav1.UpdateOptions) (PT, error) {
			err := c.Update(ctx, obj, updateOptions(opts))
			if err != nil {
				return nil, err
			}

			return obj, nil
		},
		DeleteFunc: DeleteFunc[T, PT](c, namespace),
	}
}

// PruneControl returns the controllerhelpers prune control over c for namespace.
func PruneControl[T any, PT Object[T]](c client.Client, namespace string) controllerhelpers.PruneControlInterface {
	return &controllerhelpers.PruneControlFuncs{
		DeleteFunc: DeleteFunc[T, PT](c, namespace),
	}
}

// GetObjectsControl returns the controllerhelpers control for claiming the objects of PT's kind owned by a controller
// of PCT's kind in namespace: cached lists and patches through c, and the live read of the controller through
// apiReader that adoption depends on.
func GetObjectsControl[CT, T any, PCT Object[CT], PT Object[T]](ctx context.Context, c client.Client, apiReader client.Reader, namespace string) controllerhelpers.ControlleeManagerGetObjectsFuncs[PCT, PT] {
	return controllerhelpers.ControlleeManagerGetObjectsFuncs[PCT, PT]{
		GetControllerUncachedFunc: GetFunc[CT, PCT](apiReader, namespace),
		ListObjectsFunc:           ListFunc[T, PT](ctx, c, namespace),
		PatchObjectFunc:           PatchFunc[T, PT](c, namespace),
	}
}

func newList[T any, PT Object[T]]() (client.ObjectList, error) {
	gvk, err := apiutil.GVKForObject(PT(new(T)), scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("can't get GVK for %T: %w", PT(nil), err)
	}

	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	obj, err := scheme.Scheme.New(listGVK)
	if err != nil {
		return nil, fmt.Errorf("can't create %s: %w", listGVK, err)
	}

	list, ok := obj.(client.ObjectList)
	if !ok {
		return nil, fmt.Errorf("%T is not a client.ObjectList", obj)
	}

	return list, nil
}

// The controller-runtime options only take Raw as a base: their own fields overwrite the matching Raw fields when the
// request is built, zero values included. So the metav1 options are spelled out field by field, or a DeleteOptions
// would lose its propagation policy and preconditions on the way.

func getOptions(opts metav1.GetOptions) *client.GetOptions {
	return &client.GetOptions{
		Raw: &opts,
	}
}

func createOptions(opts metav1.CreateOptions) *client.CreateOptions {
	return &client.CreateOptions{
		DryRun:          opts.DryRun,
		FieldManager:    opts.FieldManager,
		FieldValidation: opts.FieldValidation,
		Raw:             &opts,
	}
}

func updateOptions(opts metav1.UpdateOptions) *client.UpdateOptions {
	return &client.UpdateOptions{
		DryRun:          opts.DryRun,
		FieldManager:    opts.FieldManager,
		FieldValidation: opts.FieldValidation,
		Raw:             &opts,
	}
}

func patchOptions(opts metav1.PatchOptions) *client.PatchOptions {
	return &client.PatchOptions{
		DryRun:          opts.DryRun,
		Force:           opts.Force,
		FieldManager:    opts.FieldManager,
		FieldValidation: opts.FieldValidation,
		Raw:             &opts,
	}
}

func deleteOptions(opts metav1.DeleteOptions) *client.DeleteOptions {
	return &client.DeleteOptions{
		GracePeriodSeconds: opts.GracePeriodSeconds,
		Preconditions:      opts.Preconditions,
		PropagationPolicy:  opts.PropagationPolicy,
		DryRun:             opts.DryRun,
		Raw:                &opts,
	}
}
