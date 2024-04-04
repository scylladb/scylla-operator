// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"context"

	"github.com/rclone/rclone/fs/accounting"
)

// StartAccountingOperations starts token bucket and transaction limiter
// tracking.
func StartAccountingOperations() {
	// Start the token bucket limiter
	accounting.TokenBucket.StartTokenBucket(context.Background())
	// Start the bandwidth update ticker
	accounting.TokenBucket.StartTokenTicker(context.Background())
	// Start the transactions per second limiter
	accounting.StartLimitTPS(context.Background())
}
