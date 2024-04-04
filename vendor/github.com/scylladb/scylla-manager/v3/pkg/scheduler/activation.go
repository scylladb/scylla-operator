// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"container/heap"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// Activation represents when a Key will be executed.
// Properties are optional they are only set for retries to ensure we retry
// the same thing.
// Stop is also optional if present specifies window end.
// Parametrized by scheduler key type.
type Activation[K comparable] struct {
	time.Time
	Key        K
	Retry      int8
	Properties Properties
	Stop       time.Time
}

// activationHeap implements heap.Interface.
// The activations are sorted by time in ascending order.
type activationHeap[K comparable] []Activation[K]

// uuid.UUID key type is used in pkg/service/scheduler package.
var _ heap.Interface = (*activationHeap[uuid.UUID])(nil)

func (h activationHeap[_]) Len() int { return len(h) }

func (h activationHeap[_]) Less(i, j int) bool {
	return h[i].Time.Before(h[j].Time)
}

func (h activationHeap[_]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *activationHeap[K]) Push(x interface{}) {
	*h = append(*h, x.(Activation[K]))
}

func (h *activationHeap[_]) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// activationQueue is a priority queue based on activationHeap.
// There may be only a single activation for a given activation key.
// On Push if key exists it is updated.
type activationQueue[K comparable] struct {
	h activationHeap[K]
}

func newActivationQueue[K comparable]() *activationQueue[K] {
	return &activationQueue[K]{
		h: []Activation[K]{},
	}
}

// Push returns true iff head was changed.
func (q *activationQueue[K]) Push(a Activation[K]) bool {
	if idx := q.find(a.Key); idx >= 0 {
		[]Activation[K](q.h)[idx] = a
		heap.Fix(&q.h, idx)
	} else {
		heap.Push(&q.h, a)
	}
	return q.h[0].Key == a.Key
}

func (q *activationQueue[K]) Pop() (Activation[K], bool) {
	if len(q.h) == 0 {
		return Activation[K]{}, false
	}
	return heap.Pop(&q.h).(Activation[K]), true
}

func (q *activationQueue[K]) Top() (Activation[K], bool) {
	if len(q.h) == 0 {
		return Activation[K]{}, false
	}
	return []Activation[K](q.h)[0], true
}

// Remove returns true iff head if head was changed.
func (q *activationQueue[K]) Remove(key K) bool {
	idx := q.find(key)
	if idx >= 0 {
		heap.Remove(&q.h, idx)
	}
	return idx == 0
}

func (q *activationQueue[K]) find(key K) int {
	for i, v := range q.h {
		if v.Key == key {
			return i
		}
	}
	return -1
}

func (q *activationQueue[_]) Size() int {
	return len(q.h)
}
