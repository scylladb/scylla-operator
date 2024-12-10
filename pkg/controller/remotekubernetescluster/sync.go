// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (rkcc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing remote kubernetes cluster", "RemoteKubernetesCluster", name, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing remote kubernetes cluster", "RemoteKubernetesCluster", name, "duration", time.Since(startTime))
	}()

	rkc, err := rkcc.remoteKubernetesClusterLister.Get(name)
	if apierrors.IsNotFound(err) {
		for _, clusterHandler := range rkcc.dynamicClusterHandlers {
			clusterHandler.DeleteCluster(name)
		}

		return nil
	}
	if err != nil {
		return err
	}

	status := rkcc.calculateStatus(rkc)
	if rkc.DeletionTimestamp != nil {
		return rkcc.updateStatus(ctx, rkc, status)
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
			return rkcc.syncClientHealthchecks(ctx, key, rkc, status)
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

	return errors.NewAggregate(errs)
}
