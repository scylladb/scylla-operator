package helpers

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

func IsStatusNodeConfigConditionPresentAndTrue(conditions []scyllav1alpha1.NodeConfigCondition, conditionType scyllav1alpha1.NodeConfigConditionType, generation int64) bool {
	return IsStatusNodeConfigConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue, generation)
}

func IsStatusNodeConfigConditionPresentAndFalse(conditions []scyllav1alpha1.NodeConfigCondition, conditionType scyllav1alpha1.NodeConfigConditionType, generation int64) bool {
	return IsStatusNodeConfigConditionPresentAndEqual(conditions, conditionType, corev1.ConditionFalse, generation)
}

func IsStatusNodeConfigConditionPresentAndEqual(conditions []scyllav1alpha1.NodeConfigCondition, conditionType scyllav1alpha1.NodeConfigConditionType, status corev1.ConditionStatus, generation int64) bool {
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
