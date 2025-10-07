// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/util/algorithms"
	"github.com/scylladb/scylla-operator/pkg/util/blkutils"
	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	"k8s.io/utils/exec"
)

// MakeFS creates a filesystem of type fsType on the given device with the specified block size.
// It returns a boolean indicating whether there were any changes made (i.e., if the filesystem was created).
func MakeFS(ctx context.Context, executor exec.Interface, device string, blockSize int, fsType string) (bool, error) {
	existingFs, err := blkutils.GetFilesystemType(ctx, executor, device)
	if err != nil {
		return false, fmt.Errorf("can't determine existing filesystem type at %q: %w", device, err)
	}

	if existingFs == fsType {
		return false, nil
	}
	if len(existingFs) > 0 {
		return false, fmt.Errorf("can't format device %q with filesystem %q: already formatted with conflicting filesystem %q", device, fsType, existingFs)
	}

	// The minimum block size for crc enabled filesystems is 1024, and it also cannot be smaller than the logical block size.
	blockSize = algorithms.Max(1024, blockSize)

	args := []string{
		// filesystem type
		"-t", fsType,
		// block size
		"-b", fmt.Sprintf("size=%d", blockSize),
		// no discard
		"-K",
		device,
	}
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "mkfs", args...)
	if err != nil {
		return false, fmt.Errorf("can't run mkfs with args %v: %w, stdout: %q, stderr: %q", args, err, stdout.String(), stderr.String())
	}

	return true, nil
}
