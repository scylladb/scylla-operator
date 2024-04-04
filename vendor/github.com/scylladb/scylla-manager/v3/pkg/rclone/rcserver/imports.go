// Copyright (C) 2017 ScyllaDB

package rcserver

// Needed for triggering global registrations in rclone.
import (
	_ "github.com/rclone/rclone/backend/azureblob"
	_ "github.com/rclone/rclone/backend/googlecloudstorage"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/fs/accounting"
	_ "github.com/rclone/rclone/fs/operations"
	_ "github.com/rclone/rclone/fs/rc/jobs"
	_ "github.com/rclone/rclone/fs/sync"
)
