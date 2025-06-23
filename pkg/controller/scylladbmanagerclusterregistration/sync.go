// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (smcrc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBManagerClusterRegistration", "ScyllaDBManagerClusterRegistration", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBManagerClusterRegistration", "ScyllaDBManagerClusterRegistration", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	smcr, err := smcrc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("ScyllaDBManagerClusterRegistration has been deleted", "ScyllaDBManagerClusterRegistration", klog.KRef(namespace, name))
			return nil
		}

		return fmt.Errorf("can't get ScyllaDBManagerClusterRegistration %q: %w", naming.ManualRef(namespace, name), err)
	}

	// Sanity check.
	if !controllerhelpers.IsManagedByGlobalScyllaDBManagerInstance(smcr) {
		return controllertools.NewNonRetriable(fmt.Sprintf("ScyllaDBManagerClusterRegistration %q is not supported as it is not managed by the global ScyllaDB Manager instance", naming.ObjRef(smcr)))
	}

	status := smcrc.calculateStatus(smcr)

	if smcr.DeletionTimestamp != nil {
		err = controllerhelpers.RunSync(
			&status.Conditions,
			scyllaDBManagerClusterRegistrationFinalizerProgressingCondition,
			scyllaDBManagerClusterRegistrationFinalizerDegradedCondition,
			smcr.Generation,
			func() ([]metav1.Condition, error) {
				return smcrc.syncFinalizer(ctx, smcr)
			},
		)
		if err != nil {
			return fmt.Errorf("can't finalize: %w", err)
		}

		return smcrc.updateStatus(ctx, smcr, status)
	}

	if !smcrc.hasFinalizer(smcr.GetFinalizers()) {
		err = smcrc.addFinalizer(ctx, smcr)
		if err != nil {
			return fmt.Errorf("can't add finalizer: %w", err)
		}
		return nil
	}

	var errs []error
	err = controllerhelpers.RunSync(
		&status.Conditions,
		managerControllerProgressingCondition,
		managerControllerDegradedCondition,
		smcr.Generation,
		func() ([]metav1.Condition, error) {
			return smcrc.syncManager(ctx, smcr, status)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync registration: %w", err))
	}

	var aggregationErrs []error
	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.ProgressingCondition),
		metav1.Condition{
			Type:               scyllav1alpha1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: smcr.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate progressing conditions: %w", err))
	}

	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.DegradedCondition),
		metav1.Condition{
			Type:               scyllav1alpha1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: smcr.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return apimachineryutilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = smcrc.updateStatus(ctx, smcr, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (smcrc *Controller) hasFinalizer(finalizers []string) bool {
	return oslices.ContainsItem(finalizers, naming.ScyllaDBManagerClusterRegistrationFinalizer)
}

func (smcrc *Controller) addFinalizer(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) error {
	if smcrc.hasFinalizer(smcr.GetFinalizers()) {
		return nil
	}

	patch, err := controllerhelpers.AddFinalizerPatch(smcr, naming.ScyllaDBManagerClusterRegistrationFinalizer)
	if err != nil {
		return fmt.Errorf("can't create add finalizer patch: %w", err)
	}

	_, err = smcrc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(smcr.Namespace).Patch(ctx, smcr.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBManagerClusterRegistration %q: %w", naming.ObjRef(smcr), err)
	}

	klog.V(2).InfoS("Added finalizer to ScyllaDBManagerClusterRegistration", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))
	return nil
}
