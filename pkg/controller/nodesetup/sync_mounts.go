// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	"github.com/scylladb/scylla-operator/pkg/systemd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncMounts(ctx context.Context, nc *scyllav1alpha1.NodeConfig) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	var mountUnits []*systemd.NamedUnit
	if nc.Spec.LocalDiskSetup != nil {
		for _, mc := range nc.Spec.LocalDiskSetup.Mounts {
			device, err := disks.GetDeviceWithName(ctx, nsc.executor, nsc.devtmpfsPath, mc.Device)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't resolve RAID device %q: %w", mc.Device, err))
				continue
			}

			mount := systemd.Mount{
				Description: fmt.Sprintf("Managed mount by Scylla Operator"),
				Device:      device,
				MountPoint:  mc.MountPoint,
				FSType:      mc.FSType,
				Options:     append([]string{"X-mount.mkdir"}, mc.UnsupportedOptions...),
			}

			mountUnit, err := mount.MakeUnit()
			if err != nil {
				errs = append(errs, fmt.Errorf("can't make unit: %w", err))
				continue
			}

			mountUnits = append(mountUnits, mountUnit)

			klog.V(4).InfoS("Mount unit has been generated and queued for apply.", "Name", mountUnit.FileName, "Device", mc.Device, "MountPoint", mc.MountPoint)
		}
	}

	err := nsc.systemdUnitManager.EnsureUnits(ctx, nc, nsc.eventRecorder, mountUnits, nsc.systemdControl)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't ensure units: %w", err))
	}

	// TODO: Check mount units for failures.
	//       https://github.com/scylladb/scylla-operator/issues/1334

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't create mounts: %w", err)
	}

	return progressingConditions, nil
}
