package scylladbdatacenter

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// waitForExistingStatefulSetsRollout returns progressing conditions for the existing StatefulSets that haven't rolled
// out yet, so that racks bootstrap one by one. StatefulSets about to be scaled are skipped: when a member is
// decommissioned there is a Pod left that's not ready until the scale happens.
func (sdcc *Controller) waitForExistingStatefulSetsRollout(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	for _, req := range sc.requiredStatefulSets {
		sts, ok := sc.existingStatefulSets[req.Name]
		if !ok {
			continue
		}

		if req.Spec.Replicas != nil && sts.Spec.Replicas != nil &&
			*req.Spec.Replicas != *sts.Spec.Replicas {
			continue
		}

		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, sts)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if cond != nil {
			progressingConditions = append(progressingConditions, *cond)
		}
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return blockWith(progressingConditions...), fmt.Errorf("can't check existing statefulset(s) rollout status: %w", err)
	}

	return blockWith(progressingConditions...), nil
}
