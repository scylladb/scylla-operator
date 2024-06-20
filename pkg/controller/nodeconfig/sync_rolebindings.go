// Copyright (C) 2024 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) makeRoleBindings() []*rbacv1.RoleBinding {
	roleBindings := []*rbacv1.RoleBinding{
		makePerftuneRoleBinding(),
	}

	return roleBindings
}

func (ncc *Controller) pruneRoleBindings(ctx context.Context, requiredRoleBindings []*rbacv1.RoleBinding, roleBindings map[string]*rbacv1.RoleBinding) error {
	var errs []error
	for _, rb := range roleBindings {
		if rb.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredRoleBindings {
			if rb.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &rb.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncRoleBindings(
	ctx context.Context,
	roleBindings map[string]*rbacv1.RoleBinding,
) error {
	requiredRoleBindings := ncc.makeRoleBindings()

	// Delete any excessive RoleBindings.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneRoleBindings(ctx, requiredRoleBindings, roleBindings); err != nil {
		return fmt.Errorf("can't delete RoleBinding(s): %w", err)
	}

	var errs []error
	for _, crb := range requiredRoleBindings {
		_, _, err := resourceapply.ApplyRoleBinding(ctx, ncc.kubeClient.RbacV1(), ncc.roleBindingLister, ncc.eventRecorder, crb, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing rolebinding: %w", err))
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}
