// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectControl reads and writes the objects of PT's kind through c, and reads live through reader where the caller
// asks for a read its cache can't be trusted with. The reads without a context of their own use the one it was created
// with, so create it per reconciliation. It satisfies kubecrypto.ObjectControl.
type ObjectControl[T any, PT Object[T]] struct {
	ctx    context.Context
	c      client.Client
	reader client.Reader
}

// NewObjectControl returns an ObjectControl over c, with reader for the live reads; give it the manager's API reader.
func NewObjectControl[T any, PT Object[T]](ctx context.Context, c client.Client, reader client.Reader) ObjectControl[T, PT] {
	return ObjectControl[T, PT]{
		ctx:    ctx,
		c:      c,
		reader: reader,
	}
}

func (oc ObjectControl[T, PT]) GetCached(namespace, name string) (PT, error) {
	return Get[T, PT](oc.ctx, oc.c, namespace, name)
}

// Get reads live through the reader, for the decisions a cache miss must not make.
func (oc ObjectControl[T, PT]) Get(ctx context.Context, namespace, name string) (PT, error) {
	return Get[T, PT](ctx, oc.reader, namespace, name)
}

func (oc ObjectControl[T, PT]) Create(ctx context.Context, obj PT, opts metav1.CreateOptions) (PT, error) {
	err := oc.c.Create(ctx, obj, createOptions(opts))
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (oc ObjectControl[T, PT]) Update(ctx context.Context, obj PT, opts metav1.UpdateOptions) (PT, error) {
	err := oc.c.Update(ctx, obj, updateOptions(opts))
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (oc ObjectControl[T, PT]) Delete(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return DeleteFunc[T, PT](oc.c, namespace)(ctx, name, opts)
}
