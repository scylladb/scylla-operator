package scyllaoperatorconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (opc *Controller) sync(ctx context.Context) error {
	soc, socGetErr := opc.scyllaOperatorConfigLister.Get(naming.SingletonName)
	if socGetErr != nil {
		if !apierrors.IsNotFound(socGetErr) {
			return fmt.Errorf("can't get ScyllaOperatorConfig %q: %w", naming.SingletonName, socGetErr)
		}

		klog.V(2).InfoS("ScyllaOperatorConfig missing, creating a default one")

		_, createErr := opc.scyllaClient.ScyllaOperatorConfigs().Create(
			ctx,
			&scyllav1alpha1.ScyllaOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: naming.SingletonName,
				},
				// Do not set any default values into the spec so they can be auto defaulted to newer ones
				// when the operator is upgraded. The default values are projected into the status for consumption.
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{},
			},
			metav1.CreateOptions{},
		)
		if createErr != nil {
			return fmt.Errorf("can't create scyllaoperatorconfig %q: %w", naming.SingletonName, createErr)
		}

		klog.V(2).InfoS("Create ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(soc))

		// We need to wait for caches to see the new object.
		return nil
	}

	status := opc.calculateStatus(soc)

	if soc.DeletionTimestamp != nil {
		return opc.updateStatus(ctx, soc, status)
	}

	var errs []error

	err := controllerhelpers.RunSync(
		&status.Conditions,
		clusterDomainControllerProgressingCondition,
		clusterDomainControllerDegradedCondition,
		soc.Generation,
		func() ([]metav1.Condition, error) {
			return opc.syncClusterDomain(ctx, soc, status)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync cluster domain: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, soc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = opc.updateStatus(ctx, soc, status)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't update status: %w", err))
		}
	}

	return utilerrors.NewAggregate(errs)
}
