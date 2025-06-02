package blkutils_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/blkutils"
	"github.com/scylladb/scylla-operator/pkg/util/exectest"
	testingexec "k8s.io/utils/exec/testing"
)

func TestGetPartitionUUID(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                  string
		device                string
		commands              []exectest.Command
		expectedPartitionUUID string
		expectedError         error
	}{
		{
			name:   "lsblk fails with random error",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte("stdout error"),
					Stderr: []byte("stderr error"),
					Err:    testingexec.FakeExitError{Status: 666},
				},
			},
			expectedPartitionUUID: "",
			expectedError:         fmt.Errorf(`can't get block device "/dev/md0": %w`, fmt.Errorf(`can't list block device "/dev/md0": %w`, fmt.Errorf(`failed to run lsblk with args: [--json --nodeps --paths --fs --output=NAME,MODEL,FSTYPE,PARTUUID /dev/md0]: %w, stdout: "stdout error", stderr: "stderr error"`, testingexec.FakeExitError{Status: 666}))),
		},
		{
			name:   "found block device partition UUID",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/nvme1n1","model":"Amazon EC2 NVMe Instance Storage","fstype":"linux_raid_member","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`),
				},
			},
			expectedPartitionUUID: "1bbeb48b-101f-4d49-8ba4-67adc9878721",
			expectedError:         nil,
		},
		{
			name:   "empty block device partition UUID",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/nvme1n1","model":"Amazon EC2 NVMe Instance Storage","fstype":"linux_raid_member","partuuid":null}]}`),
				},
			},
			expectedPartitionUUID: "",
			expectedError:         nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			partitionUUID, err := blkutils.GetPartitionUUID(ctx, executor, tc.device)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(partitionUUID, tc.expectedPartitionUUID) {
				t.Fatalf("expected %v, got %v", tc.expectedPartitionUUID, partitionUUID)
			}
		})
	}
}

func TestIsBlockDevice(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		device              string
		commands            []exectest.Command
		expectedBlockDevice bool
		expectedError       error
	}{
		{
			name:   "lsblk fails with random error",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"/dev/md0"},
					Stdout: []byte("stdout error"),
					Stderr: []byte("stderr error"),
					Err:    testingexec.FakeExitError{Status: 666},
				},
			},
			expectedBlockDevice: false,
			expectedError:       fmt.Errorf(`failed to run lsblk with args: /dev/md0: %w, stdout: "stdout error", stderr: "stderr error"`, testingexec.FakeExitError{Status: 666}),
		},
		{
			name:   "not a block device when special error code is returned",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:  "lsblk",
					Args: []string{"/dev/md0"},
					Err:  testingexec.FakeExitError{Status: 32},
				},
			},
			expectedBlockDevice: false,
			expectedError:       nil,
		},
		{
			name:   "block device when command is successful",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:  "lsblk",
					Args: []string{"/dev/md0"},
					Err:  nil,
				},
			},
			expectedBlockDevice: true,
			expectedError:       nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			blockDevice, err := blkutils.IsBlockDevice(ctx, executor, tc.device)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(blockDevice, tc.expectedBlockDevice) {
				t.Fatalf("expected %v, got %v", tc.expectedBlockDevice, blockDevice)
			}
		})
	}
}

func TestGetBlockDevice(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		device         string
		commands       []exectest.Command
		expectedDevice *blkutils.BlockDevice
		expectedError  error
	}{
		{
			name:   "lsblk fails with error",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte("stdout error"),
					Stderr: []byte("stderr error"),
					Err:    testingexec.FakeExitError{Status: 32},
				},
			},
			expectedDevice: nil,
			expectedError:  fmt.Errorf(`can't list block device "/dev/md0": %w`, fmt.Errorf(`failed to run lsblk with args: [--json --nodeps --paths --fs --output=NAME,MODEL,FSTYPE,PARTUUID /dev/md0]: %w, stdout: "stdout error", stderr: "stderr error"`, testingexec.FakeExitError{Status: 32})),
		},
		{
			name:   "device not found",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/md0"},
					Stdout: []byte(`{}`),
				},
			},
			expectedDevice: nil,
			expectedError:  fmt.Errorf(`device "/dev/md0" not found`),
		},
		{
			name:   "block device found",
			device: "/dev/nvme1n1",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", "/dev/nvme1n1"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/nvme1n1","model":"Amazon EC2 NVMe Instance Storage","fstype":"linux_raid_member","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`),
				},
			},
			expectedDevice: &blkutils.BlockDevice{
				Name:     "/dev/nvme1n1",
				Model:    "Amazon EC2 NVMe Instance Storage",
				FSType:   "linux_raid_member",
				PartUUID: "1bbeb48b-101f-4d49-8ba4-67adc9878721",
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			devices, err := blkutils.GetBlockDevice(ctx, executor, tc.device)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(devices, tc.expectedDevice) {
				t.Fatalf("expected %v device, got %v", tc.expectedDevice, devices)
			}
		})
	}
}

func TestListBlockDevices(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		commands        []exectest.Command
		expectedDevices []*blkutils.BlockDevice
		expectedError   error
	}{
		{
			name: "no block devices field",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID"},
					Stdout: []byte(`{}`),
				},
			},
			expectedDevices: nil,
			expectedError:   nil,
		},
		{
			name: "empty list of block devices",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID"},
					Stdout: []byte(`{"blockDevices":[]}`),
				},
			},
			expectedDevices: nil,
			expectedError:   nil,
		},
		{
			name: "block devices found",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID"},
					Stdout: []byte(`{"blockdevices":[{"name":"/dev/nvme1n1","model":"Amazon EC2 NVMe Instance Storage","fstype":"linux_raid_member","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"},{"name":"/dev/nvme0n1","model":"Amazon Elastic Block Store","fstype":null,"partuuid":null}]}`),
				},
			},
			expectedDevices: []*blkutils.BlockDevice{
				{
					Name:     "/dev/nvme1n1",
					Model:    "Amazon EC2 NVMe Instance Storage",
					FSType:   "linux_raid_member",
					PartUUID: "1bbeb48b-101f-4d49-8ba4-67adc9878721",
				},
				{
					Name:     "/dev/nvme0n1",
					Model:    "Amazon Elastic Block Store",
					FSType:   "",
					PartUUID: "",
				},
			},
			expectedError: nil,
		},
		{
			name: "lsblk fails with error",
			commands: []exectest.Command{
				{
					Cmd:    "lsblk",
					Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID"},
					Stdout: []byte("stdout error"),
					Stderr: []byte("stderr error"),
					Err:    testingexec.FakeExitError{Status: 666},
				},
			},
			expectedDevices: nil,
			expectedError:   fmt.Errorf(`failed to run lsblk with args: [--json --nodeps --paths --fs --output=NAME,MODEL,FSTYPE,PARTUUID]: %w, stdout: "stdout error", stderr: "stderr error"`, testingexec.FakeExitError{Status: 666}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			devices, err := blkutils.ListBlockDevices(ctx, executor)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(devices, tc.expectedDevices) {
				t.Fatalf("expected %v, got %v", tc.expectedDevices, devices)
			}
		})
	}
}
