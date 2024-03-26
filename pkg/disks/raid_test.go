// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
)

func TestMakeRAID0(t *testing.T) {
	t.Parallel()

	makeNotExistingDevice := func(sysfsPath, deviceName string) string {
		tempDir, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-")
		if err != nil {
			t.Fatal(err)
		}

		devPath := path.Join(tempDir, "dev")
		err = os.MkdirAll(devPath, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		return path.Join(devPath, deviceName)
	}

	makeExistingDevice := func(sysfsPath, deviceName string) string {
		devicePath := makeNotExistingDevice(sysfsPath, deviceName)

		f, err := os.Create(devicePath)
		if err != nil {
			t.Fatal(err)
		}

		err = f.Close()
		if err != nil {
			t.Fatal(err)
		}

		return devicePath
	}

	makeRaidDevice := func(deviceName, raidLevel string, slaves []string) func(string) string {
		return func(sysfsPath string) string {
			raidDevice := makeExistingDevice(sysfsPath, deviceName)

			if len(raidLevel) != 0 {
				err := os.MkdirAll(path.Join(sysfsPath, fmt.Sprintf("/block/%s/md/", deviceName)), os.ModePerm)
				if err != nil {
					t.Fatal(err)
				}

				raidLevelPath := path.Join(sysfsPath, fmt.Sprintf("/block/%s/md/level", deviceName))
				err = os.WriteFile(raidLevelPath, []byte(raidLevel), os.ModePerm)
				if err != nil {
					t.Fatal(err)
				}

				for _, slave := range slaves {
					slaveDirPath := path.Join(sysfsPath, fmt.Sprintf("/block/%s/slaves/%s", deviceName, slave))
					err = os.MkdirAll(slaveDirPath, os.ModePerm)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			return raidDevice
		}
	}

	discardableDevice := func(sysfsPath, deviceName string) string {
		devicePath := makeExistingDevice(sysfsPath, deviceName)

		diskQueueDir := path.Join(sysfsPath, fmt.Sprintf("/block/%s/queue", deviceName))
		err := os.MkdirAll(diskQueueDir, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path.Join(diskQueueDir, "discard_granularity"), []byte("1"), os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		return devicePath
	}

	tt := []struct {
		name             string
		makeRaidDevice   func(sysfsPath string) string
		makeDevices      func(sysfsPath string) []string
		udevEnabled      bool
		expectedCommands func(string, []string) []exectest.Command
		expectedChanged  bool
		expectedErr      error
	}{
		{
			name:           "nothing to do when raid device already exists",
			makeRaidDevice: makeRaidDevice("md0", "raid0", []string{"loop0", "loop1"}),
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "loop0"),
					discardableDevice(sysfsPath, "loop1"),
				}
			},
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return nil
			},
			expectedChanged: false,
			expectedErr:     nil,
		},
		{
			name: "makes a RAID0 from provided devices",
			makeRaidDevice: func(sysfsPath string) string {
				return makeNotExistingDevice(sysfsPath, "md0")
			},
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
					discardableDevice(sysfsPath, "nvme2n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[1]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--homehost=<none>", fmt.Sprintf("--name=%s", raidDevice), "--raid-devices=2", devices[0], devices[1]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name: "forces a RAID0 when there's just one device",
			makeRaidDevice: func(sysfsPath string) string {
				return makeNotExistingDevice(sysfsPath, "md0")
			},
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--homehost=<none>", fmt.Sprintf("--name=%s", raidDevice), "--raid-devices=1", devices[0], "--force"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name: "skips the device discard if it doesn't requires it",
			makeRaidDevice: func(sysfsPath string) string {
				return makeNotExistingDevice(sysfsPath, "md0")
			},
			makeDevices: func(sysfsPath string) []string {
				return []string{
					makeExistingDevice(sysfsPath, "nvme1n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--homehost=<none>", fmt.Sprintf("--name=%s", raidDevice), "--raid-devices=1", devices[0], "--force"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name: "skips udev control stop/start when it's not supported",
			makeRaidDevice: func(sysfsPath string) string {
				return makeNotExistingDevice(sysfsPath, "md0")
			},
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
				}
			},
			udevEnabled: false,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--homehost=<none>", fmt.Sprintf("--name=%s", raidDevice), "--raid-devices=1", devices[0], "--force"},
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sysfsPath, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-sysfs")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(sysfsPath)

			devtmpfsPath, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-devtmpfs")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(devtmpfsPath)

			raidDevice := tc.makeRaidDevice(sysfsPath)
			devices := tc.makeDevices(sysfsPath)
			expectedCommands := tc.expectedCommands(raidDevice, devices)
			executor := exectest.NewFakeExec(expectedCommands...)

			changed, err := MakeRAID0(ctx, executor, sysfsPath, devtmpfsPath, raidDevice, devices, tc.udevEnabled)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(changed, tc.expectedChanged) {
				t.Fatalf("expected %v changed, got %v", tc.expectedChanged, changed)
			}
			if executor.CommandCalls != len(expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(expectedCommands), executor.CommandCalls)
			}
		})
	}
}

func TestGetRAIDDeviceWithName(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name               string
		raidName           func(devtmpfsPath string) string
		makeDevtmpfs       func(devtmpfsPath string) error
		expectedCommands   func(devtmpfsPath string) []exectest.Command
		expectedRaidDevice func(devtmpfsPath string) string
		expectedErr        error
	}{
		{
			name: "returns the raid device if provided as a reference to real device",
			raidName: func(devtmpfsPath string) string {
				return path.Join(devtmpfsPath, "foo")
			},
			makeDevtmpfs: func(devtmpfsPath string) error {
				f, err := os.Create(path.Join(devtmpfsPath, "foo"))
				if err != nil {
					return err
				}

				err = f.Close()
				if err != nil {
					return err
				}

				return nil
			},
			expectedCommands: func(devtmpfsPath string) []exectest.Command {
				return nil
			},
			expectedRaidDevice: func(devtmpfsPath string) string {
				return path.Join(devtmpfsPath, "/foo")
			},
			expectedErr: nil,
		},
		{
			name: "returns the raid device if it's present in /dev/md under provided raid name",
			raidName: func(_ string) string {
				return "foo"
			},
			makeDevtmpfs: func(devtmpfsPath string) error {
				err := os.MkdirAll(path.Join(devtmpfsPath, "/md"), 0770)
				if err != nil {
					return err
				}

				f, err := os.Create(path.Join(devtmpfsPath, "/md", "foo"))
				if err != nil {
					return err
				}

				err = f.Close()
				if err != nil {
					return err
				}

				return nil
			},
			expectedCommands: func(devtmpfsPath string) []exectest.Command {
				return nil
			},
			expectedRaidDevice: func(devtmpfsPath string) string {
				return path.Join(devtmpfsPath, "/md/foo")
			},
			expectedErr: nil,
		},
		{
			name: "returns a device pointed by symlink matching raid name pattern in /dev/disk/by-id",
			raidName: func(_ string) string {
				return "foo"
			},
			makeDevtmpfs: func(devtmpfsPath string) error {
				err := os.MkdirAll(path.Join(devtmpfsPath, "/disk/by-id"), 0770)
				if err != nil {
					return err
				}

				devPath := path.Join(devtmpfsPath, "foo")
				f, err := os.Create(devPath)
				if err != nil {
					return err
				}

				err = f.Close()
				if err != nil {
					return err
				}

				err = os.Symlink(devPath, path.Join(devtmpfsPath, "/disk/by-id", "md-name-foo"))
				if err != nil {
					return err
				}

				return nil
			},
			expectedCommands: func(devtmpfsPath string) []exectest.Command {
				return nil
			},
			expectedRaidDevice: func(devtmpfsPath string) string {
				return path.Join(devtmpfsPath, "foo")
			},
			expectedErr: nil,
		},
		{
			name: "returns a device from mdadm report matching name stored in device metadata",
			raidName: func(_ string) string {
				return "foo"
			},
			makeDevtmpfs: func(devtmpfsPath string) error {
				return nil
			},
			expectedCommands: func(devtmpfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
						Stdout: []byte(strings.Join([]string{
							"ARRAY /dev/bar metadata=1.2 uuid=58321568:asd12351:5dsadsa9:32121527 name=bar",
							fmt.Sprintf("ARRAY %s metadata=1.2 uuid=58641338:62a3a9ca:544625b9:ad5fe527 name=foo", path.Join(devtmpfsPath, "foo")),
							"ARRAY /dev/dar metadata=1.2 uuid=65465238:4324312ds:dsadfb1:a3214527 name=dar",
						}, "\n")),
					},
				}
			},
			expectedRaidDevice: func(devtmpfsPath string) string {
				return path.Join(devtmpfsPath, "foo")
			},
			expectedErr: nil,
		},
		{
			name: "returns an error in case when raid isn't found in any location",
			raidName: func(_ string) string {
				return "foo"
			},
			makeDevtmpfs: func(devtmpfsPath string) error {
				return nil
			},
			expectedCommands: func(devtmpfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:  "mdadm",
						Args: []string{"--detail", "--scan"},
						Stdout: []byte(strings.Join([]string{
							"ARRAY /dev/bar metadata=1.2 uuid=58321568:asd12351:5dsadsa9:32121527 name=bar",
							"ARRAY /dev/dar metadata=1.2 uuid=65465238:4324312ds:dsadfb1:a3214527 name=dar",
						}, "\n")),
					},
				}
			},
			expectedRaidDevice: func(devtmpfsPath string) string {
				return ""
			},
			expectedErr: fmt.Errorf("cannot find raid device"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			devtmpfsPath, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-devtmpfs")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(devtmpfsPath)

			err = tc.makeDevtmpfs(devtmpfsPath)
			if err != nil {
				t.Fatal(err)
			}

			expectedCommands := tc.expectedCommands(devtmpfsPath)
			executor := exectest.NewFakeExec(expectedCommands...)

			expectedRaidDevice := tc.expectedRaidDevice(devtmpfsPath)

			raidName := tc.raidName(devtmpfsPath)
			raidDevice, err := GetDeviceWithName(ctx, executor, devtmpfsPath, raidName)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(raidDevice, expectedRaidDevice) {
				t.Fatalf("expected %v raidDevice, got %v", expectedRaidDevice, raidDevice)
			}
			if executor.CommandCalls != len(expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(expectedCommands), executor.CommandCalls)
			}
		})
	}
}
