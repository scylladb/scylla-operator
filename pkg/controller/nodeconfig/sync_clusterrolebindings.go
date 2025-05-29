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
	return apimachineryutilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncClusterRoleBindings(ctx context.Context, nc *scyllav1alpha1.NodeConfig, clusterRoleBindings map[string]*rbacv1.ClusterRoleBinding) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredClusterRoleBindings := ncc.makeClusterRoleBindings()

	// Delete any excessive ClusterRoleBindings.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneClusterRoleBindings(ctx, requiredClusterRoleBindings, clusterRoleBindings); err != nil {
		return progressingConditions, fmt.Errorf("can't delete ClusterRoleBinding(s): %w", err)
	}

	var errs []error
	for _, crb := range requiredClusterRoleBindings {
		_, changed, err := resourceapply.ApplyClusterRoleBinding(ctx, ncc.kubeClient.RbacV1(), ncc.clusterRoleBindingLister, ncc.eventRecorder, crb, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, clusterRoleBindingControllerProgressingCondition, crb, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing clusterrole: %w", err))
			continue
		}
	}
	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
