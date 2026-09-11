// Package cacheconsistency provides read-your-own-writes semantics on top of shared informers.
//
// Informer caches are eventually consistent: a write acknowledged by the API server is only reflected in the cache
// once the informer has processed the corresponding watch event. A controller that decides from the cache right
// after writing can therefore decide from state that predates its own write.
//
// A ConsistencyStore closes that gap: it records writes and waits until the informers have observed them. One
// consistency handler per informer does the work. Writes are recorded with the resourceVersion returned by the API
// server and deletes with the UID of the deleted object. Wait blocks until the informer has observed all of them,
// relying on the resourceVersion being monotonically increasing for a resource type (see KEP-5505) and on watch
// events being delivered in resourceVersion order: once the informer store has seen resourceVersion R, every change
// with a lower resourceVersion is already in it.
//
// The resourceVersion the store has seen is taken from the store itself (Store.LastStoreSyncResourceVersion, added
// in client-go 1.36 and advanced by events, bookmarks and relists alike) and, should that be unavailable, from the
// events delivered to the handler itself.
//
// The approach is the one kube-controller-manager uses since Kubernetes 1.36 for its Pod-owning controllers and the
// one controller-runtime's read-your-writes consistency (kubernetes-sigs/controller-runtime pull request 3472) uses,
// adapted to client-go shared informers and listers.
package cacheconsistency

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// storePollInterval bounds how long a waiter takes to notice the store advancing without an event being delivered,
// which is how bookmarks and relists advance it.
const storePollInterval = 100 * time.Millisecond

// ConsistencyError is returned by WaitReady when the context is done before the caches have observed all the writes.
type ConsistencyError struct {
	PendingResourceVersions map[string]int64
	PendingDeletes          map[string][]types.UID
	Err                     error
}

func (e *ConsistencyError) Error() string {
	return fmt.Sprintf(
		"cache has not observed all writes yet (pending resourceVersions: %v, pending deletes: %v): %v",
		e.PendingResourceVersions, e.PendingDeletes, e.Err,
	)
}

func (e *ConsistencyError) Unwrap() error {
	return e.Err
}

type objectKey struct {
	namespace string
	name      string
}

func (k objectKey) String() string {
	if len(k.namespace) == 0 {
		return k.name
	}
	return k.namespace + "/" + k.name
}

func objectKeyFor(obj metav1.Object) objectKey {
	return objectKey{
		namespace: obj.GetNamespace(),
		name:      obj.GetName(),
	}
}

// consistencyHandler records the writes made to a resource type and tells when the informer cache of that type has
// observed them. It is registered as an event handler on the informer. It is safe for concurrent use.
type consistencyHandler struct {
	informer     cache.SharedIndexInformer
	registration cache.ResourceEventHandlerRegistration

	lock sync.Mutex
	// observedRV is the highest resourceVersion seen in any event delivered by the informer. It backs up the
	// informer store's own resourceVersion, which isn't available when the AtomicFIFO client-go feature gate is
	// disabled.
	observedRV int64
	// pendingRVs holds, per object, the resourceVersion the informer has to observe before the object's writes
	// are guaranteed to be in the cache. Entries not above observedRV are pruned.
	pendingRVs map[objectKey]int64
	// pendingDeletes holds, per object, the UIDs whose deletion the informer has to observe.
	pendingDeletes map[objectKey]sets.Set[types.UID]
	// changed is closed and replaced whenever the observed state advances, waking up the waiters.
	changed chan struct{}
}

// newConsistencyHandler registers a handler on the informer. The informer has to observe every object the handler is
// told about, so the informer's namespace and selector restrictions must include all the objects written through it.
func newConsistencyHandler(informer cache.SharedIndexInformer) (*consistencyHandler, error) {
	t := &consistencyHandler{
		informer:       informer,
		pendingRVs:     map[objectKey]int64{},
		pendingDeletes: map[objectKey]sets.Set[types.UID]{},
		changed:        make(chan struct{}),
	}

	registration, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    t.onAdd,
		UpdateFunc: t.onUpdate,
		DeleteFunc: t.onDelete,
	})
	if err != nil {
		return nil, fmt.Errorf("can't register event handler: %w", err)
	}
	t.registration = registration

	return t, nil
}

// HasSynced returns true once the handler has been delivered the initial state of the informer.
func (t *consistencyHandler) HasSynced() bool {
	return t.registration.HasSynced()
}

// wroteAt records a write acknowledged by the API server: the object identified by namespace and name now exists at
// resourceVersion.
func (t *consistencyHandler) wroteAt(namespace, name, resourceVersion string) {
	rv, err := parseResourceVersion(resourceVersion)
	if err != nil {
		klog.ErrorS(err, "Can't record write, its resourceVersion is not comparable", "Ref", klog.KRef(namespace, name))
		return
	}

	key := objectKey{namespace: namespace, name: name}

	t.lock.Lock()
	defer t.lock.Unlock()

	if rv <= t.observedResourceVersionLocked() {
		return
	}
	if rv > t.pendingRVs[key] {
		t.pendingRVs[key] = rv
	}
}

// observedResourceVersionLocked returns the resourceVersion the informer store is known to have caught up with.
func (t *consistencyHandler) observedResourceVersionLocked() int64 {
	observedRV := t.observedRV

	storeRV := t.informer.GetStore().LastStoreSyncResourceVersion()
	if len(storeRV) != 0 {
		rv, err := parseResourceVersion(storeRV)
		if err != nil {
			klog.ErrorS(err, "Can't parse the store resourceVersion")
		} else if rv > observedRV {
			observedRV = rv
		}
	}

	return observedRV
}

// deleted records a delete acknowledged by the API server. obj is the object that was deleted, as read from the
// cache; its UID identifies the deleted instance so that a recreated object with the same name is told apart.
// Objects that already have a deletionTimestamp are not recorded, as the delete didn't change them.
func (t *consistencyHandler) deleted(obj metav1.Object) {
	if obj.GetDeletionTimestamp() != nil {
		return
	}

	uid := obj.GetUID()
	if len(uid) == 0 {
		klog.ErrorS(nil, "Can't record delete of an object without UID", "Ref", klog.KObj(obj))
		return
	}

	key := objectKeyFor(obj)

	t.lock.Lock()
	defer t.lock.Unlock()

	if t.pendingDeletes[key] == nil {
		t.pendingDeletes[key] = sets.New[types.UID]()
	}
	t.pendingDeletes[key].Insert(uid)
}

// deletedByName records a delete acknowledged by the API server for the object identified by namespace and name,
// taking the deleted instance from the informer store. Nothing is recorded when the store doesn't hold the object,
// as there is nothing left for the cache to observe.
func (t *consistencyHandler) deletedByName(namespace, name string) {
	key := objectKey{namespace: namespace, name: name}

	untyped, exists, err := t.informer.GetIndexer().GetByKey(key.String())
	if err != nil {
		klog.ErrorS(err, "Can't get object from the informer store, its delete won't be recorded", "Ref", key.String())
		return
	}
	if !exists {
		return
	}

	obj, err := meta.Accessor(untyped)
	if err != nil {
		klog.ErrorS(err, "Can't access object metadata, its delete won't be recorded", "Ref", key.String())
		return
	}

	t.deleted(obj)
}

// waitReady blocks until the informer cache reflects all the recorded writes and deletes, or the context is done.
func (t *consistencyHandler) waitReady(ctx context.Context) error {
	for {
		// Grab the wait channel before checking, so that a change racing with the check is never missed.
		changed := t.waitChan()

		pendingRVs, pendingDeletes := t.settle()
		if len(pendingRVs) == 0 && len(pendingDeletes) == 0 {
			return nil
		}

		select {
		case <-changed:
		case <-time.After(storePollInterval):
		case <-ctx.Done():
			return &ConsistencyError{
				PendingResourceVersions: pendingRVs,
				PendingDeletes:          pendingDeletes,
				Err:                     ctx.Err(),
			}
		}
	}
}

func (t *consistencyHandler) waitChan() <-chan struct{} {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.changed
}

func (t *consistencyHandler) broadcastLocked() {
	close(t.changed)
	t.changed = make(chan struct{})
}

// settle drops the pending entries the cache has caught up with and returns the ones that remain.
func (t *consistencyHandler) settle() (map[string]int64, map[string][]types.UID) {
	t.lock.Lock()
	defer t.lock.Unlock()

	observedRV := t.observedResourceVersionLocked()
	for key, rv := range t.pendingRVs {
		if rv <= observedRV {
			delete(t.pendingRVs, key)
		}
	}

	for key, uids := range t.pendingDeletes {
		for uid := range uids {
			if t.isDeletedInCacheLocked(key, uid) {
				uids.Delete(uid)
			}
		}
		if uids.Len() == 0 {
			delete(t.pendingDeletes, key)
		}
	}

	if len(t.pendingRVs) == 0 && len(t.pendingDeletes) == 0 {
		return nil, nil
	}

	pendingRVs := make(map[string]int64, len(t.pendingRVs))
	for key, rv := range t.pendingRVs {
		pendingRVs[key.String()] = rv
	}
	pendingDeletes := make(map[string][]types.UID, len(t.pendingDeletes))
	for key, uids := range t.pendingDeletes {
		pendingDeletes[key.String()] = sets.List(uids)
	}

	return pendingRVs, pendingDeletes
}

// isDeletedInCacheLocked tells whether the cache no longer holds the given instance of an object: it is gone, it
// was replaced by an instance with a different UID, or it is being deleted, which is what a delete of an object
// with finalizers results in.
func (t *consistencyHandler) isDeletedInCacheLocked(key objectKey, uid types.UID) bool {
	untyped, exists, err := t.informer.GetIndexer().GetByKey(key.String())
	if err != nil {
		klog.ErrorS(err, "Can't get object from the informer store", "Ref", key.String())
		return false
	}
	if !exists {
		return true
	}

	obj, err := meta.Accessor(untyped)
	if err != nil {
		klog.ErrorS(err, "Can't access object metadata", "Ref", key.String())
		return false
	}

	return obj.GetUID() != uid || obj.GetDeletionTimestamp() != nil
}

func (t *consistencyHandler) observe(obj metav1.Object, deleted bool) {
	rv, err := parseResourceVersion(obj.GetResourceVersion())
	if err != nil {
		klog.ErrorS(err, "Can't observe event, the resourceVersion is not comparable", "Ref", klog.KObj(obj))
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	changed := false
	if rv > t.observedRV {
		t.observedRV = rv
		changed = true
	}

	// Deletions are settled against the store on wait, but a delete event is a cheap and definite signal.
	if deleted || obj.GetDeletionTimestamp() != nil {
		key := objectKeyFor(obj)
		if uids := t.pendingDeletes[key]; uids != nil && uids.Has(obj.GetUID()) {
			changed = true
		}
	}

	if changed {
		t.broadcastLocked()
	}
}

func (t *consistencyHandler) onAdd(untyped any) {
	obj, err := meta.Accessor(untyped)
	if err != nil {
		klog.ErrorS(err, "Can't access object metadata", "Object", untyped)
		return
	}

	t.observe(obj, false)
}

func (t *consistencyHandler) onUpdate(_, untyped any) {
	obj, err := meta.Accessor(untyped)
	if err != nil {
		klog.ErrorS(err, "Can't access object metadata", "Object", untyped)
		return
	}

	t.observe(obj, false)
}

func (t *consistencyHandler) onDelete(untyped any) {
	if tombstone, ok := untyped.(cache.DeletedFinalStateUnknown); ok {
		untyped = tombstone.Obj
	}

	obj, err := meta.Accessor(untyped)
	if err != nil {
		klog.ErrorS(err, "Can't access object metadata", "Object", untyped)
		return
	}

	t.observe(obj, true)
}

func parseResourceVersion(resourceVersion string) (int64, error) {
	rv, err := strconv.ParseInt(resourceVersion, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("can't parse resourceVersion %q: %w", resourceVersion, err)
	}

	return rv, nil
}
