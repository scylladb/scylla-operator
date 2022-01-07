package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncRoleBindings(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	roleBindings map[string]*rbacv1.RoleBinding,
) error {
	var err error

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
		return fmt.Errorf("can't delete role binding(s): %w", err)
	}

	_, _, err = resourceapply.ApplyRoleBinding(ctx, scc.kubeClient.RbacV1(), scc.roleBindingLister, scc.eventRecorder, requiredRoleBinding, true, false)
	if err != nil {
		return fmt.Errorf("can't apply role binding: %w", err)
	}

	return nil
}
