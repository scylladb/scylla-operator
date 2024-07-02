// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing NodeConfig", "NodeConfig", nsc.nodeConfigName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing NodeConfig", "NodeConfig", nsc.nodeConfigName, "duration", time.Since(startTime))
	}()

	nc, err := nsc.nodeConfigLister.Get(nsc.nodeConfigName)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("NodeConfig has been deleted", "NodeConfig", klog.KObj(nc))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't list nodeconfigs: %w", err)
	}

	if nc.UID != nsc.nodeConfigUID {
		return fmt.Errorf("nodeConfig UID %q doesn't match the expected UID %q", nc.UID, nc.UID)
	}

	status := nsc.calculateStatus(nc)

	if nc.DeletionTimestamp != nil {
		return nsc.updateStatus(ctx, nc, status)
	}

	sanitizedNodeName := controllerhelpers.DNS1123SubdomainToValidStatusConditionReason(nsc.nodeName)

	var errs []error
	err = controllerhelpers.RunSync(
		&status.Conditions,
		fmt.Sprintf(loopDeviceControllerNodeProgressingConditionFormat, sanitizedNodeName),
		fmt.Sprintf(loopDeviceControllerNodeDegradedConditionFormat, sanitizedNodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncLoopDevices(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync loop devices: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		fmt.Sprintf(raidControllerNodeProgressingConditionFormat, sanitizedNodeName),
		fmt.Sprintf(raidControllerNodeDegradedConditionFormat, sanitizedNodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncRAIDs(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync raids: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		fmt.Sprintf(filesystemControllerNodeProgressingConditionFormat, sanitizedNodeName),
		fmt.Sprintf(filesystemControllerNodeDegradedConditionFormat, sanitizedNodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncFilesystems(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync filesystems: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		fmt.Sprintf(mountControllerNodeProgressingConditionFormat, sanitizedNodeName),
		fmt.Sprintf(mountControllerNodeDegradedConditionFormat, sanitizedNodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncMounts(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync mounts: %w", err))
	}

	// Aggregate node conditions.
	var aggregationErrs []error
	nodeAvailableConditionType := fmt.Sprintf(internalapi.NodeAvailableConditionFormat, sanitizedNodeName)
	availableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, nodeAvailableConditionType),
		metav1.Condition{
			Type:               nodeAvailableConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate available node status conditions: %w", err))
	}

	nodeProgressingConditionType := fmt.Sprintf(internalapi.NodeProgressingConditionFormat, sanitizedNodeName)
	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, nodeProgressingConditionType),
		metav1.Condition{
			Type:               nodeProgressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing node status conditions: %w", err))
	}

	nodeDegradedConditionType := fmt.Sprintf(internalapi.NodeDegradedConditionFormat, sanitizedNodeName)
	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, nodeDegradedConditionType),
		metav1.Condition{
			Type:               nodeDegradedConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded node status conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return utilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&status.Conditions, availableCondition)
	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = nsc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update node status: %w", err))
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("failed to sync nodeconfig %q: %w", nsc.nodeConfigName, err)
	}

	return nil
}
