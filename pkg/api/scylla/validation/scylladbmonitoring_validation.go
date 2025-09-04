package validation

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateScyllaDBMonitoring(sm *scyllav1alpha1.ScyllaDBMonitoring) field.ErrorList {
	return nil
}

func ValidateScyllaDBMonitoringUpdate(new, old *scyllav1alpha1.ScyllaDBMonitoring) field.ErrorList {
	var allErrs field.ErrorList

	// 1. Spec.Components.Prometheus.Mode cannot be changed

	if err := ValidateScyllaDBMonitoring(new); err != nil {
		allErrs = append(allErrs, err...)
	}

	return nil
}

func validateScyllaDBMonitoringSpec(sm *scyllav1alpha1.ScyllaDBMonitoringSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	return allErrs
}

func validateScyllaDBMonitoringSpecUpdate(new, old *scyllav1alpha1.ScyllaDBMonitoringSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	return allErrs
}

func validateScyllaDBMonitoringComponentsUpdate(new, old *scyllav1alpha1.Components, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if new.Prometheus != nil && old.Prometheus != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringSpecComponentsPrometheusUpdate(new.Prometheus, old.Prometheus, fldPath.Child("prometheus"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringSpecComponentsPrometheusUpdate(new, old *scyllav1alpha1.PrometheusSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if new.Mode != old.Mode {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mode"), old.Mode, "is immutable and cannot be changed"))
	}

	return allErrs
}

func GetWarningsOnScyllaDBMonitoringCreate(sm *scyllav1alpha1.ScyllaDBMonitoring) []string {
	return nil
}

func GetWarningsOnScyllaDBMonitoringUpdate(new, old *scyllav1alpha1.ScyllaDBMonitoring) []string {
	return nil
}
