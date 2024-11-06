// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/blkutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

func (nsc *Controller) syncRAIDs(ctx context.Context, nc *scyllav1alpha1.NodeConfig) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	if nc.Spec.LocalDiskSetup == nil {
		return progressingConditions, nil
	}

	blockDevices, progressingConditions, err := listBlockDevices(ctx, nsc.executor, nc, nsc.devtmpfsPath)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't list block devices: %w", err)
	}

	if len(progressingConditions) != 0 {
		return progressingConditions, nil
	}

	udevControlEnabled := false
	_, err = os.Stat("/run/udev/control")
	if err == nil {
		udevControlEnabled = true
	}

	for _, rc := range nc.Spec.LocalDiskSetup.RAIDs {
		switch rc.Type {
		case scyllav1alpha1.RAID0Type:
			if len(rc.RAID0.Devices.NameRegex) == 0 && len(rc.RAID0.Devices.ModelRegex) == 0 {
				errs = append(errs, fmt.Errorf("name or model regexp must be provided in %q RAID configuration of %q NodeConfig", rc.Name, naming.ObjRef(nc)))
				continue
			}

			devices, err := filterMatchingRe(blockDevices, rc.RAID0.Devices.NameRegex, rc.RAID0.Devices.ModelRegex)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't filter devices via regexp: %w", err))
				continue
			}

			if len(devices) == 0 {
				klog.Infof("No devices found for %q RAID array, nothing to do", rc.Name)
				continue
			}

			changed, err := disks.MakeRAID0(ctx, nsc.executor, nsc.sysfsPath, nsc.devtmpfsPath, rc.Name, devices, udevControlEnabled)
			if err != nil {
				nsc.eventRecorder.Eventf(
					nc,
					corev1.EventTypeWarning,
					"CreateRAIDFailed",
					"Failed to create %q RAID0 array from %s devices: %v",
					rc.Name, strings.Join(devices, ","), err,
				)
				errs = append(errs, fmt.Errorf("can't create %q RAID0 array out of %q: %w", rc.Name, strings.Join(devices, ","), err))
				continue
			}

			if !changed {
				klog.V(4).InfoS("RAID0 array already created, nothing to do", "RAIDName", rc.Name, "Devices", strings.Join(devices, ","))
				continue
			}

			klog.V(2).InfoS("RAID0 array has been created", "RAIDName", rc.Name, "Devices", strings.Join(devices, ","))
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeNormal,
				"RAIDCreated",
				"RAID0 array %q using %s devices has been created",
				rc.Name, strings.Join(devices, ","),
			)
		default:
			errs = append(errs, fmt.Errorf("unsupported RAID type: %q", rc.Type))
			continue
		}
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("failed to create raids: %w", err)
	}

	return progressingConditions, nil
}

func filterMatchingRe(blockDevices []*blockDevice, nameRegexp, modelRegexp string) ([]string, error) {
	var err error
	var nameRe, modelRe *regexp.Regexp

	if len(nameRegexp) > 0 {
		nameRe, err = regexp.Compile(nameRegexp)
		if err != nil {
			return nil, fmt.Errorf("can't compile device name regexp %q: %w", nameRegexp, err)
		}
	}

	if len(modelRegexp) > 0 {
		modelRe, err = regexp.Compile(modelRegexp)
		if err != nil {
			return nil, fmt.Errorf("can't compile device model regexp %q: %w", modelRegexp, err)
		}
	}

	var devices []string
	for _, bd := range blockDevices {
		if nameRe != nil && !(nameRe.MatchString(bd.Name) || (nameRe.MatchString(bd.APIFullPath))) {
			continue
		}

		if modelRe != nil && !modelRe.MatchString(bd.Model) {
			continue
		}

		devices = append(devices, bd.Name)
	}

	return devices, nil
}

type blockDevice struct {
	blkutils.BlockDevice
	// APIFullPath is a full path of a block devices that was created from API, ex. /dev/loops/my-loop-device.
	APIFullPath string
}

// listBlockDevices returns a list of block devices on host together with names under which they were referenced on API object.
func listBlockDevices(ctx context.Context, executor exec.Interface, nc *scyllav1alpha1.NodeConfig, devfsPath string) ([]*blockDevice, []metav1.Condition, error) {
	bds, err := blkutils.ListBlockDevices(ctx, executor)
	if err != nil {
		return nil, nil, fmt.Errorf("can't list block devices: %w", err)
	}

	var progressingConditions []metav1.Condition
	blockDevices := make([]*blockDevice, 0, len(bds))

	for _, bd := range bds {
		var apiFullPath string
		for _, lc := range nc.Spec.LocalDiskSetup.LoopDevices {
			symlinkPath := getLoopDeviceSymlinkPath(devfsPath, lc.Name)

			_, err := os.Stat(symlinkPath)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return nil, progressingConditions, fmt.Errorf("can't stat path %q: %w", symlinkPath, err)
				}

				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               raidControllerNodeSetupProgressingConditionFormat,
					Status:             metav1.ConditionTrue,
					Reason:             "AwaitingLoopDevice",
					Message:            fmt.Sprintf("Loop device %q not created yet", lc.Name),
					ObservedGeneration: nc.Generation,
				})
				continue
			}

			resolvedDevice, err := filepath.EvalSymlinks(symlinkPath)
			if err != nil {
				return nil, progressingConditions, fmt.Errorf("can't evaluate symlink %q: %w", symlinkPath, err)
			}

			if resolvedDevice == bd.Name {
				apiFullPath = symlinkPath
			}
		}

		blockDevices = append(blockDevices, &blockDevice{
			BlockDevice: blkutils.BlockDevice{
				Name:     bd.Name,
				Model:    bd.Model,
				FSType:   bd.FSType,
				PartUUID: bd.PartUUID,
			},
			APIFullPath: apiFullPath,
		})
	}

	return blockDevices, progressingConditions, nil
}
