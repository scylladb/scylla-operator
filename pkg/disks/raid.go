// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/naming"
	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	"k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

var (
	ErrRAIDNotFound = fmt.Errorf("cannot find raid device")
)

func MakeRAID0(ctx context.Context, executor exec.Interface, sysfsPath, devtmpfsPath, name string, devices []string, udevControlEnabled bool) (changed bool, err error) {
	raidDevice, err := GetDeviceWithName(ctx, executor, devtmpfsPath, name)
	if err != nil && !errors.Is(err, ErrRAIDNotFound) {
		return false, fmt.Errorf("can't get raid device with name %q: %w", name, err)
	}

	if len(raidDevice) != 0 {
		_, err = os.Stat(raidDevice)
		if err != nil {
			return false, fmt.Errorf("RAID with %q name is found at %q, but it doesn't exsist", name, raidDevice)
		}

		klog.V(4).InfoS("RAID device already exists", "name", name, "device", raidDevice)

		mdLevel, slaves, err := getRAIDInfo(sysfsPath, raidDevice)
		if err != nil {
			return false, fmt.Errorf("can't get raid info about %q device: %w", raidDevice, err)
		}

		if mdLevel != "raid0" {
			return false, fmt.Errorf(`expected "raid0" md level of existing raid device, got %q`, mdLevel)
		}

		deviceNames := make([]string, 0, len(devices))
		for _, d := range devices {
			deviceNames = append(deviceNames, path.Base(d))
		}

		sort.Strings(deviceNames)
		sort.Strings(slaves)

		if !equality.Semantic.DeepEqual(slaves, deviceNames) {
			return false, fmt.Errorf("existing raid device at %q consists of %q devices, expected %q", raidDevice, strings.Join(slaves, ","), strings.Join(deviceNames, ","))
		}

		return false, nil
	}

	for _, device := range devices {
		deviceName := path.Base(device)
		_, err := os.Stat(device)
		if os.IsNotExist(err) {
			return false, fmt.Errorf("device %q does not exists", deviceName)
		}

		// Before array is created, devices must be discarded, otherwise raid creation would fail.
		// Not all device types support it, we discard them only if discard_granularity is set.
		discardPath := path.Join(sysfsPath, fmt.Sprintf("/block/%s/queue/discard_granularity", deviceName))

		_, err = os.Stat(discardPath)
		if !os.IsNotExist(err) {
			discard, err := os.ReadFile(discardPath)
			if err != nil {
				return false, fmt.Errorf("can't read discard file at %q: %w", discardPath, err)
			}

			if strings.TrimSpace(string(discard)) != "0" {
				stdout, stderr, err := oexec.RunCommand(ctx, executor, "blkdiscard", device)
				if err != nil {
					return false, fmt.Errorf("can't discard device %q: %w, stdout: %q, stderr: %q", deviceName, err, stdout.String(), stderr.String())
				}
			}
		}
	}

	if udevControlEnabled {
		// Settle udev events and temporarily stop exec queue to prevent contention on device handles.
		stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "settle")
		if err != nil {
			return false, fmt.Errorf("can't run udevadm settle: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}

		stdout, stderr, err = oexec.RunCommand(ctx, executor, "udevadm", "control", "--stop-exec-queue")
		if err != nil {
			return false, fmt.Errorf("can't stop udevadm exec queue: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}
		defer func() {
			// It's idempotent.
			stdout, stderr, startErr := oexec.RunCommand(ctx, executor, "udevadm", "control", "--start-exec-queue")
			if startErr != nil {
				err = utilerrors.NewAggregate([]error{err, fmt.Errorf("can't start udevadm exec queue: %w, stdout: %q, stderr: %q", startErr, stdout.String(), stderr.String())})
			}
		}()
	}

	createRaidArgs := []string{
		"--create",
		"--verbose",
		"--run",
		name,
		"--level=0",
		"--chunk=1024",
		"--homehost=<none>",
		fmt.Sprintf("--name=%s", name),
		fmt.Sprintf("--raid-devices=%d", len(devices)),
	}

	createRaidArgs = append(createRaidArgs, devices...)

	if len(devices) == 1 {
		createRaidArgs = append(createRaidArgs, "--force")
	}

	klog.V(4).Infof("Creating RAID array using args: %v", createRaidArgs)
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "mdadm", createRaidArgs...)
	if err != nil {
		return false, fmt.Errorf("can't run mdadm with args %q: %w, stdout: %q, stderr: %q", createRaidArgs, err, stdout.String(), stderr.String())
	}

	if udevControlEnabled {
		// Enable udev exec queue again and wait for settle.
		stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "control", "--start-exec-queue")
		if err != nil {
			return false, fmt.Errorf("can't stop udevadm exec queue: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}

		stdout, stderr, err = oexec.RunCommand(ctx, executor, "udevadm", "settle")
		if err != nil {
			return false, fmt.Errorf("can't run udevadm settle: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}
	}

	return true, nil
}

// GetDeviceWithName finds a device path to the raid array with given name.
// Because mdadm - tool used for array creation - doesn't always use the name we provide,
// we need to find the device using several heuristics. Different versions of mdadm behaves differently, so there
// are multiple steps of it.
// 1. Try provided name, it could be a reference to a device - if it exists, resolve and return it.
// 2. Try expected name and location /dev/md/name - if file exists, that's the device.
// 3. Try expected name and location /dev/loops/name - if file exists, resolve symlink and return the device it points to.
// 4. List /dev/disk/by-id/md-name-* files, if name we search for is a suffix, then the symlink points to the raid device.
// 5. Parse mdadm report and find the device by checking name stored in metadata.
// It supports passing either a name of raid device, or full /dev/md/name pattern.
func GetDeviceWithName(ctx context.Context, executor exec.Interface, devtmpfsPath, name string) (string, error) {
	// Try directly the name, it could be a reference to existing device
	_, err := os.Stat(name)
	if err == nil {
		// It may be a symlink to real device, so resolve it
		resolvedDevice, err := filepath.EvalSymlinks(name)
		if err != nil {
			return "", fmt.Errorf("can't evaluate symlink %q: %w", name, err)
		}

		return resolvedDevice, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("can't stat %q: %w", name, err)
	}

	// Try expected name in the /dev/md.
	expectedDevicePath := path.Join(devtmpfsPath, "md", name)
	_, err = os.Stat(expectedDevicePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("can't stat %q: %w", expectedDevicePath, err)
	}
	if err == nil {
		return expectedDevicePath, nil
	}

	// Try expected name in the /dev/loops
	expectedDevicePath = path.Join(devtmpfsPath, naming.DevLoopDeviceDirectoryName, name)
	_, err = os.Stat(expectedDevicePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("can't stat %q: %w", expectedDevicePath, err)
	}
	if err == nil {
		resolvedDevice, err := filepath.EvalSymlinks(expectedDevicePath)
		if err != nil {
			return "", fmt.Errorf("can't evaluate symlink %q: %w", expectedDevicePath, err)
		}
		return resolvedDevice, nil
	}

	// Strip /dev/md/ prefix to get just the name under which raids are created.
	name = strings.TrimPrefix(name, fmt.Sprintf("%s/", path.Join(devtmpfsPath, "md")))

	// Try to find name via /dev/disk/by-id symlinks created by udev rules installed alongside mdadm
	disksByIDPath := path.Join(devtmpfsPath, "/disk/by-id")
	var foundDevice string

	err = filepath.WalkDir(disksByIDPath, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if fpath == disksByIDPath || d.IsDir() {
			return nil
		}

		const (
			mdNamePrefix = "md-name-"
		)

		fileName := path.Base(fpath)
		if strings.HasPrefix(fileName, mdNamePrefix) && strings.HasSuffix(fileName, name) {
			realDevice, err := filepath.EvalSymlinks(fpath)
			if err != nil {
				return fmt.Errorf("can't evaluate device %q symlink: %w", fpath, err)
			}

			foundDevice = realDevice

			return filepath.SkipAll
		}

		return nil
	})

	if len(foundDevice) != 0 {
		return foundDevice, nil
	}

	// Try to find it in mdadm report
	args := []string{
		"--detail",
		"--scan",
	}
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "mdadm", args...)
	if err != nil {
		return "", fmt.Errorf("can't run mdadm with args %q: %w, stdout: %q, stderr: %q", args, err, stdout, stderr)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		fields := strings.Fields(line)
		if fields[0] != "ARRAY" {
			continue
		}

		for i := 2; i < len(fields); i++ {
			if fields[i] == fmt.Sprintf("name=%s", name) {
				device := fields[1]
				return device, nil
			}
		}
	}

	return "", ErrRAIDNotFound
}

func getRAIDInfo(sysfsPath, device string) (string, []string, error) {
	realDevice, err := filepath.EvalSymlinks(device)
	if err != nil {
		return "", nil, fmt.Errorf("can't evaluate device %q symlink: %w", device, err)
	}

	mdLevelRaw, err := os.ReadFile(path.Join(sysfsPath, fmt.Sprintf("/block/%s/md/level", path.Base(realDevice))))
	if err != nil {
		return "", nil, fmt.Errorf("can't check existing raid device md level: %w", err)
	}

	mdLevel := strings.TrimSpace(string(mdLevelRaw))

	slavesDirPattern := path.Join(sysfsPath, fmt.Sprintf("/block/%s/slaves/*", path.Base(realDevice)))
	slavePaths, err := filepath.Glob(slavesDirPattern)
	if err != nil {
		return "", nil, fmt.Errorf("can't determine slaves of raid array: %s", err)
	}

	slaves := make([]string, 0, len(slavePaths))
	for _, slavePath := range slavePaths {
		slaves = append(slaves, path.Base(slavePath))
	}

	return mdLevel, slaves, nil
}
