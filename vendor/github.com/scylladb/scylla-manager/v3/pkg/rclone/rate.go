// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
)

// SetRateLimit sets the global rate limit to a given amount of MiB per second.
// Set to 0 to for full throttle.
func SetRateLimit(mib int) {
	var bw fs.BwPair
	if mib > 0 {
		bw.Tx = fs.SizeSuffix(mib) * fs.MebiByte
		bw.Rx = fs.SizeSuffix(mib) * fs.MebiByte
	}
	accounting.TokenBucket.SetBwLimit(bw)
}
