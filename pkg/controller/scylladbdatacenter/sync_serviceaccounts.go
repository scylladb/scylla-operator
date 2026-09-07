package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sdcc *Controller) syncServiceAccounts(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	serviceAccounts map[string]*corev1.ServiceAccount,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredServiceAccount := MakeServiceAccount(sdc)

	// Delete any excessive ServiceAccounts.
	// Delete has to be the fist action to avoid getting stuck on quota.
	prunedServiceAccounts, err := controllerhelpers.PruneObjects(
		ctx,
		[]*corev1.ServiceAccount{requiredServiceAccount},
		serviceAccounts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.kubeClient.CoreV1().ServiceAccounts(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	for _, sa := range prunedServiceAccounts {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceAccountControllerProgressingCondition, sa, "delete", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete service account(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyServiceAccount(ctx, sdcc.kubeClient.CoreV1(), sdcc.serviceAccountLister, sdcc.eventRecorder, requiredServiceAccount, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceAccountControllerProgressingCondition, requiredServiceAccount, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply service account: %w", err)
	}

	return progressingConditions, nil
}
