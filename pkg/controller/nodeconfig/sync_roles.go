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

func (ncc *Controller) makeRoles() []*rbacv1.Role {
	Roles := []*rbacv1.Role{
		makePerftuneRole(),
	}

	return Roles
}

func (ncc *Controller) pruneRoles(ctx context.Context, requiredRoles []*rbacv1.Role, Roles map[string]*rbacv1.Role) error {
	var errs []error
	for _, r := range Roles {
		if r.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredRoles {
			if r.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.RbacV1().Roles(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &r.UID,
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

func (ncc *Controller) syncRoles(ctx context.Context, Roles map[string]*rbacv1.Role) error {
	requiredRoles := ncc.makeRoles()

	// Delete any excessive Roles.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneRoles(ctx, requiredRoles, Roles); err != nil {
		return fmt.Errorf("can't delete Role(s): %w", err)
	}

	var errs []error
	for _, cr := range requiredRoles {
		_, _, err := resourceapply.ApplyRole(ctx, ncc.kubeClient.RbacV1(), ncc.roleLister, ncc.eventRecorder, cr, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing Role: %w", err))
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}
