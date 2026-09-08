package cacheconsistency

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

const (
	testNamespace = "ns"
	testName      = "cm"
	testUID       = types.UID("uid-1")
)

func newConfigMap(rv string, uid types.UID) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			UID:             uid,
			ResourceVersion: rv,
		},
	}
}

// listOnlyListerWatcher opts the reflector out of streaming lists, which the fake watcher doesn't implement, so that
// the informer populates its cache with a plain list.
type listOnlyListerWatcher struct {
	*cache.ListWatch
}

func (lw *listOnlyListerWatcher) IsWatchListSemanticsUnSupported() bool {
	return true
}

// testInformer runs a shared informer over a fake watch so that events can be delivered at will.
type testInformer struct {
	informer cache.SharedIndexInformer
	watcher  *watch.RaceFreeFakeWatcher
	handler  *consistencyHandler
}

// newTestSharedInformer returns a not yet started shared informer of the given list, fed by the returned fake watcher.
func newTestSharedInformer(t *testing.T, exampleObject runtime.Object, list runtime.Object) (cache.SharedIndexInformer, *watch.RaceFreeFakeWatcher) {
	t.Helper()

	watcher := watch.NewRaceFreeFake()
	lw := &listOnlyListerWatcher{
		ListWatch: &cache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				return list, nil
			},
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				return watcher, nil
			},
		},
	}

	return cache.NewSharedIndexInformer(lw, exampleObject, 0, cache.Indexers{}), watcher
}

// runTestInformer runs the informer until the test ends and waits for it and the given handlers to sync.
func runTestInformer(t *testing.T, informer cache.SharedIndexInformer, synced ...cache.InformerSynced) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), append([]cache.InformerSynced{informer.HasSynced}, synced...)...) {
		t.Fatal("can't sync informer")
	}
}

func newTestInformer(t *testing.T, initial ...*corev1.ConfigMap) *testInformer {
	t.Helper()

	list := &corev1.ConfigMapList{
		ListMeta: metav1.ListMeta{ResourceVersion: "10"},
	}
	for _, cm := range initial {
		list.Items = append(list.Items, *cm)
	}

	informer, watcher := newTestSharedInformer(t, &corev1.ConfigMap{}, list)

	handler, err := newConsistencyHandler(informer)
	if err != nil {
		t.Fatalf("can't create consistency handler: %v", err)
	}

	runTestInformer(t, informer, handler.HasSynced)

	return &testInformer{
		informer: informer,
		watcher:  watcher,
		handler:  handler,
	}
}

func waitWithTimeout(t *testing.T, handler *consistencyHandler, timeout time.Duration) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	return handler.waitReady(ctx)
}

func expectWaitBlocks(t *testing.T, handler *consistencyHandler) {
	t.Helper()

	err := waitWithTimeout(t, handler, 200*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wait to block until the deadline, got: %v", err)
	}
}

func expectWaitReturns(t *testing.T, handler *consistencyHandler) {
	t.Helper()

	err := waitWithTimeout(t, handler, 5*time.Second)
	if err != nil {
		t.Fatalf("expected wait to return, got: %v", err)
	}
}

func TestConsistencyHandler_WaitWithNothingPending(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_WriteAlreadyObserved(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "5")

	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_WriteBlocksUntilObserved(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "20")
	expectWaitBlocks(t, ti.handler)

	// An event with a lower resourceVersion doesn't unblock the wait.
	ti.watcher.Modify(newConfigMap("15", testUID))
	expectWaitBlocks(t, ti.handler)

	ti.watcher.Modify(newConfigMap("20", testUID))
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_WriteIsUnblockedByAnyEventOfTheKind(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "20")
	expectWaitBlocks(t, ti.handler)

	// Events are delivered in resourceVersion order, so observing a higher one for another object proves the write
	// has been observed too.
	other := newConfigMap("21", "uid-other")
	other.Name = "other"
	ti.watcher.Add(other)
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_CreateBlocksUntilObserved(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t)

	ti.handler.wroteAt(testNamespace, testName, "20")
	expectWaitBlocks(t, ti.handler)

	ti.watcher.Add(newConfigMap("20", testUID))
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_DeleteBlocksUntilGoneFromTheStore(t *testing.T) {
	t.Parallel()

	cm := newConfigMap("5", testUID)
	ti := newTestInformer(t, cm)

	ti.handler.deleted(cm)
	expectWaitBlocks(t, ti.handler)

	deleted := newConfigMap("30", testUID)
	ti.watcher.Delete(deleted)
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_DeleteIsSatisfiedByDeletionTimestamp(t *testing.T) {
	t.Parallel()

	cm := newConfigMap("5", testUID)
	ti := newTestInformer(t, cm)

	ti.handler.deleted(cm)
	expectWaitBlocks(t, ti.handler)

	// An object with finalizers stays in the store with a deletionTimestamp set.
	terminating := newConfigMap("30", testUID)
	terminating.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	terminating.Finalizers = []string{"test/finalizer"}
	ti.watcher.Modify(terminating)
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_DeleteIsSatisfiedByRecreation(t *testing.T) {
	t.Parallel()

	cm := newConfigMap("5", testUID)
	ti := newTestInformer(t, cm)

	ti.handler.deleted(cm)
	expectWaitBlocks(t, ti.handler)

	// A tombstone can be missed on relist; the store holding a different instance under the same name is enough.
	ti.watcher.Modify(newConfigMap("30", "uid-2"))
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_DeleteOfTerminatingObjectIsNotRecorded(t *testing.T) {
	t.Parallel()

	cm := newConfigMap("5", testUID)
	cm.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	cm.Finalizers = []string{"test/finalizer"}
	ti := newTestInformer(t, cm)

	ti.handler.deleted(cm)
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_DeleteAndRecreateAreBothRecorded(t *testing.T) {
	t.Parallel()

	cm := newConfigMap("5", testUID)
	ti := newTestInformer(t, cm)

	// Mirrors resourceapply recreating an object: delete, then create under the same name.
	ti.handler.deleted(cm)
	ti.handler.wroteAt(testNamespace, testName, "40")
	expectWaitBlocks(t, ti.handler)

	ti.watcher.Delete(newConfigMap("35", testUID))
	expectWaitBlocks(t, ti.handler)

	ti.watcher.Add(newConfigMap("40", "uid-2"))
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_WaitReportsPendingOnTimeout(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "20")

	err := waitWithTimeout(t, ti.handler, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected an error")
	}
	var pendingErr *ConsistencyError
	if !errors.As(err, &pendingErr) {
		t.Fatalf("expected a %T, got %T: %v", pendingErr, err, err)
	}
	expected := "cache has not observed all writes yet (pending resourceVersions: map[ns/cm:20], pending deletes: map[]): context deadline exceeded"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

func TestConsistencyHandler_UnparsableResourceVersionIsIgnored(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "not-a-number")
	expectWaitReturns(t, ti.handler)
}

func TestConsistencyHandler_WriteIsUnblockedByBookmark(t *testing.T) {
	t.Parallel()

	ti := newTestInformer(t, newConfigMap("5", testUID))

	ti.handler.wroteAt(testNamespace, testName, "20")
	expectWaitBlocks(t, ti.handler)

	// A watch bookmark advances the store's resourceVersion without any object event being delivered.
	ti.watcher.Action(watch.Bookmark, newConfigMap("25", testUID))
	expectWaitReturns(t, ti.handler)
}
