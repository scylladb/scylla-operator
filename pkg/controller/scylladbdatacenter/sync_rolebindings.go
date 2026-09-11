package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	var deletionErrors []error
	for _, rb := range roleBindings {
		if rb.DeletionTimestamp != nil {
			continue
		}

		if rb.Name == requiredRoleBinding.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, rb, "delete", sdc.Generation)
		err = sdcc.client.Delete(ctx, rb, client.Preconditions{UID: &rb.UID}, client.PropagationPolicy(propagationPolicy))
		deletionErrors = append(deletionErrors, err)
	}
	err = apimachineryutilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete role binding(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyRoleBindingWithControl(ctx, ctrlclient.ApplyControl[rbacv1.RoleBinding](ctx, sdcc.client, sdc.Namespace), sdcc.eventRecorder, requiredRoleBinding, resourceapply.ApplyOptions{
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
