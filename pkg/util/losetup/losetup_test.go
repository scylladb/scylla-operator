// Copyright (c) 2024 ScyllaDB.

package losetup

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
	testingexec "k8s.io/utils/exec/testing"
)

func TestListLoopDevices(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		commands            []exectest.Command
		expectedLoopDevices []*LoopDevice
		expectedError       error
	}{
		{
			name: "losetup fails with random error",
			commands: []exectest.Command{
				{
					Cmd:    "losetup",
					Args:   []string{"--all", "--list", "--json"},
					Stdout: []byte("stdout output"),
					Stderr: []byte("stderr error"),
					Err:    testingexec.FakeExitError{Status: 666},
				},
			},
			expectedLoopDevices: nil,
			expectedError:       fmt.Errorf(`failed to run losetup with args: [--all --list --json]: %w, stdout: "stdout output", stderr: "stderr error"`, testingexec.FakeExitError{Status: 666}),
		},
		{
			name: "losetup returns empty list when losetup doesn't output anything",
			commands: []exectest.Command{
				{
					Cmd:    "losetup",
					Args:   []string{"--all", "--list", "--json"},
					Stdout: []byte(""),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedLoopDevices: []*LoopDevice{},
			expectedError:       nil,
		},
		{
			name: "losetup returns empty list when losetup returns empty array of loop devices",
			commands: []exectest.Command{
				{
					Cmd:    "losetup",
					Args:   []string{"--all", "--list", "--json"},
					Stdout: []byte(fmt.Sprintf(`{"loopdevices":[]}`)),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedLoopDevices: []*LoopDevice{},
			expectedError:       nil,
		},
		{
			name: "losetup returns loop devices from json output",
			commands: []exectest.Command{
				{
					Cmd:    "losetup",
					Args:   []string{"--all", "--list", "--json"},
					Stdout: []byte(fmt.Sprintf(`{"loopdevices":[{"name":"/dev/loop0","back-file":"/mnt/disk0.img"},{"name":"/dev/loop1","back-file":"/mnt/disk1.img"}]}`)),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedLoopDevices: []*LoopDevice{
				{
					Name:        "/dev/loop0",
					BackingFile: "/mnt/disk0.img",
				},
				{
					Name:        "/dev/loop1",
					BackingFile: "/mnt/disk1.img",
				},
			},
			expectedError: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			loopDevices, err := ListLoopDevices(ctx, executor)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(loopDevices, tc.expectedLoopDevices) {
				t.Fatalf("expected %v, got %v", tc.expectedLoopDevices, loopDevices)
			}
		})
	}
}
