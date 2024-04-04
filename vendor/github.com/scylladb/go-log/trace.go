// Copyright (C) 2017 ScyllaDB

package log

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// WithTraceID ensures that the context has a trace ID, if not returns a new
// context with a random trace ID.
func WithTraceID(ctx context.Context) context.Context {
	if ctx.Value(ctxTraceID) != nil {
		return ctx
	}

	return WithNewTraceID(ctx)
}

// WithNewTraceID returns a new context with a random trace ID.
func WithNewTraceID(ctx context.Context) context.Context {
	v := zap.String("_trace_id", newTraceID())
	return context.WithValue(ctx, ctxTraceID, &v)
}

// CopyTraceID allows for copying the trace ID from a context to another context.
func CopyTraceID(ctx, from context.Context) context.Context {
	v, ok := from.Value(ctxTraceID).(*zapcore.Field)
	if !ok {
		return ctx
	}

	return context.WithValue(ctx, ctxTraceID, v)
}

// TraceID returns trace ID of the context.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx.Value(ctxTraceID).(*zapcore.Field)
	if !ok {
		return ""
	}
	return v.String
}

func newTraceID() string {
	var uuid [16]byte
	_, err := io.ReadFull(rand.Reader, uuid[:])
	if err != nil {
		return ""
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	return base64.RawURLEncoding.EncodeToString(uuid[:])
}
