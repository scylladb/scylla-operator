// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sdc *Controller) syncRoleBindings(
	ctx context.Context,
	sd *scyllav1alpha1.ScyllaDatacenter,
	roleBindings map[string]*rbacv1.RoleBinding,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredRoleBinding := MakeRoleBinding(sd)

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
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, rb, "delete", sd.Generation)
		err = sdc.kubeClient.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &rb.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete role binding(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyRoleBinding(ctx, sdc.kubeClient.RbacV1(), sdc.roleBindingLister, sdc.eventRecorder, requiredRoleBinding, true, false)
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, requiredRoleBinding, "apply", sd.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply role binding: %w", err)
	}

	return progressingConditions, nil
}
