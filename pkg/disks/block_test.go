// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestGetBlockSize(t *testing.T) {
	t.Parallel()

	makeSysfs := func(blockSize int) func(string) string {
		return func(tempDir string) string {
			stat := syscall.Stat_t{}
			err := syscall.Stat(tempDir, &stat)
			if err != nil {
				t.Fatal(err)
			}

			major, minor := stat.Rdev/256, stat.Rdev%256
			deviceQueuePath := path.Join(tempDir, fmt.Sprintf("/sys/dev/block/%d:%d/queue", major, minor))
			err = os.MkdirAll(deviceQueuePath, os.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(path.Join(deviceQueuePath, "logical_block_size"), []byte(fmt.Sprintf("%d", blockSize)), os.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			return path.Join(tempDir, "/sys")
		}
	}

	makeFakeDevice := func(blockSize int) func(string) string {
		return func(tempDir string) string {
			devPath := "/dev/md0"

			err := os.MkdirAll(path.Join(tempDir, devPath), os.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			return path.Join(tempDir, devPath)
		}
	}

	tt := []struct {
		name              string
		makeDevice        func(string) string
		makeSysfs         func(string) string
		expectedBlockSize int
		expectedError     error
	}{
		{
			name: "error when disk path doesn't exists",
			makeDevice: func(string) string {
				return "/path/that/does/not/exists/dev/md0"
			},
			makeSysfs:         func(tempDir string) string { return tempDir },
			expectedBlockSize: 0,
			expectedError:     fmt.Errorf(`can't stat device "/path/that/does/not/exists/dev/md0": %w`, unix.ENOENT),
		},
		{
			name:              "returns device logical block size from host sysfs",
			makeDevice:        makeFakeDevice(424242),
			makeSysfs:         makeSysfs(424242),
			expectedBlockSize: 424242,
			expectedError:     nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-get-block-size-")
			if err != nil {
				t.Fatal(err)
			}

			sysfs := tc.makeSysfs(tempDir)
			device := tc.makeDevice(tempDir)
			blockSize, err := GetBlockSize(device, sysfs)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %#v error, got %#v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(blockSize, tc.expectedBlockSize) {
				t.Fatalf("expected %d block size, got %d", tc.expectedBlockSize, blockSize)
			}
		})
	}
}
