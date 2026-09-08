package cacheconsistency

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
)

// ConsistencyStore records the writes of a controller, per kind, and tells when the informer caches have observed all
// of them. It is the controller-side counterpart of kube-controller-manager's ConsistencyStore. Writes of kinds that
// have no informer registered are ignored, which lets a controller write objects it doesn't read back from a cache,
// like PersistentVolumeClaims.
//
// Registration is not synchronized with use: all Register calls have to happen before the store is used.
// A nil *ConsistencyStore is a no-op, so controllers constructed without one keep working.
type ConsistencyStore struct {
	handlers map[schema.GroupVersionKind]*consistencyHandler
}

// NewConsistencyStore returns an empty store.
func NewConsistencyStore() *ConsistencyStore {
	return &ConsistencyStore{
		handlers: map[schema.GroupVersionKind]*consistencyHandler{},
	}
}

// Register makes the store record writes of the kind and wait for the informer to observe them.
func (ts *ConsistencyStore) Register(gvk schema.GroupVersionKind, informer cache.SharedIndexInformer) error {
	if _, exists := ts.handlers[gvk]; exists {
		return fmt.Errorf("%s is already registered", gvk)
	}

	handler, err := newConsistencyHandler(informer)
	if err != nil {
		return fmt.Errorf("can't create consistency handler for %s: %w", gvk, err)
	}
	ts.handlers[gvk] = handler

	return nil
}

// HasSynced returns true once the store has been delivered the initial state of all the informers.
func (ts *ConsistencyStore) HasSynced() bool {
	if ts == nil {
		return true
	}

	for _, handler := range ts.handlers {
		if !handler.HasSynced() {
			return false
		}
	}

	return true
}

// WroteAt records a write acknowledged by the API server: the object of the given kind identified by namespace and
// name now exists at resourceVersion.
func (ts *ConsistencyStore) WroteAt(gvk schema.GroupVersionKind, namespace, name, resourceVersion string) {
	if ts == nil {
		return
	}

	handler := ts.handlers[gvk]
	if handler == nil {
		return
	}

	handler.wroteAt(namespace, name, resourceVersion)
}

// Deleted records a delete acknowledged by the API server for the object of the given kind identified by namespace
// and name. The deleted instance is taken from the informer store.
func (ts *ConsistencyStore) Deleted(gvk schema.GroupVersionKind, namespace, name string) {
	if ts == nil {
		return
	}

	handler := ts.handlers[gvk]
	if handler == nil {
		return
	}

	handler.deletedByName(namespace, name)
}

// WaitReady blocks until all the informer caches reflect the recorded writes and deletes, or the context is done.
func (ts *ConsistencyStore) WaitReady(ctx context.Context) error {
	if ts == nil {
		return nil
	}

	var errs []error
	for gvk, handler := range ts.handlers {
		err := handler.waitReady(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't wait for %s cache: %w", gvk.Kind, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
