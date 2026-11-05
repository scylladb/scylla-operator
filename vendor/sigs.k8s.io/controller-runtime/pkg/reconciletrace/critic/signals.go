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

package critic

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
)

// Signals are the facts about an episode computed without a model. They hold for any controller.
type Signals struct {
	Reconciles int `json:"reconciles"`
	// ReconcileErrors are the reconcile ends that returned an error.
	ReconcileErrors []uint64 `json:"reconcileErrors,omitempty"`
	// NoOpWrites are the updates and patches the API server didn't persist.
	NoOpWrites []uint64 `json:"noOpWrites,omitempty"`
	// Conflicts are the writes rejected with a conflict.
	Conflicts []uint64 `json:"conflicts,omitempty"`
	// FailedWrites are the other writes that failed.
	FailedWrites []uint64 `json:"failedWrites,omitempty"`
	// SelfTriggeredIdleReconciles are reconcile starts triggered only by watch events of the controller's own writes
	// and by its own requeues, that wrote nothing.
	SelfTriggeredIdleReconciles []uint64 `json:"selfTriggeredIdleReconciles,omitempty"`
	// IdleReconciles is the number of reconciles that wrote nothing.
	IdleReconciles int `json:"idleReconciles"`
	// RepeatedlyWrittenObjects are the objects written in several reconciles.
	RepeatedlyWrittenObjects []RepeatedWrite `json:"repeatedlyWrittenObjects,omitempty"`
	// Requeues counts the requeues by cause.
	Requeues       map[reconciletrace.CauseType]int `json:"requeues,omitempty"`
	MaxQueueWait   time.Duration                    `json:"maxQueueWait"`
	MaxReconcile   time.Duration                    `json:"maxReconcile"`
	TotalReconcile time.Duration                    `json:"totalReconcile"`
	Span           time.Duration                    `json:"span"`
}

// RepeatedWrite is an object written in several reconciles.
type RepeatedWrite struct {
	Object     string   `json:"object"`
	Reconciles int      `json:"reconciles"`
	Writes     []uint64 `json:"writes"`
}

func isWrite(c *reconciletrace.ClientCall) bool {
	return c.Verb != "get" && c.Verb != "list"
}

// objectKey identifies the object of a client call or event across resource versions.
func objectKey(ref reconciletrace.ObjectRef, subResource string) string {
	key := fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name)
	if subResource != "" {
		key += "/" + subResource
	}

	return key
}

// ComputeSignals computes the signals of e.
func ComputeSignals(e *Episode) Signals {
	s := Signals{
		Requeues: map[reconciletrace.CauseType]int{},
		Span:     e.End().Sub(e.Start()),
	}

	// ownWrites maps the resource versions the controller's writes produced to the written objects, to tell the
	// watch events of its own writes.
	ownWrites := map[string]string{}
	enqueues := map[uint64]*reconciletrace.Enqueue{}
	writesByReconcile := map[string]int{}
	type objectWrites struct {
		reconciles map[string]bool
		writes     []uint64
	}
	writesByObject := map[string]*objectWrites{}
	var objectOrder []string

	for i := range e.Records {
		r := &e.Records[i]
		switch r.Type {
		case reconciletrace.RecordTypeEnqueue:
			enqueues[r.ID] = r.Enqueue
			switch r.Enqueue.Cause {
			case reconciletrace.CauseTypeRequeue, reconciletrace.CauseTypeRequeueAfter, reconciletrace.CauseTypeError:
				s.Requeues[r.Enqueue.Cause]++
			}

		case reconciletrace.RecordTypeClientCall:
			c := r.ClientCall
			if !isWrite(c) {
				continue
			}
			switch {
			case c.ErrorReason == "Conflict":
				s.Conflicts = append(s.Conflicts, r.ID)
			case c.Error != "":
				s.FailedWrites = append(s.FailedWrites, r.ID)
			case c.NoOp:
				s.NoOpWrites = append(s.NoOpWrites, r.ID)
			}
			if c.Error != "" || c.NoOp {
				continue
			}

			writesByReconcile[r.ReconcileID]++
			if c.ResourceVersionAfter != "" {
				ownWrites[c.ResourceVersionAfter] = objectKey(c.Object, "")
			}
			key := objectKey(c.Object, c.SubResource)
			ow, ok := writesByObject[key]
			if !ok {
				ow = &objectWrites{reconciles: map[string]bool{}}
				writesByObject[key] = ow
				objectOrder = append(objectOrder, key)
			}
			ow.reconciles[r.ReconcileID] = true
			ow.writes = append(ow.writes, r.ID)

		case reconciletrace.RecordTypeReconcileEnd:
			s.Reconciles++
			end := r.ReconcileEnd
			if end.Error != "" {
				s.ReconcileErrors = append(s.ReconcileErrors, r.ID)
			}
			s.TotalReconcile += end.Duration
			s.MaxReconcile = max(s.MaxReconcile, end.Duration)
			if writesByReconcile[r.ReconcileID] == 0 {
				s.IdleReconciles++
			}
		}
	}

	// A second pass: a reconcile's triggers are recorded before it, its writes after.
	for i := range e.Records {
		r := &e.Records[i]
		if r.Type != reconciletrace.RecordTypeReconcileStart {
			continue
		}
		s.MaxQueueWait = max(s.MaxQueueWait, r.ReconcileStart.QueueWait)
		if writesByReconcile[r.ReconcileID] > 0 || len(r.ReconcileStart.Triggers) == 0 {
			continue
		}
		selfTriggered := true
		for _, id := range r.ReconcileStart.Triggers {
			if !isOwnCause(enqueues[id], ownWrites) {
				selfTriggered = false
				break
			}
		}
		if selfTriggered {
			s.SelfTriggeredIdleReconciles = append(s.SelfTriggeredIdleReconciles, r.ID)
		}
	}

	for _, key := range objectOrder {
		ow := writesByObject[key]
		if len(ow.reconciles) < 2 {
			continue
		}
		s.RepeatedlyWrittenObjects = append(s.RepeatedlyWrittenObjects, RepeatedWrite{
			Object:     key,
			Reconciles: len(ow.reconciles),
			Writes:     ow.writes,
		})
	}
	slices.SortStableFunc(s.RepeatedlyWrittenObjects, func(a, b RepeatedWrite) int {
		return b.Reconciles - a.Reconciles
	})

	return s
}

// isOwnCause tells an enqueue caused by the controller itself: its own requeue, or a watch event of one of its own
// writes.
func isOwnCause(enq *reconciletrace.Enqueue, ownWrites map[string]string) bool {
	if enq == nil {
		return false
	}

	switch enq.Cause {
	case reconciletrace.CauseTypeRequeue, reconciletrace.CauseTypeRequeueAfter, reconciletrace.CauseTypeError:
		return true
	case reconciletrace.CauseTypeEvent:
		return isOwnEvent(enq.Event, ownWrites)
	default:
		return false
	}
}

// isOwnEvent tells the watch event of one of the controller's own writes: the event's object is at a resource
// version one of the writes produced.
func isOwnEvent(ev *reconciletrace.Event, ownWrites map[string]string) bool {
	if ev == nil || ev.Type == "Delete" {
		return false
	}

	written, ok := ownWrites[ev.Object.ResourceVersion]
	return ok && written == objectKey(ev.Object, "")
}

// Summary renders the signals for a prompt or a report.
func (s Signals) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "- reconciles: %d over %s, %d of them wrote nothing\n", s.Reconciles, s.Span.Round(time.Millisecond), s.IdleReconciles)
	fmt.Fprintf(&b, "- reconcile time: total %s, max %s; max queue wait %s\n", s.TotalReconcile.Round(time.Millisecond), s.MaxReconcile.Round(time.Millisecond), s.MaxQueueWait.Round(time.Millisecond))
	fmt.Fprintf(&b, "- requeues: %s\n", formatCounts(s.Requeues))
	fmt.Fprintf(&b, "- reconcile errors: %s\n", formatIDs(s.ReconcileErrors))
	fmt.Fprintf(&b, "- no-op writes (accepted, not persisted): %s\n", formatIDs(s.NoOpWrites))
	fmt.Fprintf(&b, "- conflicts: %s; other failed writes: %s\n", formatIDs(s.Conflicts), formatIDs(s.FailedWrites))
	fmt.Fprintf(&b, "- reconciles triggered only by the controller's own writes or requeues that wrote nothing: %s\n", formatIDs(s.SelfTriggeredIdleReconciles))
	if len(s.RepeatedlyWrittenObjects) == 0 {
		b.WriteString("- objects written in several reconciles: none\n")
	} else {
		b.WriteString("- objects written in several reconciles:\n")
		for _, rw := range s.RepeatedlyWrittenObjects {
			fmt.Fprintf(&b, "  - %s: %d reconciles, writes %s\n", rw.Object, rw.Reconciles, formatIDs(rw.Writes))
		}
	}

	return b.String()
}

func formatIDs(ids []uint64) string {
	if len(ids) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("#%d", id))
	}

	return strings.Join(parts, ", ")
}

func formatCounts[K ~string](counts map[K]int) string {
	if len(counts) == 0 {
		return "none"
	}

	keys := make([]K, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}

	return strings.Join(parts, ", ")
}
