package managerv2

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (smc *Controller) syncPodDisruptionBudgets(
	ctx context.Context,
	sm *v1alpha1.ScyllaManager,
	pdbs map[string]*policyv1.PodDisruptionBudget,
) error {
	var err error

	requiredPDB := MakePodDisruptionBudget(sm)

	// Delete any excessive PodDisruptionBudgets.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, pdb := range pdbs {
		if pdb.DeletionTimestamp != nil {
			continue
		}

		if pdb.Name == requiredPDB.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = smc.kubeClient.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Delete(ctx, pdb.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &pdb.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete pdb(s): %w", err)
	}

	_, _, err = resourceapply.ApplyPodDisruptionBudget(ctx, smc.kubeClient.PolicyV1(), smc.pdbLister, smc.eventRecorder, requiredPDB, false)
	if err != nil {
		return fmt.Errorf("can't apply pdb: %w", err)
	}

	return nil
}
