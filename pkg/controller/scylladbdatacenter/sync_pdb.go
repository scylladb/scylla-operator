package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (sdcc *Controller) syncPodDisruptionBudgets(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	pdbs map[string]*policyv1.PodDisruptionBudget,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredPDB := MakePodDisruptionBudget(sdc)

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
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, pdbControllerProgressingCondition, pdb, "delete", sdc.Generation)
		err = sdcc.client.Delete(ctx, pdb, client.Preconditions{UID: &pdb.UID}, client.PropagationPolicy(propagationPolicy))
		deletionErrors = append(deletionErrors, err)
	}
	err = apimachineryutilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete pdb(s): %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, changed, err := resourceapply.ApplyPodDisruptionBudgetWithControl(ctx, ctrlclient.ApplyControl[policyv1.PodDisruptionBudget](ctx, sdcc.client, sdc.Namespace), sdcc.eventRecorder, requiredPDB, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, pdbControllerProgressingCondition, requiredPDB, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply pdb: %w", err)
	}

	return progressingConditions, nil
}
