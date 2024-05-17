// Copyright (C) 2024 ScyllaDB

package logutil

import (
	"context"

	"github.com/scylladb/go-log"
)

// LogOnError logs provided msg and keyvals only when f results encounters error.
// It's useful for logging resource cleanup errors that don't directly influence function execution.
func LogOnError(ctx context.Context, log log.Logger, f func() error, msg string, keyvals ...interface{}) {
	if err := f(); err != nil {
		log.Error(ctx, msg, append(append([]any{}, keyvals...), "error", err)...)
	}
}
