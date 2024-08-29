// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
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

	statusConditions := status.Conditions.ToMetaV1Conditions()

	var errs []error
	err = controllerhelpers.RunSync(
		&statusConditions,
		fmt.Sprintf(loopDeviceControllerNodeProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(loopDeviceControllerNodeDegradedConditionFormat, nsc.nodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncLoopDevices(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync loop devices: %w", err))
	}

	err = controllerhelpers.RunSync(
		&statusConditions,
		fmt.Sprintf(raidControllerNodeProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(raidControllerNodeDegradedConditionFormat, nsc.nodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncRAIDs(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync raids: %w", err))
	}

	err = controllerhelpers.RunSync(
		&statusConditions,
		fmt.Sprintf(filesystemControllerNodeProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(filesystemControllerNodeDegradedConditionFormat, nsc.nodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return nsc.syncFilesystems(ctx, nc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync filesystems: %w", err))
	}

	err = controllerhelpers.RunSync(
		&statusConditions,
		fmt.Sprintf(mountControllerNodeProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(mountControllerNodeDegradedConditionFormat, nsc.nodeName),
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
	nodeAvailableConditionType := fmt.Sprintf(internalapi.NodeAvailableConditionFormat, nsc.nodeName)
	nodeAvailableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeAvailableConditionType),
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

	nodeProgressingConditionType := fmt.Sprintf(internalapi.NodeProgressingConditionFormat, nsc.nodeName)
	nodeProgressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeProgressingConditionType),
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

	nodeDegradedConditionType := fmt.Sprintf(internalapi.NodeDegradedConditionFormat, nsc.nodeName)
	nodeDegradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeDegradedConditionType),
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

	apimeta.SetStatusCondition(&statusConditions, nodeAvailableCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeProgressingCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeDegradedCondition)

	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(statusConditions)
	err = nsc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return utilerrors.NewAggregate(errs)
}
