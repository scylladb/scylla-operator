// Copyright (C) 2017 ScyllaDB

package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ctxt is a context key type.
type ctxt byte

// ctxt enumeration.
const (
	ctxTraceID ctxt = iota
	ctxFields
)

// WithFields assigns additional fields to the logging context.
// Existing fields are overridden and new fields are added.
func WithFields(ctx context.Context, keyvals ...interface{}) context.Context {
	if len(keyvals)%2 != 0 || len(keyvals) == 0 {
		return ctx
	}

	fields := make([]zapcore.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		// Consume this value and the next, treating them as a key-value pair.
		key, val := keyvals[i], keyvals[i+1]
		if keyStr, ok := key.(string); !ok {
			break
		} else {
			fields = append(fields, zap.Any(keyStr, val))
		}
	}

	var value []zapcore.Field
	v := ctx.Value(ctxFields)
	if v == nil {
		value = make([]zapcore.Field, 0, len(fields))
	} else {
		value = v.([]zapcore.Field)
	}
	for i := range fields {
		found := false
		for j := range value {
			if value[j].Key == fields[i].Key {
				value[j] = fields[i]
				found = true
				break
			}
		}
		if !found {
			value = append(value, fields[i])
		}
	}
	return context.WithValue(ctx, ctxFields, value)
}

// contextFields returns key-value pairs assigned to the context sorted by the
// key.
func contextFields(ctx context.Context) []zapcore.Field {
	if ctx == nil {
		return nil
	}
	v, ok := ctx.Value(ctxFields).([]zapcore.Field)
	if !ok {
		return nil
	}

	return v
}
