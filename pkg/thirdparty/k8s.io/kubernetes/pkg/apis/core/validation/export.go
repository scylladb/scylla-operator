// Copyright (c) 2025 ScyllaDB.

package validation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidatePodAffinity(podAffinity *corev1.PodAffinity, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	return validatePodAffinity(podAffinity, allowInvalidLabelValueInSelector, fldPath)
}

func ValidateNodeAffinity(na *corev1.NodeAffinity, fldPath *field.Path) field.ErrorList {
	return validateNodeAffinity(na, fldPath)
}

func ValidatePodAntiAffinity(podAntiAffinity *corev1.PodAntiAffinity, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	return validatePodAntiAffinity(podAntiAffinity, allowInvalidLabelValueInSelector, fldPath)
}
