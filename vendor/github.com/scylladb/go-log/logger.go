// Copyright (C) 2017 ScyllaDB

package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger logs messages.
type Logger struct {
	base       *zap.Logger
	baseFields []zapcore.Field
}

// NewLogger creates a new logger backed by a zap.Logger.
func NewLogger(base *zap.Logger) Logger {
	return Logger{base: base}
}

// NopLogger doesn't log anything.
var NopLogger = Logger{}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (l Logger) Named(name string) Logger {
	if l.base == nil {
		return l
	}
	return Logger{base: l.base.Named(name)}
}

// WithOptions clones the current Logger, applies the supplied Options, and
// returns the resulting Logger. It's safe to use concurrently.
func (l Logger) WithOptions(opts ...zap.Option) Logger {
	if l.base == nil {
		return l
	}
	return Logger{base: l.base.WithOptions(opts...)}
}

// With adds a variadic number of fields to the logging context.
func (l Logger) With(keyvals ...interface{}) Logger {
	if l.base == nil {
		return l
	}
	fields := l.zapify(context.Background(), keyvals)
	baseFields := append(l.baseFields, fields...)
	return Logger{base: l.base.With(fields...), baseFields: baseFields}
}

// Sync flushes any buffered log entries. Applications should take care to call
// Sync before exiting.
func (l Logger) Sync() error {
	if l.base == nil {
		return nil
	}
	return l.base.Sync()
}

// Debug logs a message with some additional context.
func (l Logger) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log(ctx, zapcore.DebugLevel, msg, keyvals)
}

// Info logs a message with some additional context.
func (l Logger) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log(ctx, zapcore.InfoLevel, msg, keyvals)
}

// Error logs a message with some additional context.
func (l Logger) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log(ctx, zapcore.ErrorLevel, msg, keyvals)
}

// Fatal logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func (l Logger) Fatal(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log(ctx, zapcore.FatalLevel, msg, keyvals)
}

func (l Logger) log(ctx context.Context, lvl zapcore.Level, msg string, keyvals []interface{}) {
	if l.base == nil {
		return
	}
	if !l.base.Core().Enabled(lvl) {
		return
	}

	if ce := l.base.Check(lvl, msg); ce != nil {
		ce.Write(l.zapify(ctx, keyvals)...)
	}
}

// Filed ordering: logger fields > context fields > passed fields > trace_id.
func (l Logger) zapify(ctx context.Context, keyvals []interface{}) []zapcore.Field {
	if len(keyvals)%2 != 0 {
		l.base.DPanic("odd number of elements")
		return nil
	}

	var (
		extraFields int
		extra       []zapcore.Field
		trace       *zapcore.Field
		ok          bool
	)

	if ctx != nil {
		trace, ok = ctx.Value(ctxTraceID).(*zapcore.Field)
		if ok {
			extraFields++
		}
		extra = Fields(ctx)
		extraFields += len(extra)
	}

	if len(keyvals)+extraFields == 0 {
		return nil
	}

	fields := make([]zapcore.Field, 0, len(keyvals)/2+extraFields)

	if len(extra) > 0 {
		// Exclude fields that are set by calling With on logger.
		for i := range extra {
			if containsKey(l.baseFields, extra[i].Key) > -1 {
				continue
			}
			fields = append(fields, extra[i])
		}
	}

	for i := 0; i < len(keyvals); i += 2 {
		// Consume this value and the next, treating them as a key-value pair.
		key, val := keyvals[i], keyvals[i+1]

		if keyStr, ok := key.(string); !ok {
			l.base.DPanic("key not a string", zap.Any("key", key))
			break
		} else {
			j := containsKey(fields, keyStr)
			if j > -1 {
				fields[j] = zap.Any(keyStr, val)
				continue
			}
			fields = append(fields, zap.Any(keyStr, val))
		}
	}

	if trace != nil {
		fields = append(fields, *trace)
	}

	return fields
}

func containsKey(fields []zapcore.Field, key string) int {
	for i := range fields {
		if fields[i].Key == key {
			return i
		}
	}
	return -1
}

// BaseOf unwraps l and returns the base zap.Logger.
func BaseOf(l Logger) *zap.Logger {
	return l.base
}
