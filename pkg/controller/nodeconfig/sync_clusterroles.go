// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) makeClusterRoles() []*rbacv1.ClusterRole {
	clusterRoles := []*rbacv1.ClusterRole{
		NodeConfigClusterRole(),
	}

	return clusterRoles
}

func (ncc *Controller) pruneClusterRoles(ctx context.Context, requiredClusterRoles []*rbacv1.ClusterRole, clusterRoles map[string]*rbacv1.ClusterRole) error {
	var errs []error
	for _, cr := range clusterRoles {
		if cr.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredClusterRoles {
			if cr.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.RbacV1().ClusterRoles().Delete(ctx, cr.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &cr.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return apimachineryutilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncClusterRoles(ctx context.Context, nc *scyllav1alpha1.NodeConfig, clusterRoles map[string]*rbacv1.ClusterRole) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredClusterRoles := ncc.makeClusterRoles()

	// Delete any excessive ClusterRoles.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneClusterRoles(ctx, requiredClusterRoles, clusterRoles); err != nil {
		return progressingConditions, fmt.Errorf("can't delete ClusterRole(s): %w", err)
	}

	var errs []error
	for _, cr := range requiredClusterRoles {
		_, changed, err := resourceapply.ApplyClusterRole(ctx, ncc.kubeClient.RbacV1(), ncc.clusterRoleLister, ncc.eventRecorder, cr, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, clusterRoleControllerProgressingCondition, cr, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing clusterrole: %w", err))
			continue
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
