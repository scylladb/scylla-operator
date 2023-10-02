package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncPodDisruptionBudgets(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	status *scyllav1.ScyllaClusterStatus,
	pdbs map[string]*policyv1.PodDisruptionBudget,
) (*scyllav1.ScyllaClusterStatus, error) {
	var err error

	requiredPDB := MakePodDisruptionBudget(sc)

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
		err = scc.kubeClient.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Delete(ctx, pdb.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &pdb.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return status, fmt.Errorf("can't delete pdb(s): %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, _, err = resourceapply.ApplyPodDisruptionBudget(ctx, scc.kubeClient.PolicyV1(), scc.pdbLister, scc.eventRecorder, requiredPDB, true)
	if err != nil {
		return status, fmt.Errorf("can't apply pdb: %w", err)
	}

	return status, nil
}
