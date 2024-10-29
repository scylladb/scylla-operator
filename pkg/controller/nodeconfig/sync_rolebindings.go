// Copyright (C) 2024 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) syncRoleBindings(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	roleBindings map[string]*rbacv1.RoleBinding,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredRoleBindings := []*rbacv1.RoleBinding{
		makePerftuneRoleBinding(),
		makeRlimitsRoleBinding(),
	}

	// Delete any excessive RoleBindings.
	// Delete has to be the first action to avoid getting stuck on quota.
	err := controllerhelpers.Prune(
		ctx,
		requiredRoleBindings,
		roleBindings,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ncc.kubeClient.RbacV1().RoleBindings(naming.ScyllaOperatorNodeTuningNamespace).Delete,
		},
		ncc.eventRecorder)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune RoleBinding(s): %w", err)
	}

	var errs []error
	for _, rb := range requiredRoleBindings {
		_, changed, err := resourceapply.ApplyRoleBinding(ctx, ncc.kubeClient.RbacV1(), ncc.roleBindingLister, ncc.eventRecorder, rb, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, rb, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing rolebinding: %w", err))
			continue
		}
	}
	return progressingConditions, utilerrors.NewAggregate(errs)
}
