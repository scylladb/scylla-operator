// Copyright (c) 2023 ScyllaDB.

package validation

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	imgreference "github.com/containers/image/v5/docker/reference"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	apimachineryvalidationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateImageRef(imageRef string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(imageRef) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "image reference can't be empty"))
	} else {
		_, err := imgreference.Parse(imageRef)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, imageRef, fmt.Sprintf("unable to parse image: %v", err)))
		}
	}

	return allErrs
}

func ValidateSemanticVersion(v string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(v) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "version can't be empty"))
	} else if len(strings.TrimSpace(v)) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, v, "version contains only spaces"))
	} else {
		_, err := semver.ParseTolerant(v)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, v, fmt.Sprintf("unable to parse version: %v", err)))
		}
	}

	return allErrs
}

func ValidateScyllaOperatorConfig(nc *scyllav1alpha1.ScyllaOperatorConfig) field.ErrorList {
	return ValidateScyllaOperatorConfigSpec(&nc.Spec, field.NewPath("spec"))
}

func ValidateScyllaOperatorConfigSpec(spec *scyllav1alpha1.ScyllaOperatorConfigSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.UnsupportedBashToolsImageOverride != nil {
		allErrs = append(allErrs, ValidateImageRef(*spec.UnsupportedBashToolsImageOverride, fldPath.Child("unsupportedBashToolsImageOverride"))...)
	}

	if spec.UnsupportedGrafanaImageOverride != nil {
		allErrs = append(allErrs, ValidateImageRef(*spec.UnsupportedGrafanaImageOverride, fldPath.Child("unsupportedGrafanaImageOverride"))...)
	}

	if spec.UnsupportedPrometheusVersionOverride != nil {
		allErrs = append(allErrs, ValidateSemanticVersion(*spec.UnsupportedPrometheusVersionOverride, fldPath.Child("unsupportedPrometheusVersionOverride"))...)
	}

	if spec.ConfiguredClusterDomain != nil {
		allErrs = append(allErrs, apimachineryvalidationutils.IsFullyQualifiedDomainName(fldPath.Child("configuredClusterDomain"), *spec.ConfiguredClusterDomain)...)
		for _, msg := range apimachineryvalidationutils.IsValidLabelValue(*spec.ConfiguredClusterDomain) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("configuredClusterDomain"), *spec.ConfiguredClusterDomain, msg))
		}
	}

	return allErrs
}

func ValidateScyllaOperatorConfigUpdate(new, old *scyllav1alpha1.ScyllaOperatorConfig) field.ErrorList {
	allErrs := ValidateScyllaOperatorConfig(new)

	return allErrs
}
