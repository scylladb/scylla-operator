package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
)

func (sdcc *Controller) syncConfigs(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	requiredConfigMaps, err := MakeManagedScyllaDBConfigMaps(sdc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make managed scylladb config: %w", err)
	}

	for _, cm := range requiredConfigMaps {
		_, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, configControllerProgressingCondition, cm, "apply", sdc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply configmap %q: %w", naming.ObjRef(cm), err))
		}
	}

	return progressingConditions, errors.NewAggregate(errs)
}
