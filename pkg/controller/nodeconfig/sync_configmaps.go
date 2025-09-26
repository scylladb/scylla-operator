// Copyright (C) 2025 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) syncConfigMaps(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	configMaps map[string]*corev1.ConfigMap,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredConfigMaps, err := makeConfigMaps(nc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required ConfigMap(s): %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredConfigMaps,
		configMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ncc.kubeClient.CoreV1().ConfigMaps(naming.ScyllaOperatorNodeTuningNamespace).Delete,
		},
		ncc.eventRecorder)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune ConfigMap(s): %w", err)
	}

	var errs []error
	for _, cm := range requiredConfigMaps {
		_, changed, err := resourceapply.ApplyConfigMap(ctx, ncc.kubeClient.CoreV1(), ncc.configMapLister, ncc.eventRecorder, cm, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, configMapControllerProgressingCondition, cm, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply ConfigMap: %w", err))
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
