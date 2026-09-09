package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
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
		_, changed, err := resourceapply.ApplyConfigMapWithControl(ctx, ctrlclient.ApplyControl[corev1.ConfigMap](ctx, sdcc.client, sdc.Namespace), sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, configControllerProgressingCondition, cm, "apply", sdc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply configmap %q: %w", naming.ObjRef(cm), err))
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
