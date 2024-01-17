// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"

	"github.com/scylladb/scylla-operator/pkg/util/losetup"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

// DeleteLoopDevice removes loop device and files associated with it like image and symlink.
func DeleteLoopDevice(ctx context.Context, executor exec.Interface, deviceSymlinkPath, devicePath, imagePath string) error {
	err := detachLoopDevice(ctx, executor, devicePath)
	if err != nil {
		return fmt.Errorf("can't detach loop device %q: %w", devicePath, err)
	}

	err = os.Remove(imagePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("can't image file %q of loop device %q: %w", imagePath, devicePath, err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		klog.V(4).InfoS("Device image does not exist. Nothing to do.", "Device", devicePath, "Image", imagePath)
	}

	err = deleteSymlinkToDevice(deviceSymlinkPath)
	if err != nil {
		return fmt.Errorf("can't delete device %q symlink: %w", devicePath, err)
	}

	symlinksDir := path.Dir(deviceSymlinkPath)
	err = deleteEmptyDeviceSymlinkDir(symlinksDir)
	if err != nil {
		return fmt.Errorf("can't delete device symlinks directory: %w", err)
	}

	return nil
}

// ApplyLoopDevice creates an image for loop device, actual loop device and symlink associated with provided name.
// Creation of loop device consist of following steps:
// 1. Creating an image file for the loop device using the provided image and sizeBytes parameters.
// 2. Attaching loop device to the created image file.
// 3. Creating a symbolic link under /dev/loops with the specified name.
// False is returned when there was nothing to do, meaning such loop device, image and symlink already exists.
func ApplyLoopDevice(ctx context.Context, executor exec.Interface, imagePath, symlinkPath string, sizeBytes int64) (bool, error) {
	imageCreated, err := makeLoopDeviceImage(imagePath, sizeBytes)
	if err != nil {
		return false, fmt.Errorf("can't make loop device image file: %w", err)
	}

	deviceCreated, loopDevicePath, err := attachLoopDevice(ctx, executor, imagePath)
	if err != nil {
		return false, fmt.Errorf("can't attach loop device to image at %q: %w", imagePath, err)
	}

	symlinkCreated, err := makeDeviceSymlink(loopDevicePath, symlinkPath)
	if err != nil {
		return false, fmt.Errorf("can't make symlink to loop device: %w", err)
	}

	changed := imageCreated || deviceCreated || symlinkCreated
	return changed, nil
}

func makeLoopDeviceImage(imagePath string, sizeBytes int64) (bool, error) {
	imgStat, err := os.Stat(imagePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("can't stat %q: %w", imagePath, err)
	}
	if err == nil {
		if sizeBytes == imgStat.Size() {
			klog.V(4).InfoS("Loop device image already exists, nothing to do.", "Image", imagePath)
			return false, nil
		}
		return false, fmt.Errorf("device image resize is not supported, existing image file %q has size %d, different than expected %d", imagePath, imgStat.Size(), sizeBytes)
	}

	klog.V(4).InfoS("Creating loop device image", "Image", imagePath, "Size", sizeBytes)
	err = createSparseFile(imagePath, sizeBytes)
	if err != nil {
		return false, fmt.Errorf("can't create image file at %q: %w", imagePath, err)
	}

	return true, nil
}

func createSparseFile(filePath string, sizeBytes int64) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("can't create file at %q: %w", filePath, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("can't close file at %q: %w", filePath, err)
	}

	err = os.Truncate(filePath, sizeBytes)
	if err != nil {
		return fmt.Errorf("can't truncate file at %q: %w", filePath, err)
	}

	return nil
}

func attachLoopDevice(ctx context.Context, executor exec.Interface, imagePath string) (bool, string, error) {
	loopDevices, err := losetup.ListLoopDevices(ctx, executor)
	if err != nil {
		return false, "", fmt.Errorf("can't list loop devices: %w", err)
	}

	for _, ld := range loopDevices {
		if imagePath == ld.BackingFile {
			klog.V(4).InfoS("Loop device already exists, nothing to do.", "Device", ld.Name, "Image", ld.BackingFile)
			return false, ld.Name, nil
		}
	}

	klog.V(4).InfoS("Attaching loop device to image.", "Image", imagePath)
	devicePath, err := losetup.AttachLoopDevice(ctx, executor, imagePath)
	if err != nil {
		return false, "", fmt.Errorf("can't attach loop device to image %q: %w", imagePath, err)
	}
	klog.V(4).InfoS("Successfully attached loop device to image.", "Device", devicePath, "Image", imagePath)

	return true, devicePath, nil
}

func makeDeviceSymlink(loopDevicePath, devicePath string) (bool, error) {
	symlinkDirPath := path.Dir(devicePath)
	_, err := os.Stat(symlinkDirPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("can't stat %q: %w", symlinkDirPath, err)
		}

		mkdirErr := os.Mkdir(symlinkDirPath, 0770)
		if mkdirErr != nil {
			return false, fmt.Errorf("can't mkdir %q: %w", symlinkDirPath, mkdirErr)
		}
	}

	_, err = os.Stat(devicePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("can't stat %q: %w", devicePath, err)
	}

	if errors.Is(err, fs.ErrNotExist) {
		klog.V(4).InfoS("Creating device symlink to loop device", "Device", loopDevicePath, "Symlink", devicePath)
		symlinkErr := os.Symlink(loopDevicePath, devicePath)
		if symlinkErr != nil {
			return false, fmt.Errorf("can't create symlink from %q to %q: %w", loopDevicePath, devicePath, symlinkErr)
		}

		return true, nil
	}

	klog.V(4).InfoS("Device symlink already exists, nothing to do.", "Device", loopDevicePath, "Symlink", devicePath)
	return false, nil
}

func deleteSymlinkToDevice(deviceSymlinkPath string) error {
	klog.V(4).InfoS("Removing device symlink", "Symlink", deviceSymlinkPath)
	err := os.Remove(deviceSymlinkPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("can't remove device symlink %q: %w", deviceSymlinkPath, err)
		}

		klog.V(4).InfoS("Device symlink does not exist. Nothing to do.", "DeviceSymlink", deviceSymlinkPath)
		return nil
	}

	return nil
}

func deleteEmptyDeviceSymlinkDir(symlinksDir string) (err error) {
	_, err = os.Stat(symlinksDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("can't stat symlinks directory %q: %w", symlinksDir, err)
		}

		klog.V(4).InfoS("Symlinks directory does not exist. Nothing to do.", "Directory", symlinksDir)
		return nil
	}

	d, err := os.Open(symlinksDir)
	if err != nil {
		return fmt.Errorf("can't open symlink dir at %q: %w", symlinksDir, err)
	}
	defer func() {
		dirCloseErr := d.Close()
		if dirCloseErr != nil {
			err = utilerrors.NewAggregate([]error{err, dirCloseErr})
		}
	}()

	_, err = d.Readdirnames(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("can't read directory names in %q: %w", symlinksDir, err)
	}

	// EOF after reading single dirname means directory is empty
	if errors.Is(err, io.EOF) {
		klog.V(4).InfoS("Removing empty device symlink directory", "Directory", symlinksDir)
		err = os.Remove(symlinksDir)
		if err != nil {
			return fmt.Errorf("can't remove symlink directory at %q: %w", symlinksDir, err)
		}
	}

	return nil
}

func detachLoopDevice(ctx context.Context, executor exec.Interface, devicePath string) error {
	loopDevices, err := losetup.ListLoopDevices(ctx, executor)
	if err != nil {
		return fmt.Errorf("can't list loop devices: %w", err)
	}

	exists := slices.ContainsFunc(loopDevices, func(device *losetup.LoopDevice) bool {
		return device.Name == devicePath
	})
	if !exists {
		klog.V(4).InfoS("Device does not exist. Nothing to do.", "Device", devicePath)
		return nil
	}

	err = losetup.DetachLoopDevice(ctx, executor, devicePath)
	if err != nil {
		return fmt.Errorf("can't detach loop device %q: %w", devicePath, err)
	}

	return nil
}
