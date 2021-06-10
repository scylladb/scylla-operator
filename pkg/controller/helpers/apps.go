package helpers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
)

func IsStatefulSetRolledOut(sts *appsv1.StatefulSet) (bool, error) {
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		return false, fmt.Errorf("can't determine rollout status for %s strategy type", sts.Spec.UpdateStrategy.Type)
	}

	if sts.Spec.Replicas == nil {
		// This should never happen, but better safe then sorry.
		return false, fmt.Errorf("statefulset.spec.replicas can't be nil")
	}

	if sts.Spec.UpdateStrategy.RollingUpdate == nil {
		// This should never happen, but better safe then sorry.
		return false, fmt.Errorf("statefulset.spec.updatestrategy.rollingupdate can't be nil")
	}

	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return false, nil
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return false, nil
	}

	if sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		return sts.Status.UpdatedReplicas == (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition), nil
	} else {
		return sts.Status.UpdateRevision == sts.Status.CurrentRevision, nil
	}
}
