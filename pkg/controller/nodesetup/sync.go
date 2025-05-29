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
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
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
		fmt.Sprintf(loopDeviceControllerNodeSetupProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(loopDeviceControllerNodeSetupDegradedConditionFormat, nsc.nodeName),
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
		fmt.Sprintf(raidControllerNodeSetupProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(raidControllerNodeSetupDegradedConditionFormat, nsc.nodeName),
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
		fmt.Sprintf(filesystemControllerNodeSetupProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(filesystemControllerNodeSetupDegradedConditionFormat, nsc.nodeName),
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
		fmt.Sprintf(mountControllerNodeSetupProgressingConditionFormat, nsc.nodeName),
		fmt.Sprintf(mountControllerNodeSetupDegradedConditionFormat, nsc.nodeName),
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
	nodeSetupAvailableConditionType := fmt.Sprintf(internalapi.NodeSetupAvailableConditionFormat, nsc.nodeName)
	nodeSetupAvailableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeSetupAvailableConditionType),
		metav1.Condition{
			Type:               nodeSetupAvailableConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate available node setup status conditions: %w", err))
	}

	nodeSetupProgressingConditionType := fmt.Sprintf(internalapi.NodeSetupProgressingConditionFormat, nsc.nodeName)
	nodeSetupProgressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeSetupProgressingConditionType),
		metav1.Condition{
			Type:               nodeSetupProgressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing node setup status conditions: %w", err))
	}

	nodeSetupDegradedConditionType := fmt.Sprintf(internalapi.NodeSetupDegradedConditionFormat, nsc.nodeName)
	nodeSetupDegradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeSetupDegradedConditionType),
		metav1.Condition{
			Type:               nodeSetupDegradedConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded node setup status conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return apimachineryutilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&statusConditions, nodeSetupAvailableCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeSetupProgressingCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeSetupDegradedCondition)

	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(statusConditions)
	err = nsc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
