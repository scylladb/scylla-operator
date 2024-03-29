// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) makeClusterRoleBindings() []*rbacv1.ClusterRoleBinding {
	clusterRoleBindings := []*rbacv1.ClusterRoleBinding{
		makeNodeConfigClusterRoleBinding(),
	}

	return clusterRoleBindings
}

func (ncc *Controller) pruneClusterRoleBindings(ctx context.Context, requiredClusterRoleBindings []*rbacv1.ClusterRoleBinding, clusterRoleBindings map[string]*rbacv1.ClusterRoleBinding) error {
	var errs []error
	for _, cr := range clusterRoleBindings {
		if cr.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredClusterRoleBindings {
			if cr.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.RbacV1().ClusterRoleBindings().Delete(ctx, cr.Name, metav1.DeleteOptions{
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
	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncClusterRoleBindings(
	ctx context.Context,
	clusterRoleBindings map[string]*rbacv1.ClusterRoleBinding,
) error {
	requiredClusterRoleBindings := ncc.makeClusterRoleBindings()

	// Delete any excessive ClusterRoleBindings.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneClusterRoleBindings(ctx, requiredClusterRoleBindings, clusterRoleBindings); err != nil {
		return fmt.Errorf("can't delete ClusterRoleBinding(s): %w", err)
	}

	var errs []error
	for _, crb := range requiredClusterRoleBindings {
		_, _, err := resourceapply.ApplyClusterRoleBinding(ctx, ncc.kubeClient.RbacV1(), ncc.clusterRoleBindingLister, ncc.eventRecorder, crb, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing clusterrole: %w", err))
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}
