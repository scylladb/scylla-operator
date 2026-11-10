//go:build envtest

package controllers

import (
	"context"
	"slices"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A trigger table checks which objects a controller reconciles for each kind of change: the rows run in order
// against one fixture, each waiting for the controller to go idle, applying its change and comparing the
// reconciled objects with the expected ones. The controller reports its reconciliations to a reconcileRecorder
// through its onReconcile option.

const (
	// triggerQuietWindow is how long the controller has to go without a reconciliation for the fixture to count as
	// settled, and how long a change that must not trigger one is watched. The resync period is hours away, so
	// once the controller stops reacting to its own writes nothing but the row's change can wake it.
	triggerQuietWindow = 2 * time.Second

	// triggerTimeout bounds the wait for the controller to go idle and for the expected reconciliations.
	triggerTimeout = 15 * time.Second

	// triggerAnnotation is an annotation no controller manages, so setting it changes an object without changing
	// what the controller wants it to be.
	triggerAnnotation = "internal.scylla-operator.scylladb.com/envtest-trigger"
)

// reconcileRecorder records the objects a controller reconciles.
type reconcileRecorder struct {
	mu         sync.Mutex
	reconciled []types.NamespacedName
	last       time.Time
}

func (r *reconcileRecorder) observe(key types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reconciled = append(r.reconciled, key)
	r.last = time.Now()
}

// names returns the names of the objects reconciled since the last reset, each once.
func (r *reconcileRecorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var names []string
	for _, key := range r.reconciled {
		if !slices.Contains(names, key.Name) {
			names = append(names, key.Name)
		}
	}

	return names
}

func (r *reconcileRecorder) sinceLast() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	return time.Since(r.last)
}

// waitUntilIdle waits until the controller has run no reconciliation for the quiet window, then forgets the
// reconciliations so far.
func (r *reconcileRecorder) waitUntilIdle(ctx context.Context) {
	g.GinkgoHelper()

	o.Eventually(r.sinceLast).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.BeNumerically(">=", triggerQuietWindow))

	r.mu.Lock()
	defer r.mu.Unlock()
	r.reconciled = nil
}

// triggerActions are the changes the rows apply, for embedding into a controller's trigger fixture.
type triggerActions struct {
	client client.Client
}

// annotate sets the trigger annotation on obj to a fresh value.
func (a triggerActions) annotate(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[triggerAnnotation] = apimachineryutilrand.String(8)
	obj.SetAnnotations(annotations)

	err := a.client.Patch(ctx, obj, patch)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (a triggerActions) create(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	err := a.client.Create(ctx, obj)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (a triggerActions) delete(ctx context.Context, obj client.Object) {
	g.GinkgoHelper()

	err := a.client.Delete(ctx, obj)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// triggerRow is a row of a trigger table over fixture F.
type triggerRow[F any] struct {
	// change is what the row does to the cluster.
	change func(ctx context.Context, f F)
	// expected are the names of the objects the change must have reconciled, and nothing else; none means the
	// change must not reconcile anything.
	expected []string
}

// runTriggerRow waits for the controller to go idle, applies the row's change to f and verifies that the
// controller reconciles exactly the expected objects.
func runTriggerRow[F any](ctx context.Context, recorder *reconcileRecorder, f F, row triggerRow[F]) {
	g.GinkgoHelper()

	g.By("Waiting for the controller to go idle")
	recorder.waitUntilIdle(ctx)

	g.By("Applying the change")
	row.change(ctx, f)

	if len(row.expected) == 0 {
		g.By("Verifying nothing is reconciled")
		o.Consistently(recorder.names).WithContext(ctx).WithTimeout(triggerQuietWindow).WithPolling(100 * time.Millisecond).Should(o.BeEmpty())
		return
	}

	g.By("Waiting for the expected objects to be reconciled, and no other")
	o.Eventually(recorder.names).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.ConsistOf(row.expected))
	o.Consistently(recorder.names).WithContext(ctx).WithTimeout(triggerQuietWindow).WithPolling(100 * time.Millisecond).Should(o.ConsistOf(row.expected))
}
