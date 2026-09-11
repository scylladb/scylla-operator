package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sdcc *Controller) syncRoleBindings(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	roleBindings map[string]*rbacv1.RoleBinding,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredRoleBinding := MakeRoleBinding(sdc)

	// Delete any excessive RoleBindings.
	// Delete has to be the fist action to avoid getting stuck on quota.
	prunedRoleBindings, err := controllerhelpers.PruneObjects(
		ctx,
		[]*rbacv1.RoleBinding{requiredRoleBinding},
		roleBindings,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.kubeClient.RbacV1().RoleBindings(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	for _, rb := range prunedRoleBindings {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, rb, "delete", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete role binding(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyRoleBinding(ctx, sdcc.kubeClient.RbacV1(), sdcc.roleBindingLister, sdcc.eventRecorder, requiredRoleBinding, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, requiredRoleBinding, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply role binding: %w", err)
	}

	return progressingConditions, nil
}
