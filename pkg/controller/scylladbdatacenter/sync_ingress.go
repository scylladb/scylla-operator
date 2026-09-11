package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sdcc *Controller) syncIngresses(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	ingresses map[string]*networkingv1.Ingress,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredIngresses := MakeIngresses(sdc, services)

	// Delete any excessive Ingresses.
	// Delete has to be the fist action to avoid getting stuck on quota.
	prunedIngresses, err := controllerhelpers.PruneObjects(
		ctx,
		requiredIngresses,
		ingresses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.kubeClient.NetworkingV1().Ingresses(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	for _, ingress := range prunedIngresses {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, ingressControllerProgressingCondition, ingress, "delete", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete ingress(s): %w", err)
	}

	for _, requiredIngress := range requiredIngresses {
		_, changed, err := resourceapply.ApplyIngress(ctx, sdcc.kubeClient.NetworkingV1(), sdcc.ingressLister, sdcc.eventRecorder, requiredIngress, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, ingressControllerProgressingCondition, requiredIngress, "apply", sdc.Generation)
		}
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply ingress: %w", err)
		}
	}

	return progressingConditions, nil
}
