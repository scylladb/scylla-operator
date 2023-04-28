// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	var errs []error
	var conditions []metav1.Condition

	if nc.DeletionTimestamp != nil {
		return nsc.updateNodeStatus(ctx, nc, conditions)
	}

	err = controllerhelpers.RunSync(
		&conditions,
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
		&conditions,
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
		&conditions,
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

	err = controllerhelpers.SetAggregatedNodeConditions(nsc.nodeName, &conditions, nc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate conditions: %w", err))
	}

	err = nsc.updateNodeStatus(ctx, nc, conditions)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update node status: %w", err))
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("failed to sync nodeconfig %q: %w", nsc.nodeConfigName, err)
	}

	return nil
}
