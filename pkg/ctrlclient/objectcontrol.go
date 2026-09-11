// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectControl reads and writes the objects of PT's kind through c. The reads without a context of their own use
// the one it was created with, so create it per reconciliation. It satisfies kubecrypto.ObjectControl.
type ObjectControl[T any, PT Object[T]] struct {
	ctx context.Context
	c   client.Client
}

func NewObjectControl[T any, PT Object[T]](ctx context.Context, c client.Client) ObjectControl[T, PT] {
	return ObjectControl[T, PT]{
		ctx: ctx,
		c:   c,
	}
}

func (oc ObjectControl[T, PT]) GetCached(namespace, name string) (PT, error) {
	return Get[T, PT](oc.ctx, oc.c, namespace, name)
}

// Get reads through the client under ctx. With a read-your-writes client that is as fresh as a live read for
// everything this controller wrote.
func (oc ObjectControl[T, PT]) Get(ctx context.Context, namespace, name string) (PT, error) {
	return Get[T, PT](ctx, oc.c, namespace, name)
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
