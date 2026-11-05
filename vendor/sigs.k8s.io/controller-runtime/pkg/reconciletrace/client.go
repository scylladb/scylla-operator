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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientOptions tune what a wrapped client records.
type ClientOptions struct {
	// Label tells the client apart in the records, e.g. "client" for the cached client and "apiReader" for the live
	// one. Defaults to "client".
	Label string
	// SkipReads skips recording gets and lists.
	SkipReads bool
	// RecordBodies records the objects written, as the API server returned them. Managed fields are dropped and
	// Secret data is redacted.
	RecordBodies bool
	// MaxBodyBytes drops a recorded body larger than this. Defaults to 64KiB.
	MaxBodyBytes int
}

func (o ClientOptions) withDefaults() ClientOptions {
	if o.Label == "" {
		o.Label = "client"
	}
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = 64 << 10
	}

	return o
}

// WrapClient returns a client recording every call made with a reconcile's context to recorder. Calls made outside
// a reconcile are recorded without a reconcile ID.
func WrapClient(c client.Client, recorder Recorder, opts ClientOptions) client.Client {
	return &tracingClient{
		Client:   c,
		recorder: recorder,
		opts:     opts.withDefaults(),
	}
}

// WrapReader returns a reader recording every call to recorder, like WrapClient.
func WrapReader(r client.Reader, recorder Recorder, opts ClientOptions) client.Reader {
	return &tracingReader{
		Reader: r,
		tracer: tracer{
			recorder: recorder,
			opts:     opts.withDefaults(),
		},
	}
}

type tracer struct {
	recorder Recorder
	opts     ClientOptions
	// kindFor resolves the kind of an object, when the client can tell it.
	kindFor func(obj runtime.Object) string
}

func (t tracer) kind(obj runtime.Object) string {
	if t.kindFor != nil {
		if kind := t.kindFor(obj); kind != "" {
			return kind
		}
	}

	return typeName(obj)
}

// record records a call to obj that started at start and returned err.
func (t tracer) record(ctx context.Context, call ClientCall, obj runtime.Object, start time.Time, err error) {
	call.Client = t.opts.Label
	call.Duration = time.Since(start)
	if obj != nil {
		ref := objectRef(obj)
		ref.Kind = t.kind(obj)
		call.Object = ref
	}
	if err != nil {
		call.Error = err.Error()
		call.ErrorReason = string(apierrors.ReasonForError(err))
	}

	countCall(ctx)
	info, _ := ReconcileFromContext(ctx)
	emit(t.recorder, Record{
		Type:        RecordTypeClientCall,
		Controller:  info.Controller,
		Request:     info.Request,
		ReconcileID: info.ReconcileID,
		ClientCall:  &call,
	})
}

// recordWrite records a write of obj. sent is the object the API server's response is decoded into: obj, or the
// subresource body; its resource version was rvBefore when it was sent.
func (t tracer) recordWrite(ctx context.Context, call ClientCall, obj, sent client.Object, rvBefore string, start time.Time, err error) {
	call.ResourceVersionBefore = rvBefore
	if err == nil && call.Verb != "delete" {
		call.ResourceVersionAfter = sent.GetResourceVersion()
	}
	if err == nil && (call.Verb == "update" || call.Verb == "patch") {
		call.NoOp = rvBefore != "" && rvBefore == call.ResourceVersionAfter
	}
	if err == nil && t.opts.RecordBodies && call.Verb != "delete" {
		call.Body, call.BodyTruncated = t.body(sent)
	}

	t.record(ctx, call, obj, start, err)
}

// body marshals obj for the record, without its managed fields and with Secret data redacted.
func (t tracer) body(obj client.Object) (json.RawMessage, bool) {
	obj = obj.DeepCopyObject().(client.Object)
	obj.SetManagedFields(nil)

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, false
	}
	if t.kind(obj) == "Secret" {
		for _, field := range []string{"data", "stringData"} {
			if values, ok := u[field].(map[string]any); ok {
				for k := range values {
					values[k] = "REDACTED"
				}
			}
		}
	}

	b, err := json.Marshal(u)
	if err != nil {
		return nil, false
	}
	if len(b) > t.opts.MaxBodyBytes {
		return nil, true
	}

	return b, false
}

type tracingReader struct {
	client.Reader
	tracer
}

func (r *tracingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	start := time.Now()
	err := r.Reader.Get(ctx, key, obj, opts...)
	if !r.opts.SkipReads {
		ref := obj
		if err != nil {
			// The object isn't filled in; record the key that was asked for.
			ref = obj.DeepCopyObject().(client.Object)
			ref.SetNamespace(key.Namespace)
			ref.SetName(key.Name)
		}
		r.record(ctx, ClientCall{Verb: "get"}, ref, start, err)
	}

	return err
}

func (r *tracingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	start := time.Now()
	err := r.Reader.List(ctx, list, opts...)
	if !r.opts.SkipReads {
		r.recordList(ctx, list, opts, start, err)
	}

	return err
}

func (t tracer) recordList(ctx context.Context, list client.ObjectList, opts []client.ListOption, start time.Time, err error) {
	listOpts := (&client.ListOptions{}).ApplyOptions(opts)
	call := ClientCall{
		Verb: "list",
		Object: ObjectRef{
			Kind:      strings.TrimSuffix(t.kind(list), "List"),
			Namespace: listOpts.Namespace,
		},
		Items: meta.LenList(list),
	}
	if listOpts.LabelSelector != nil {
		call.LabelSelector = listOpts.LabelSelector.String()
	}

	t.record(ctx, call, nil, start, err)
}

type tracingClient struct {
	client.Client
	recorder Recorder
	opts     ClientOptions
}

func (c *tracingClient) tracer() tracer {
	return tracer{
		recorder: c.recorder,
		opts:     c.opts,
		kindFor: func(obj runtime.Object) string {
			gvk, err := c.GroupVersionKindFor(obj)
			if err != nil {
				return ""
			}
			return strings.TrimSuffix(gvk.Kind, "List")
		},
	}
}

func (c *tracingClient) reader() *tracingReader {
	return &tracingReader{Reader: c.Client, tracer: c.tracer()}
}

func (c *tracingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.reader().Get(ctx, key, obj, opts...)
}

func (c *tracingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.reader().List(ctx, list, opts...)
}

func (c *tracingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	start := time.Now()
	err := c.Client.Create(ctx, obj, opts...)
	c.tracer().recordWrite(ctx, ClientCall{Verb: "create"}, obj, obj, "", start, err)

	return err
}

func (c *tracingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	rv := obj.GetResourceVersion()
	start := time.Now()
	err := c.Client.Update(ctx, obj, opts...)
	c.tracer().recordWrite(ctx, ClientCall{Verb: "update"}, obj, obj, rv, start, err)

	return err
}

func (c *tracingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	rv := obj.GetResourceVersion()
	start := time.Now()
	err := c.Client.Patch(ctx, obj, patch, opts...)
	c.tracer().recordWrite(ctx, ClientCall{Verb: "patch", PatchType: string(patch.Type())}, obj, obj, rv, start, err)

	return err
}

func (c *tracingClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	start := time.Now()
	err := c.Client.Apply(ctx, obj, opts...)
	c.tracer().record(ctx, ClientCall{Verb: "apply", Object: ObjectRef{Kind: typeName(obj)}}, nil, start, err)

	return err
}

func (c *tracingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	rv := obj.GetResourceVersion()
	start := time.Now()
	err := c.Client.Delete(ctx, obj, opts...)
	c.tracer().recordWrite(ctx, ClientCall{Verb: "delete"}, obj, obj, rv, start, err)

	return err
}

func (c *tracingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	start := time.Now()
	err := c.Client.DeleteAllOf(ctx, obj, opts...)
	c.tracer().record(ctx, ClientCall{Verb: "deleteAllOf"}, obj, start, err)

	return err
}

func (c *tracingClient) Status() client.SubResourceWriter {
	return &tracingSubResourceClient{
		SubResourceClient: c.Client.SubResource("status"),
		writer:            c.Client.Status(),
		subResource:       "status",
		tracer:            c.tracer(),
	}
}

func (c *tracingClient) SubResource(subResource string) client.SubResourceClient {
	sc := c.Client.SubResource(subResource)

	return &tracingSubResourceClient{
		SubResourceClient: sc,
		writer:            sc,
		subResource:       subResource,
		tracer:            c.tracer(),
	}
}

// tracingSubResourceClient records the writes to a subresource. Reads of a subresource are rare and not recorded.
type tracingSubResourceClient struct {
	client.SubResourceClient
	writer      client.SubResourceWriter
	subResource string
	tracer      tracer
}

// The subresource writes decode the API server's response into the subresource body when one is passed, e.g. the
// Scale of a scale update, and leave obj as it was sent. The resource version after the write is read from the body.

func (s *tracingSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	start := time.Now()
	err := s.writer.Create(ctx, obj, subResource, opts...)
	s.tracer.recordWrite(ctx, ClientCall{Verb: "create", SubResource: s.subResource}, obj, subResource, "", start, err)

	return err
}

func (s *tracingSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	sent := obj
	if body := (&client.SubResourceUpdateOptions{}).ApplyOptions(opts).SubResourceBody; body != nil {
		sent = body
	}
	rv := sent.GetResourceVersion()
	start := time.Now()
	err := s.writer.Update(ctx, obj, opts...)
	s.tracer.recordWrite(ctx, ClientCall{Verb: "update", SubResource: s.subResource}, obj, sent, rv, start, err)

	return err
}

func (s *tracingSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	sent := obj
	if body := (&client.SubResourcePatchOptions{}).ApplyOptions(opts).SubResourceBody; body != nil {
		sent = body
	}
	rv := sent.GetResourceVersion()
	start := time.Now()
	err := s.writer.Patch(ctx, obj, patch, opts...)
	s.tracer.recordWrite(ctx, ClientCall{Verb: "patch", SubResource: s.subResource, PatchType: string(patch.Type())}, obj, sent, rv, start, err)

	return err
}

var _ client.Client = &tracingClient{}
var _ client.Reader = &tracingReader{}
