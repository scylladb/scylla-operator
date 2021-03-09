package v1

import corev1 "k8s.io/api/core/v1"

func SetRackCondition(rackStatus *RackStatus, newCondition RackConditionType) {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == newCondition {
			rackStatus.Conditions[i].Status = corev1.ConditionTrue
			return
		}
	}
	rackStatus.Conditions = append(
		rackStatus.Conditions,
		RackCondition{Type: newCondition, Status: corev1.ConditionTrue},
	)
}

// FindCRDCondition returns the condition you're looking for or nil
func IsRackConditionTrue(rackStatus *RackStatus, conditionType RackConditionType) bool {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == conditionType {
			return rackStatus.Conditions[i].Status == corev1.ConditionTrue
		}
	}
	return false
}
