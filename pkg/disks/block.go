// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func GetBlockSize(device string, sysfsPath string) (int, error) {
	var stat syscall.Stat_t
	err := syscall.Stat(device, &stat)
	if err != nil {
		return 0, fmt.Errorf("can't stat device %q: %w", device, err)
	}

	major, minor := unix.Major(stat.Rdev), unix.Minor(stat.Rdev)
	blockSizeRaw, err := os.ReadFile(path.Join(sysfsPath, fmt.Sprintf("/dev/block/%d:%d/queue/logical_block_size", major, minor)))
	if err != nil {
		return 0, fmt.Errorf("can't read device %d:%d logical block size file: %w", major, minor, err)
	}

	blockSize, err := strconv.Atoi(strings.TrimSpace(string(blockSizeRaw)))
	if err != nil {
		return 0, fmt.Errorf("can't parse block size %q: %w", string(blockSizeRaw), err)
	}

	return blockSize, nil
}
