package cacheconsistency

import (
	"context"
	"fmt"
	"reflect"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// Client is the method set client-gen gives every typed client, over the object type T and the list type L.
// It mirrors k8s.io/client-go/gentype, so the generated typed client interfaces satisfy it structurally.
type Client[T kubeinterfaces.ObjectInterface, L any] interface {
	Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
	Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error)
	List(ctx context.Context, opts metav1.ListOptions) (L, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

// StatusClient is the verb client-gen adds for kinds with a status subresource.
type StatusClient[T kubeinterfaces.ObjectInterface] interface {
	UpdateStatus(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
}

// ApplyClient is the verb client-gen adds when apply configurations are generated, over the apply configuration
// type AC.
type ApplyClient[T kubeinterfaces.ObjectInterface, AC any] interface {
	Apply(ctx context.Context, obj AC, opts metav1.ApplyOptions) (T, error)
}

// ApplyStatusClient is the verb client-gen adds for kinds with both a status subresource and apply configurations.
type ApplyStatusClient[T kubeinterfaces.ObjectInterface, AC any] interface {
	ApplyStatus(ctx context.Context, obj AC, opts metav1.ApplyOptions) (T, error)
}

// ClientWithStatus is a Client of a kind with a status subresource.
type ClientWithStatus[T kubeinterfaces.ObjectInterface, L any] interface {
	Client[T, L]
	StatusClient[T]
}

// ClientWithApply is a Client of a kind with apply configurations.
type ClientWithApply[T kubeinterfaces.ObjectInterface, L, AC any] interface {
	Client[T, L]
	ApplyClient[T, AC]
}

// ClientWithApplyAndStatus is a Client of a kind with both a status subresource and apply configurations.
type ClientWithApplyAndStatus[T kubeinterfaces.ObjectInterface, L, AC any] interface {
	Client[T, L]
	StatusClient[T]
	ApplyClient[T, AC]
	ApplyStatusClient[T, AC]
}

// recorder identifies the kind and namespace of a client and holds the store its writes are recorded in.
type recorder struct {
	gvk       schema.GroupVersionKind
	namespace string
	store     *ConsistencyStore
}

// newRecorder resolves the kind of T through the scheme, so that no kind name has to be spelled out for a client.
// T has to be a pointer to a type registered in the scheme, which holds for every type a typed client is generated
// for; anything else is a programming error and panics.
func newRecorder[T kubeinterfaces.ObjectInterface](namespace string, store *ConsistencyStore) *recorder {
	var zero T
	obj, ok := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(runtime.Object)
	if !ok {
		panic(fmt.Sprintf("cacheconsistency: %T is not a runtime.Object", zero))
	}

	gvk, err := resource.GetObjectGVK(obj)
	if err != nil {
		panic(fmt.Sprintf("cacheconsistency: can't determine the kind of %T: %v", zero, err))
	}

	return &recorder{
		gvk:       *gvk,
		namespace: namespace,
		store:     store,
	}
}

// observeWrite records a successful write of obj.
func (t *recorder) observeWrite(obj metav1.Object) {
	t.store.WroteAt(t.gvk, obj.GetNamespace(), obj.GetName(), obj.GetResourceVersion())
}

// observeDelete records a delete that removed the object from the API server, or found it already removed, so that
// the next read waits for the cache to drop it too. It returns err unchanged.
func (t *recorder) observeDelete(name string, err error) error {
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.store.Deleted(t.gvk, t.namespace, name)

	return err
}

// observeWrite records a successful write of obj and returns the write's results unchanged.
func observeWrite[T kubeinterfaces.ObjectInterface](t *recorder, obj T, err error) (T, error) {
	if err != nil {
		return obj, err
	}

	t.observeWrite(obj)

	return obj, nil
}

// RecordingClient is a Client that records its writes in the ConsistencyStore it was created with, so that WaitReady
// on the store covers them. Reads pass through untouched. DeleteCollection passes through unrecorded, as its response
// doesn't tell what was deleted; the operator doesn't use it.
type RecordingClient[T kubeinterfaces.ObjectInterface, L any] struct {
	Client[T, L]
	recorder *recorder
}

// NewRecordingClient wraps client, a typed client of T scoped to namespace, so that its writes are recorded in store.
func NewRecordingClient[T kubeinterfaces.ObjectInterface, L any](client Client[T, L], namespace string, store *ConsistencyStore) *RecordingClient[T, L] {
	return newRecordingClient(client, newRecorder[T](namespace, store))
}

func newRecordingClient[T kubeinterfaces.ObjectInterface, L any](client Client[T, L], t *recorder) *RecordingClient[T, L] {
	return &RecordingClient[T, L]{
		Client:   client,
		recorder: t,
	}
}

func (c *RecordingClient[T, L]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	created, err := c.Client.Create(ctx, obj, opts)
	return observeWrite(c.recorder, created, err)
}

func (c *RecordingClient[T, L]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	updated, err := c.Client.Update(ctx, obj, opts)
	return observeWrite(c.recorder, updated, err)
}

func (c *RecordingClient[T, L]) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
	patched, err := c.Client.Patch(ctx, name, pt, data, opts, subresources...)
	return observeWrite(c.recorder, patched, err)
}

func (c *RecordingClient[T, L]) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.recorder.observeDelete(name, c.Client.Delete(ctx, name, opts))
}

type recordingStatusClient[T kubeinterfaces.ObjectInterface] struct {
	StatusClient[T]
	recorder *recorder
}

func (c *recordingStatusClient[T]) UpdateStatus(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	updated, err := c.StatusClient.UpdateStatus(ctx, obj, opts)
	return observeWrite(c.recorder, updated, err)
}

type recordingApplyClient[T kubeinterfaces.ObjectInterface, AC any] struct {
	ApplyClient[T, AC]
	recorder *recorder
}

func (c *recordingApplyClient[T, AC]) Apply(ctx context.Context, obj AC, opts metav1.ApplyOptions) (T, error) {
	applied, err := c.ApplyClient.Apply(ctx, obj, opts)
	return observeWrite(c.recorder, applied, err)
}

type recordingApplyStatusClient[T kubeinterfaces.ObjectInterface, AC any] struct {
	ApplyStatusClient[T, AC]
	recorder *recorder
}

func (c *recordingApplyStatusClient[T, AC]) ApplyStatus(ctx context.Context, obj AC, opts metav1.ApplyOptions) (T, error) {
	applied, err := c.ApplyStatusClient.ApplyStatus(ctx, obj, opts)
	return observeWrite(c.recorder, applied, err)
}

// RecordingClientWithStatus is a RecordingClient of a kind with a status subresource.
type RecordingClientWithStatus[T kubeinterfaces.ObjectInterface, L any] struct {
	*RecordingClient[T, L]
	*recordingStatusClient[T]
	recorder *recorder
}

// NewRecordingClientWithStatus is NewRecordingClient for a kind with a status subresource.
func NewRecordingClientWithStatus[T kubeinterfaces.ObjectInterface, L any](client ClientWithStatus[T, L], namespace string, store *ConsistencyStore) *RecordingClientWithStatus[T, L] {
	t := newRecorder[T](namespace, store)
	return &RecordingClientWithStatus[T, L]{
		RecordingClient:       newRecordingClient(client, t),
		recordingStatusClient: &recordingStatusClient[T]{StatusClient: client, recorder: t},
		recorder:              t,
	}
}

// RecordingClientWithApply is a RecordingClient of a kind with apply configurations.
type RecordingClientWithApply[T kubeinterfaces.ObjectInterface, L, AC any] struct {
	*RecordingClient[T, L]
	*recordingApplyClient[T, AC]
	recorder *recorder
}

// NewRecordingClientWithApply is NewRecordingClient for a kind with apply configurations.
func NewRecordingClientWithApply[T kubeinterfaces.ObjectInterface, L, AC any](client ClientWithApply[T, L, AC], namespace string, store *ConsistencyStore) *RecordingClientWithApply[T, L, AC] {
	t := newRecorder[T](namespace, store)
	return &RecordingClientWithApply[T, L, AC]{
		RecordingClient:      newRecordingClient(client, t),
		recordingApplyClient: &recordingApplyClient[T, AC]{ApplyClient: client, recorder: t},
		recorder:             t,
	}
}

// RecordingClientWithApplyAndStatus is a RecordingClient of a kind with both a status subresource and apply
// configurations.
type RecordingClientWithApplyAndStatus[T kubeinterfaces.ObjectInterface, L, AC any] struct {
	*RecordingClient[T, L]
	*recordingStatusClient[T]
	*recordingApplyClient[T, AC]
	*recordingApplyStatusClient[T, AC]
	recorder *recorder
}

// NewRecordingClientWithApplyAndStatus is NewRecordingClient for a kind with both a status subresource and apply
// configurations.
func NewRecordingClientWithApplyAndStatus[T kubeinterfaces.ObjectInterface, L, AC any](client ClientWithApplyAndStatus[T, L, AC], namespace string, store *ConsistencyStore) *RecordingClientWithApplyAndStatus[T, L, AC] {
	t := newRecorder[T](namespace, store)
	return &RecordingClientWithApplyAndStatus[T, L, AC]{
		RecordingClient:            newRecordingClient(client, t),
		recordingStatusClient:      &recordingStatusClient[T]{StatusClient: client, recorder: t},
		recordingApplyClient:       &recordingApplyClient[T, AC]{ApplyClient: client, recorder: t},
		recordingApplyStatusClient: &recordingApplyStatusClient[T, AC]{ApplyStatusClient: client, recorder: t},
		recorder:                   t,
	}
}
