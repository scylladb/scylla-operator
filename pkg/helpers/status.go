package helpers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsStatusConditionPresentAndTrue(conditions []metav1.Condition, conditionType string, generation int64) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, metav1.ConditionTrue, generation)
}

func IsStatusConditionPresentAndFalse(conditions []metav1.Condition, conditionType string, generation int64) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, metav1.ConditionFalse, generation)
}

func IsStatusConditionPresentAndEqual(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus, generation int64) bool {
	for _, condition := range conditions {
		if condition.Type != conditionType {
			continue
		}

		if condition.ObservedGeneration != generation {
			return false
		}

		return condition.Status == status
	}

	return false
}
