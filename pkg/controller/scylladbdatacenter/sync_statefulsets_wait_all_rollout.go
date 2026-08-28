package scylladbdatacenter

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// waitForAllStatefulSetsRollout waits for all racks to be up and ready before any of them is upgraded or updated.
// TODO: This blocks unstucking by an update. Also blocks lowering resources when the cluster is running low.
func (sdcc *Controller) waitForAllStatefulSetsRollout(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	for _, req := range sc.requiredStatefulSets {
		sts := sc.existingStatefulSets[req.Name]

		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, sts)
		if err != nil {
			return nil, err
		}

		if cond != nil {
			return []metav1.Condition{*cond}, nil
		}
	}

	return nil, nil
}
