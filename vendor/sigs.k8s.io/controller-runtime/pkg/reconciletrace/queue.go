/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciletrace

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EventScopedQueue is implemented by a queue that attributes the adds made while handling a watch event to that
// event. The event handlers of informer-backed sources hand the queue returned by ForEvent to the user's handler.
type EventScopedQueue[request comparable] interface {
	ForEvent(e Event) priorityqueue.PriorityQueue[request]
}

var lastEventSeq atomic.Uint64

// NewCreateEvent describes a create event for obj.
func NewCreateEvent(obj any, inInitialList bool) Event {
	return Event{
		Seq:           lastEventSeq.Add(1),
		Type:          "Create",
		Object:        objectRef(obj),
		InInitialList: inInitialList,
	}
}

// NewUpdateEvent describes an update event from oldObj to newObj.
func NewUpdateEvent(oldObj, newObj any) Event {
	e := Event{
		Seq:    lastEventSeq.Add(1),
		Type:   "Update",
		Object: objectRef(newObj),
	}

	if o, ok := oldObj.(metav1.Object); ok {
		e.OldResourceVersion = o.GetResourceVersion()
		e.GenerationChanged = o.GetGeneration() != e.Object.Generation
		e.Resync = e.OldResourceVersion == e.Object.ResourceVersion
	}

	return e
}

// NewDeleteEvent describes a delete event for obj.
func NewDeleteEvent(obj any) Event {
	return Event{
		Seq:    lastEventSeq.Add(1),
		Type:   "Delete",
		Object: objectRef(obj),
	}
}

// pendingTriggers are the enqueues not yet taken off the queue by a reconcile.
type pendingTriggers struct {
	ids   []uint64
	first time.Time
}

// Queue wraps a controller's workqueue to record every enqueue with its cause and the reconciles that consume them.
// The workqueue coalesces the adds of a request that is waiting; the Queue keeps their records until the request is
// taken off the queue, and hands them to the reconcile as its triggers.
type Queue[request comparable] struct {
	priorityqueue.PriorityQueue[request]

	controller string
	recorder   Recorder

	mu       sync.Mutex
	pending  map[request]pendingTriggers
	inFlight map[request]pendingTriggers
}

var _ priorityqueue.PriorityQueue[reconcile.Request] = &Queue[reconcile.Request]{}
var _ EventScopedQueue[reconcile.Request] = &Queue[reconcile.Request]{}

// NewQueue wraps q of the named controller, recording to recorder.
func NewQueue[request comparable](controller string, q priorityqueue.PriorityQueue[request], recorder Recorder) *Queue[request] {
	return &Queue[request]{
		PriorityQueue: q,
		controller:    controller,
		recorder:      recorder,
		pending:       map[request]pendingTriggers{},
		inFlight:      map[request]pendingTriggers{},
	}
}

// Add implements workqueue.TypedInterface.
func (q *Queue[request]) Add(item request) {
	q.AddWithCause(Enqueue{Cause: CauseTypeUnknown}, priorityqueue.AddOpts{}, item)
}

// AddAfter implements workqueue.TypedDelayingInterface.
func (q *Queue[request]) AddAfter(item request, duration time.Duration) {
	q.AddWithCause(Enqueue{Cause: CauseTypeUnknown}, priorityqueue.AddOpts{After: duration}, item)
}

// AddRateLimited implements workqueue.TypedRateLimitingInterface.
func (q *Queue[request]) AddRateLimited(item request) {
	q.AddWithCause(Enqueue{Cause: CauseTypeUnknown}, priorityqueue.AddOpts{RateLimited: true}, item)
}

// AddWithOpts implements priorityqueue.PriorityQueue.
func (q *Queue[request]) AddWithOpts(o priorityqueue.AddOpts, items ...request) {
	q.AddWithCause(Enqueue{Cause: CauseTypeUnknown}, o, items...)
}

// AddWithCause adds items with o, recording an enqueue with cause for each of them. The cause's options are taken
// from o.
func (q *Queue[request]) AddWithCause(cause Enqueue, o priorityqueue.AddOpts, items ...request) {
	cause.After = o.After
	cause.RateLimited = o.RateLimited
	cause.Priority = o.Priority

	for _, item := range items {
		c := cause
		id := emit(q.recorder, Record{
			Type:       RecordTypeEnqueue,
			Controller: q.controller,
			Request:    FormatRequest(item),
			Enqueue:    &c,
		})

		q.mu.Lock()
		p := q.pending[item]
		if len(p.ids) == 0 {
			p.first = time.Now()
		}
		p.ids = append(p.ids, id)
		q.pending[item] = p
		q.mu.Unlock()
	}

	q.PriorityQueue.AddWithOpts(o, items...)
}

// Get implements workqueue.TypedInterface.
func (q *Queue[request]) Get() (request, bool) {
	item, shutdown := q.PriorityQueue.Get()
	if !shutdown {
		q.take(item)
	}

	return item, shutdown
}

// GetWithPriority implements priorityqueue.PriorityQueue.
func (q *Queue[request]) GetWithPriority() (request, int, bool) {
	item, priority, shutdown := q.PriorityQueue.GetWithPriority()
	if !shutdown {
		q.take(item)
	}

	return item, priority, shutdown
}

// take moves the pending triggers of item to the reconcile about to process it.
func (q *Queue[request]) take(item request) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.inFlight[item] = q.pending[item]
	delete(q.pending, item)
}

// ForEvent implements EventScopedQueue.
func (q *Queue[request]) ForEvent(e Event) priorityqueue.PriorityQueue[request] {
	return &eventQueue[request]{
		PriorityQueue: q,
		parent:        q,
		event:         e,
	}
}

// RecordReconcileStart records the start of the reconcile of req, with the enqueues that triggered it, and returns ctx
// carrying the reconcile.
func (q *Queue[request]) RecordReconcileStart(ctx context.Context, req request, reconcileID string, priority int) context.Context {
	q.mu.Lock()
	triggers := q.inFlight[req]
	delete(q.inFlight, req)
	q.mu.Unlock()

	info := ReconcileInfo{
		Controller:  q.controller,
		Request:     FormatRequest(req),
		ReconcileID: reconcileID,
	}

	start := &ReconcileStart{
		Triggers: triggers.ids,
		Priority: priority,
	}
	if !triggers.first.IsZero() {
		start.QueueWait = time.Since(triggers.first)
	}

	emit(q.recorder, Record{
		Type:           RecordTypeReconcileStart,
		Controller:     info.Controller,
		Request:        info.Request,
		ReconcileID:    info.ReconcileID,
		ReconcileStart: start,
	})

	return WithReconcile(withCallCounter(ctx), info)
}

// RecordReconcileEnd records the end of the reconcile ctx carries.
func (q *Queue[request]) RecordReconcileEnd(ctx context.Context, duration time.Duration, result reconcile.Result, err error) {
	info, _ := ReconcileFromContext(ctx)

	end := &ReconcileEnd{
		Duration:     duration,
		Requeue:      result.Requeue, //nolint:staticcheck // Requeue is recorded until it is removed.
		RequeueAfter: result.RequeueAfter,
		ClientCalls:  callCount(ctx),
	}
	if err != nil {
		end.Error = err.Error()
		end.Terminal = errors.Is(err, reconcile.TerminalError(nil))
	}

	emit(q.recorder, Record{
		Type:         RecordTypeReconcileEnd,
		Controller:   info.Controller,
		Request:      info.Request,
		ReconcileID:  info.ReconcileID,
		ReconcileEnd: end,
	})
}

// RequeueFromReconcile adds req back with o on behalf of the reconcile ctx carries, recording cause.
func (q *Queue[request]) RequeueFromReconcile(ctx context.Context, cause CauseType, o priorityqueue.AddOpts, req request) {
	info, _ := ReconcileFromContext(ctx)
	q.AddWithCause(Enqueue{Cause: cause, ByReconcileID: info.ReconcileID}, o, req)
}

// eventQueue attributes the adds made through it to one watch event. It embeds the parent queue so that it is a
// priorityqueue.PriorityQueue too: the handlers type-assert that to lower the priority of initial list events.
type eventQueue[request comparable] struct {
	priorityqueue.PriorityQueue[request]

	parent *Queue[request]
	event  Event
}

func (e *eventQueue[request]) cause() Enqueue {
	ev := e.event
	return Enqueue{Cause: CauseTypeEvent, Event: &ev}
}

func (e *eventQueue[request]) Add(item request) {
	e.parent.AddWithCause(e.cause(), priorityqueue.AddOpts{}, item)
}

func (e *eventQueue[request]) AddAfter(item request, duration time.Duration) {
	e.parent.AddWithCause(e.cause(), priorityqueue.AddOpts{After: duration}, item)
}

func (e *eventQueue[request]) AddRateLimited(item request) {
	e.parent.AddWithCause(e.cause(), priorityqueue.AddOpts{RateLimited: true}, item)
}

func (e *eventQueue[request]) AddWithOpts(o priorityqueue.AddOpts, items ...request) {
	e.parent.AddWithCause(e.cause(), o, items...)
}

type callCounterKey struct{}

func withCallCounter(ctx context.Context) context.Context {
	return context.WithValue(ctx, callCounterKey{}, &atomic.Int64{})
}

func countCall(ctx context.Context) {
	if c, ok := ctx.Value(callCounterKey{}).(*atomic.Int64); ok {
		c.Add(1)
	}
}

func callCount(ctx context.Context) int {
	if c, ok := ctx.Value(callCounterKey{}).(*atomic.Int64); ok {
		return int(c.Load())
	}

	return 0
}
