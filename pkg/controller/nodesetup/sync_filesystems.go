// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncFilesystems(ctx context.Context, nc *scyllav1alpha1.NodeConfig) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	if nc.Spec.LocalDiskSetup == nil {
		return progressingConditions, nil
	}

	for _, fs := range nc.Spec.LocalDiskSetup.Filesystems {
		device, err := disks.GetDeviceWithName(ctx, nsc.executor, nsc.devtmpfsPath, fs.Device)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't resolve RAID device %q: %w", fs.Device, err))
			continue
		}

		blockSize, err := disks.GetBlockSize(device, nsc.sysfsPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't determine block size of device %q at %q: %w", fs.Device, device, err))
			continue
		}

		changed, err := disks.MakeFS(ctx, nsc.executor, device, blockSize, string(fs.Type))
		if err != nil {
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeWarning,
				"CreateFilesystemFailed",
				"Failed to create filesystem %s on %s device: %v",
				fs.Type, fs.Device, err,
			)
			errs = append(errs, fmt.Errorf("can't create filesystem %q on device %q at %q: %w", fs.Type, fs.Device, device, err))
			continue
		}

		if !changed {
			klog.V(4).InfoS("Device already formatted, nothing to do", "Device", fs.Device, "Filesystem", fs.Type)
			continue
		}

		klog.V(2).InfoS("Filesystem has been created", "Device", fs.Device, "Filesystem", fs.Type)
		nsc.eventRecorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"FilesystemCreated",
			"%s filesystem has been created device %s",
			fs.Type, fs.Device,
		)
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("failed to create filesystems: %w", err)
	}

	return progressingConditions, nil
}
