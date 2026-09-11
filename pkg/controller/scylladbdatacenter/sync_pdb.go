package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	prunedPodDisruptionBudgets, err := controllerhelpers.PruneObjects(
		ctx,
		[]*policyv1.PodDisruptionBudget{requiredPDB},
		pdbs,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.kubeClient.PolicyV1().PodDisruptionBudgets(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	for _, pdb := range prunedPodDisruptionBudgets {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, pdbControllerProgressingCondition, pdb, "delete", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete pdb(s): %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, changed, err := resourceapply.ApplyPodDisruptionBudget(ctx, sdcc.kubeClient.PolicyV1(), sdcc.pdbLister, sdcc.eventRecorder, requiredPDB, resourceapply.ApplyOptions{
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
