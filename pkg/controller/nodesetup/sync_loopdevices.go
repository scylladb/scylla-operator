// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"path"
	"path/filepath"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/losetup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

func (nsc *Controller) syncLoopDevices(ctx context.Context, nc *scyllav1alpha1.NodeConfig) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredLoopDeviceImages := getRequiredLoopDeviceImages(nsc.devtmpfsPath, nc)
	existingLoopDevices, err := discoverNodeConfigLoopDevices(ctx, nsc.executor, nsc.devtmpfsPath)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get node config loop devices: %w", err)
	}

	err = nsc.pruneLoopDevices(ctx, nc, existingLoopDevices, requiredLoopDeviceImages)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune loop devices: %w", err)
	}

	if nc.Spec.LocalDiskSetup == nil {
		return progressingConditions, nil
	}

	var errs []error
	for _, ld := range nc.Spec.LocalDiskSetup.LoopDevices {
		sizeInBytes, ok := ld.Size.AsInt64()
		if !ok {
			errs = append(errs, fmt.Errorf("invalid loop device size %q", ld.Size.String()))
			continue
		}

		symlinkPath := getLoopDeviceSymlinkPath(nsc.devtmpfsPath, ld.Name)
		changed, err := disks.ApplyLoopDevice(ctx, nsc.executor, ld.ImagePath, symlinkPath, sizeInBytes)
		if err != nil {
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeWarning,
				"CreateLoopDeviceFailed",
				"Failed to create loop device %s: %v",
				ld.Name, err,
			)
			errs = append(errs, fmt.Errorf("can't create loop device %q: %w", ld.Name, err))
			continue
		}

		if !changed {
			klog.V(4).InfoS("Device is already present", "Device", ld.Name, "Size", ld.Size)
			continue
		}

		klog.V(2).InfoS("Created loop device", "Name", ld.Name, "Size", ld.Size)
		nsc.eventRecorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"LoopDeviceCreated",
			"Create loop device %q (size %s)",
			ld.Name, ld.Size.String(),
		)
	}

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("failed to create loop devices: %w", err)
	}

	return progressingConditions, nil
}

func (nsc *Controller) pruneLoopDevices(ctx context.Context, nc *scyllav1alpha1.NodeConfig, existingLoopDevices []loopDevice, requiredLoopDeviceImages []loopDevice) error {
	var errs []error
	for _, ld := range existingLoopDevices {
		_, _, isRequired := oslices.Find(requiredLoopDeviceImages, func(rld loopDevice) bool {
			return equality.Semantic.DeepEqual(ld, rld)
		})
		if isRequired {
			continue
		}

		devicePath, err := filepath.EvalSymlinks(ld.Symlink)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't evaluate device symlink %q: %w", ld.Symlink, err))
			continue
		}
		err = disks.DeleteLoopDevice(ctx, nsc.executor, ld.Symlink, devicePath, ld.Image)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't delete loop device %q: %w", ld.Name, err))
			continue
		}

		nsc.eventRecorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"LoopDeviceDeleted",
			"Loop device %q having %q image was deleted",
			ld.Name, ld.Image,
		)
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("failed to delete stale loop devices: %w", err)
	}
	return nil
}

type loopDevice struct {
	Name    string
	Image   string
	Symlink string
}

func discoverNodeConfigLoopDevices(ctx context.Context, executor exec.Interface, devfsPath string) ([]loopDevice, error) {
	systemLoopDevices, err := losetup.ListLoopDevices(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("can't list loop devices: %w", err)
	}

	ncDeviceSymlinks, err := filepath.Glob(path.Join(devfsPath, naming.DevLoopDeviceDirectoryName, "*"))
	if err != nil {
		return nil, fmt.Errorf("can't glob symlinks directory: %w", err)
	}

	loopDeviceToSymlink := make(map[string]string, len(ncDeviceSymlinks))
	for _, ds := range ncDeviceSymlinks {
		ldName, err := filepath.EvalSymlinks(ds)
		if err != nil {
			return nil, fmt.Errorf("can't evaluate symlink %q: %w", ds, err)
		}

		loopDeviceToSymlink[ldName] = ds
	}

	ncSystemLoopDevices := oslices.Filter(systemLoopDevices, func(device *losetup.LoopDevice) bool {
		_, ok := loopDeviceToSymlink[device.Name]
		return ok
	})

	ncLoopDevices := make([]loopDevice, 0, len(ncSystemLoopDevices))
	for _, ld := range ncSystemLoopDevices {
		symlink := loopDeviceToSymlink[ld.Name]
		ncLoopDevices = append(ncLoopDevices, loopDevice{
			Name:    path.Base(symlink),
			Image:   ld.BackingFile,
			Symlink: symlink,
		})
	}

	return ncLoopDevices, nil
}

func getRequiredLoopDeviceImages(devfsPath string, nc *scyllav1alpha1.NodeConfig) []loopDevice {
	if nc.Spec.LocalDiskSetup == nil {
		return nil
	}

	var lds []loopDevice
	for _, ld := range nc.Spec.LocalDiskSetup.LoopDevices {
		lds = append(lds, loopDevice{
			Name:    ld.Name,
			Image:   ld.ImagePath,
			Symlink: getLoopDeviceSymlinkPath(devfsPath, ld.Name),
		})
	}

	return lds
}

func getLoopDeviceSymlinkPath(devfsPath, deviceName string) string {
	return path.Join(devfsPath, naming.DevLoopDeviceDirectoryName, deviceName)
}
