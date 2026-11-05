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
	"encoding/json"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RecordType tells what a Record describes.
type RecordType string

const (
	// RecordTypeEnqueue is a request added to the workqueue, by a watch event, by the controller after a reconcile, or
	// by something else.
	RecordTypeEnqueue RecordType = "Enqueue"
	// RecordTypeReconcileStart is a reconcile taken off the workqueue.
	RecordTypeReconcileStart RecordType = "ReconcileStart"
	// RecordTypeReconcileEnd is a reconcile that returned.
	RecordTypeReconcileEnd RecordType = "ReconcileEnd"
	// RecordTypeClientCall is a call made through a client wrapped with WrapClient or WrapReader.
	RecordTypeClientCall RecordType = "ClientCall"
)

// Record is one entry of a trace. Exactly one of the type-specific fields is set, matching Type.
type Record struct {
	// ID is unique within the process and grows with time.
	ID   uint64     `json:"id"`
	Time time.Time  `json:"time"`
	Type RecordType `json:"type"`

	// Controller is the name of the controller the record belongs to.
	Controller string `json:"controller,omitempty"`
	// Request is the request the record belongs to, as formatted by fmt.
	Request string `json:"request,omitempty"`
	// ReconcileID identifies the reconcile the record belongs to, if any.
	ReconcileID string `json:"reconcileID,omitempty"`

	Enqueue        *Enqueue        `json:"enqueue,omitempty"`
	ReconcileStart *ReconcileStart `json:"reconcileStart,omitempty"`
	ReconcileEnd   *ReconcileEnd   `json:"reconcileEnd,omitempty"`
	ClientCall     *ClientCall     `json:"clientCall,omitempty"`
}

// ObjectRef identifies an object at a resource version.
type ObjectRef struct {
	Kind            string `json:"kind"`
	Namespace       string `json:"namespace,omitempty"`
	Name            string `json:"name,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	Generation      int64  `json:"generation,omitempty"`
}

// CauseType tells why a request was enqueued.
type CauseType string

const (
	// CauseTypeEvent is a watch event mapped to the request by an event handler.
	CauseTypeEvent CauseType = "Event"
	// CauseTypeRequeueAfter is a reconcile that returned a RequeueAfter.
	CauseTypeRequeueAfter CauseType = "RequeueAfter"
	// CauseTypeRequeue is a reconcile that returned Requeue.
	CauseTypeRequeue CauseType = "Requeue"
	// CauseTypeError is a reconcile that returned an error, retried with backoff.
	CauseTypeError CauseType = "Error"
	// CauseTypeUnknown is an add the controller can't attribute, e.g. from a source that isn't backed by an informer.
	CauseTypeUnknown CauseType = "Unknown"
)

// Event is a watch event as an event handler saw it.
type Event struct {
	// Seq groups the enqueues of one event: an event mapped to several requests has one Seq.
	Seq uint64 `json:"seq"`
	// Type is Create, Update or Delete.
	Type   string    `json:"type"`
	Object ObjectRef `json:"object"`
	// OldResourceVersion is the resource version before an update.
	OldResourceVersion string `json:"oldResourceVersion,omitempty"`
	// GenerationChanged tells an update that changed the spec, for kinds that track a generation.
	GenerationChanged bool `json:"generationChanged,omitempty"`
	// Resync is an update with an unchanged resource version, delivered by the informer's periodic resync.
	Resync bool `json:"resync,omitempty"`
	// InInitialList is a create delivered by the informer's initial list.
	InInitialList bool `json:"inInitialList,omitempty"`
}

// Enqueue describes a RecordTypeEnqueue record.
type Enqueue struct {
	Cause CauseType `json:"cause"`
	// Event is set for CauseTypeEvent.
	Event *Event `json:"event,omitempty"`
	// ByReconcileID is the reconcile that requeued the request, for the requeue causes.
	ByReconcileID string        `json:"byReconcileID,omitempty"`
	After         time.Duration `json:"after,omitempty"`
	RateLimited   bool          `json:"rateLimited,omitempty"`
	Priority      *int          `json:"priority,omitempty"`
}

// ReconcileStart describes a RecordTypeReconcileStart record.
type ReconcileStart struct {
	// Triggers are the IDs of the enqueue records the workqueue coalesced into this reconcile.
	Triggers []uint64 `json:"triggers,omitempty"`
	// QueueWait is the time between the first of the triggers and the start of the reconcile.
	QueueWait time.Duration `json:"queueWait,omitempty"`
	Priority  int           `json:"priority,omitempty"`
}

// ReconcileEnd describes a RecordTypeReconcileEnd record.
type ReconcileEnd struct {
	Duration     time.Duration `json:"duration"`
	Requeue      bool          `json:"requeue,omitempty"`
	RequeueAfter time.Duration `json:"requeueAfter,omitempty"`
	Error        string        `json:"error,omitempty"`
	Terminal     bool          `json:"terminal,omitempty"`
	// ClientCalls is the number of client calls recorded for the reconcile.
	ClientCalls int `json:"clientCalls,omitempty"`
}

// ClientCall describes a RecordTypeClientCall record.
type ClientCall struct {
	// Client is the label the client was wrapped with, e.g. "client" or "apiReader".
	Client string `json:"client"`
	// Verb is one of get, list, create, update, patch, apply, delete and deleteAllOf.
	Verb        string    `json:"verb"`
	SubResource string    `json:"subResource,omitempty"`
	Object      ObjectRef `json:"object"`
	// ResourceVersionBefore is the resource version of the object passed to a write.
	ResourceVersionBefore string `json:"resourceVersionBefore,omitempty"`
	// ResourceVersionAfter is the resource version the API server returned for a write. For a subresource write with a
	// subresource body, e.g. a scale update, it is the body's.
	ResourceVersionAfter string `json:"resourceVersionAfter,omitempty"`
	// NoOp is an update or patch the API server accepted without persisting it: the resource version didn't change.
	NoOp bool `json:"noOp,omitempty"`
	// LabelSelector is the label selector of a list.
	LabelSelector string `json:"labelSelector,omitempty"`
	// Items is the number of items a list returned.
	Items    int           `json:"items,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	// ErrorReason is the API status reason of the error, e.g. Conflict or NotFound.
	ErrorReason string `json:"errorReason,omitempty"`
	// PatchType is the patch type of a patch.
	PatchType string `json:"patchType,omitempty"`
	// Body is the written object as the API server returned it, redacted, when the client records bodies.
	Body json.RawMessage `json:"body,omitempty"`
	// BodyTruncated tells a body dropped for exceeding the size limit.
	BodyTruncated bool `json:"bodyTruncated,omitempty"`
}

var lastID atomic.Uint64

// nextID returns the next record ID.
func nextID() uint64 {
	return lastID.Add(1)
}

// emit stamps r with an ID and time and records it.
func emit(rec Recorder, r Record) uint64 {
	r.ID = nextID()
	r.Time = time.Now()
	rec.Record(r)

	return r.ID
}

// ReconcileInfo identifies the reconcile a context belongs to.
type ReconcileInfo struct {
	Controller  string
	Request     string
	ReconcileID string
}

type reconcileInfoKey struct{}

// WithReconcile returns a context carrying info, so that the client calls made with it are attributed to the
// reconcile.
func WithReconcile(ctx context.Context, info ReconcileInfo) context.Context {
	return context.WithValue(ctx, reconcileInfoKey{}, info)
}

// ReconcileFromContext returns the reconcile ctx belongs to.
func ReconcileFromContext(ctx context.Context) (ReconcileInfo, bool) {
	info, ok := ctx.Value(reconcileInfoKey{}).(ReconcileInfo)
	return info, ok
}

// FormatRequest formats a request the way records carry it.
func FormatRequest(req any) string {
	return fmt.Sprint(req)
}

// objectRef returns a reference to obj, which is usually a metav1.Object. The kind is taken from obj's type name,
// because typed objects served by a cache have no TypeMeta.
func objectRef(obj any) ObjectRef {
	ref := ObjectRef{
		Kind: typeName(obj),
	}

	if o, ok := obj.(metav1.Object); ok {
		ref.Namespace = o.GetNamespace()
		ref.Name = o.GetName()
		ref.ResourceVersion = o.GetResourceVersion()
		ref.Generation = o.GetGeneration()
	}

	return ref
}

func typeName(obj any) string {
	t := reflect.TypeOf(obj)
	if t == nil {
		return ""
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}
