// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) getCanAdoptFunc(ctx context.Context) func() error {
	return func() error {
		fresh, err := ncdc.scyllaClient.ScyllaV1alpha1().NodeConfigs().Get(ctx, ncdc.nodeConfigName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != ncdc.nodeConfigUID {
			return fmt.Errorf("original NodeConfig %v is gone: got uid %v, wanted %v", ncdc.nodeConfigName, fresh.UID, ncdc.nodeConfigUID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v has just been deleted at %v", fresh.Name, fresh.DeletionTimestamp)
		}

		return nil
	}
}

func (ncdc *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started sync", "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished sync", "duration", time.Since(startTime))
	}()

	nc, err := ncdc.nodeConfigLister.Get(ncdc.nodeConfigName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("can't get current nodeconfig %q: %w", ncdc.nodeConfigName, err)
		}

		klog.V(2).InfoS("NodeConfig has been deleted", "NodeConfig", klog.KRef("", ncdc.nodeConfigName))
		return nil
	}

	if nc.UID != ncdc.nodeConfigUID {
		// In normal circumstances we should be deleted first by GC because of an ownerRef to the NodeConfig.
		return fmt.Errorf("nodeConfig UID %q doesn't match the expected UID %q", nc.UID, nc.UID)
	}

	status := ncdc.calculateStatus(nc)

	if nc.DeletionTimestamp != nil {
		return ncdc.updateStatus(ctx, nc, status)
	}

	statusConditions := status.Conditions.ToMetaV1Conditions()

	type CT = *appsv1.DaemonSet
	var objectErrs []error

	dsControllerRef, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return fmt.Errorf("can't get controller ref: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigJobForNodeUIDLabel: string(ncdc.nodeUID),
	})

	jobs, err := controllerhelpers.GetObjectsWithFilter[CT, *batchv1.Job](
		ctx,
		&metav1.ObjectMeta{
			Name:              dsControllerRef.Name,
			UID:               dsControllerRef.UID,
			DeletionTimestamp: nil,
		},
		daemonSetControllerGVK,
		selector,
		func(job *batchv1.Job) bool {
			return job.Spec.Template.Spec.NodeName == ncdc.nodeName
		},
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *batchv1.Job]{
			GetControllerUncachedFunc: ncdc.kubeClient.AppsV1().DaemonSets(ncdc.namespace).Get,
			ListObjectsFunc:           ncdc.namespacedJobLister.Jobs(ncdc.namespace).List,
			PatchObjectFunc:           ncdc.kubeClient.BatchV1().Jobs(ncdc.namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	nodeStatus := &scyllav1alpha1.NodeConfigNodeStatus{
		Name: ncdc.nodeName,
	}

	var errs []error
	err = controllerhelpers.RunSync(
		&statusConditions,
		fmt.Sprintf(jobControllerNodeTuneProgressingConditionFormat, ncdc.nodeName),
		fmt.Sprintf(jobControllerNodeTuneDegradedConditionFormat, ncdc.nodeName),
		nc.Generation,
		func() ([]metav1.Condition, error) {
			return ncdc.syncJobs(ctx, nc, jobs, nodeStatus)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync jobs: %w", err))
	}

	// Aggregate node conditions.
	var aggregationErrs []error
	nodeTuneAvailableConditionType := fmt.Sprintf(internalapi.NodeTuneAvailableConditionFormat, ncdc.nodeName)
	nodeTuneAvailableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeTuneAvailableConditionType),
		metav1.Condition{
			Type:               nodeTuneAvailableConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate available node tune status conditions: %w", err))
	}

	nodeTuneProgressingConditionType := fmt.Sprintf(internalapi.NodeTuneProgressingConditionFormat, ncdc.nodeName)
	nodeTuneProgressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeTuneProgressingConditionType),
		metav1.Condition{
			Type:               nodeTuneProgressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing node tune status conditions: %w", err))
	}

	nodeTuneDegradedConditionType := fmt.Sprintf(internalapi.NodeTuneDegradedConditionFormat, ncdc.nodeName)
	nodeTuneDegradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(statusConditions, nodeTuneDegradedConditionType),
		metav1.Condition{
			Type:               nodeTuneDegradedConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: nc.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded node tune status conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return utilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&statusConditions, nodeTuneAvailableCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeTuneProgressingCondition)
	apimeta.SetStatusCondition(&statusConditions, nodeTuneDegradedCondition)

	status.Conditions = scyllav1alpha1.NewNodeConfigConditions(statusConditions)
	status.NodeStatuses = controllerhelpers.SetNodeStatus(status.NodeStatuses, nodeStatus)

	err = ncdc.updateStatus(ctx, nc, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return utilerrors.NewAggregate(errs)
}
