package scylladbdatacenter

import (
	"context"
)

// waitForAllStatefulSetsRollout waits for all racks to be up and ready before any of them is upgraded or updated.
// TODO: This blocks unstucking by an update. Also blocks lowering resources when the cluster is running low.
func (sdcc *Controller) waitForAllStatefulSetsRollout(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	for _, req := range sc.requiredStatefulSets {
		sts := sc.existingStatefulSets[req.Name]

		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, sts)
		if err != nil {
			return proceed(), err
		}

		if cond != nil {
			return blockWith(*cond), nil
		}
	}

	return proceed(), nil
}
