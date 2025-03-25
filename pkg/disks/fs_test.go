// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
)

func TestMakeFS(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name             string
		device           string
		blockSize        int
		fsType           string
		expectedCommands []exectest.Command
		expectedFinished bool
		expectedErr      error
	}{
		{
			name:      "nothing to do if device is already formatted to required filesystem",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/md0","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:      "fail if there's already a different filesystem",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/md0","model":"Amazon EC2 NVMe Instance Storage","fstype":"ext4","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedFinished: false,
			expectedErr:      fmt.Errorf(`can't format device "/dev/md0" with filesystem "xfs": already formatted with conflicting filesystem "ext4"`),
		},
		{
			name:      "format the device if existing disk filesystem is empty",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/md0","model":"Amazon EC2 NVMe Instance Storage","fstype":"","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`),
					Stderr: nil,
					Err:    nil,
				},
				{
					Cmd:    "mkfs",
					Args:   []string{"-t", "xfs", "-b", "size=1024", "-K", "/dev/md0"},
					Stdout: nil,
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.expectedCommands...)

			finished, err := MakeFS(ctx, executor, tc.device, tc.blockSize, tc.fsType, nil)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(finished, tc.expectedFinished) {
				t.Fatalf("expected finished %v, got %v", tc.expectedFinished, finished)
			}
			if executor.CommandCalls != len(tc.expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(tc.expectedCommands), executor.CommandCalls)
			}
		})
	}
}
