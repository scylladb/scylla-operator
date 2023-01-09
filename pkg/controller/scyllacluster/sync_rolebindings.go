package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncRoleBindings(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	roleBindings map[string]*rbacv1.RoleBinding,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredRoleBinding := MakeRoleBinding(sc)

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
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, rb, "delete", sc.Generation)
		err = scc.kubeClient.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{
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

	_, changed, err := resourceapply.ApplyRoleBinding(ctx, scc.kubeClient.RbacV1(), scc.roleBindingLister, scc.eventRecorder, requiredRoleBinding, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, roleBindingControllerProgressingCondition, requiredRoleBinding, "apply", sc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply role binding: %w", err)
	}

	return progressingConditions, nil
}
