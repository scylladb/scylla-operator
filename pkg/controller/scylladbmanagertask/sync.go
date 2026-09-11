// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (smtc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	key := req.NamespacedName
	rq := &controllertools.Requeue{}

	namespace, name := key.Namespace, key.Name

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBManagerTask", "ScyllaDBManagerTask", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBManagerTask", "ScyllaDBManagerTask", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	smt, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBManagerTask](ctx, smtc.client, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).InfoS("ScyllaDBManagerTask has been deleted", "ScyllaDBManagerTask", klog.KRef(namespace, name))
			return rq.Result(), nil
		}

		return rq.Result(), fmt.Errorf("can't get ScyllaDBManagerTask %q: %w", naming.ManualRef(namespace, name), err)
	}

	status := smtc.calculateStatus(smt)

	if smt.DeletionTimestamp != nil {
		err = controllerhelpers.RunSync(
			&status.Conditions,
			scyllaDBManagerTaskFinalizerProgressingCondition,
			scyllaDBManagerTaskFinalizerDegradedCondition,
			smt.Generation,
			func() ([]metav1.Condition, error) {
				return smtc.syncFinalizer(ctx, smt)
			},
		)
		return rq.Result(), smtc.updateStatus(ctx, smt, status)
	}

	if !smtc.hasFinalizer(smt.GetFinalizers()) {
		err = smtc.addFinalizer(ctx, smt)
		if err != nil {
			return rq.Result(), fmt.Errorf("can't add finalizer: %w", err)
		}
		return rq.Result(), nil
	}

	var errs []error
	err = controllerhelpers.RunSync(
		&status.Conditions,
		managerControllerProgressingCondition,
		managerControllerDegradedCondition,
		smt.Generation,
		func() ([]metav1.Condition, error) {
			return smtc.syncManager(ctx, smt, status)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync manager: %w", err))
	}

	var aggregationErrs []error
	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav1alpha1.ProgressingCondition),
		metav1.Condition{
			Type:               scyllav1alpha1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: smt.Generation,
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
			ObservedGeneration: smt.Generation,
		},
	)
	if err != nil {
		aggregationErrs = append(aggregationErrs, fmt.Errorf("can't aggregate degraded conditions: %w", err))
	}

	if len(aggregationErrs) > 0 {
		errs = append(errs, aggregationErrs...)
		return rq.Result(), apimachineryutilerrors.NewAggregate(errs)
	}

	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = smtc.updateStatus(ctx, smt, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return rq.Result(), apimachineryutilerrors.NewAggregate(errs)
}

func (smtc *Controller) hasFinalizer(finalizers []string) bool {
	return oslices.ContainsItem(finalizers, naming.ScyllaDBManagerTaskFinalizer)
}

func (smtc *Controller) addFinalizer(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask) error {
	patch, err := controllerhelpers.AddFinalizerPatch(smt, naming.ScyllaDBManagerTaskFinalizer)
	if err != nil {
		return fmt.Errorf("can't create add finalizer patch: %w", err)
	}

	err = smtc.patch(ctx, smt, patch)
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBManagerTask %q: %w", naming.ObjRef(smt), err)
	}

	klog.V(2).InfoS("Added finalizer to ScyllaDBManagerTask", "ScyllaDBManagerTask", klog.KObj(smt))
	return nil
}

// patch applies a merge patch to the ScyllaDBManagerTask, without touching the cached object.
func (smtc *Controller) patch(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask, patch []byte) error {
	return smtc.client.Patch(ctx, &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: smt.Namespace,
			Name:      smt.Name,
		},
	}, client.RawPatch(types.MergePatchType, patch))
}
