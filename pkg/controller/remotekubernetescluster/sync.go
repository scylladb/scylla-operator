// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (rkcc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	name := req.Name
	rq := &controllertools.Requeue{}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing remote kubernetes cluster", "RemoteKubernetesCluster", name, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing remote kubernetes cluster", "RemoteKubernetesCluster", name, "duration", time.Since(startTime))
	}()

	rkc, err := ctrlclient.Get[scyllav1alpha1.RemoteKubernetesCluster](ctx, rkcc.client, "", name)
	if apierrors.IsNotFound(err) {
		for _, clusterHandler := range rkcc.dynamicClusterHandlers {
			clusterHandler.DeleteCluster(name)
		}

		return rq.Result(), nil
	}
	if err != nil {
		return rq.Result(), err
	}

	status := rkcc.calculateStatus(rkc)
	if rkc.DeletionTimestamp != nil {
		err = controllerhelpers.RunSync(
			&status.Conditions,
			remoteKubernetesClusterFinalizerProgressingCondition,
			remoteKubernetesClusterFinalizerDegradedCondition,
			rkc.Generation,
			func() ([]metav1.Condition, error) {
				return rkcc.syncFinalizer(ctx, rkc)
			},
		)
		if err != nil {
			return rq.Result(), fmt.Errorf("can't finalize: %w", err)
		}

		return rq.Result(), rkcc.updateStatus(ctx, rkc, status)
	}

	if !oslices.ContainsItem(rkc.GetFinalizers(), naming.RemoteKubernetesClusterFinalizer) {
		err = rkcc.addFinalizer(ctx, rkc)
		if err != nil {
			return rq.Result(), fmt.Errorf("can't add finalizer: %w", err)
		}
		return rq.Result(), nil
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		dynamicClusterHandlersControllerProgressingCondition,
		dynamicClusterHandlersControllerDegradedCondition,
		rkc.Generation,
		func() ([]metav1.Condition, error) {
			return rkcc.syncDynamicClusterHandlers(ctx, rkc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync dynamic cluster handlers: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		clientHealthcheckControllerProgressingCondition,
		clientHealthcheckControllerDegradedCondition,
		rkc.Generation,
		func() ([]metav1.Condition, error) {
			return rkcc.syncClientHealthchecks(ctx, rq, rkc, status)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync client healthchecks: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, rkc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = rkcc.updateStatus(ctx, rkc, status)
		errs = append(errs, err)
	}

	return rq.Result(), apimachineryutilerrors.NewAggregate(errs)
}
