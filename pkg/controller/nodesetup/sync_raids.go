// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"os"
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
)

func (nsc *Controller) syncRAIDs(ctx context.Context, nc *scyllav1alpha1.NodeConfig) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	if nc.Spec.LocalDiskSetup == nil {
		return progressingConditions, nil
	}

	blockDevices, err := blkutils.ListBlockDevices(ctx, nsc.executor)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't list block devices: %w", err)
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

func filterMatchingRe(blockDevices []*blkutils.BlockDevice, nameRegexp, modelRegexp string) ([]string, error) {
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
	for _, blockDevice := range blockDevices {
		if nameRe != nil && !nameRe.MatchString(blockDevice.Name) {
			continue
		}

		if modelRe != nil && !modelRe.MatchString(blockDevice.Model) {
			continue
		}

		devices = append(devices, blockDevice.Name)
	}

	return devices, nil
}
