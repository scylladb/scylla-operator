package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncServiceAccounts(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	serviceAccounts map[string]*corev1.ServiceAccount,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredServiceAccount := MakeServiceAccount(sc)

	// Delete any excessive ServiceAccounts.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, sa := range serviceAccounts {
		if sa.DeletionTimestamp != nil {
			continue
		}

		if sa.Name == requiredServiceAccount.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceAccountControllerProgressingCondition, sa, "delete", sc.Generation)
		err = scc.kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Delete(ctx, sa.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sa.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete service account(s): %w", err)
	}

	_, changed, err := resourceapply.ApplyServiceAccount(ctx, scc.kubeClient.CoreV1(), scc.serviceAccountLister, scc.eventRecorder, requiredServiceAccount, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceAccountControllerProgressingCondition, requiredServiceAccount, "apply", sc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply service account: %w", err)
	}

	return progressingConditions, nil
}
