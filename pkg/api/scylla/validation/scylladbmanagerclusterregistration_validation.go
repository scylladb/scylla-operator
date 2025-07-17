// Copyright (C) 2025 ScyllaDB

package validation

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	scyllaDBManagerClusterRegistrationSupportedLocalScyllaDBReferenceKinds = []string{
		scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
		scyllav1alpha1.ScyllaDBClusterGVK.Kind,
	}
)

func ValidateScyllaDBManagerClusterRegistration(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationObjectMeta(&smcr.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationSpec(&smcr.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationObjectMeta(objectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	globalScyllaDBManagerLabelValue, hasGlobalScyllaDBManagerLabel := objectMeta.Labels[naming.GlobalScyllaDBManagerLabel]
	if !hasGlobalScyllaDBManagerLabel {
		allErrs = append(allErrs, field.Required(fldPath.Child("labels").Key(naming.GlobalScyllaDBManagerLabel), ""))
	} else if globalScyllaDBManagerLabelValue != naming.LabelValueTrue {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("labels").Key(naming.GlobalScyllaDBManagerLabel), globalScyllaDBManagerLabelValue, fmt.Sprintf("must be %q", naming.LabelValueTrue)))
	}

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationSpec(spec *scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateLocalScyllaDBReference(&spec.ScyllaDBClusterRef, scyllaDBManagerClusterRegistrationSupportedLocalScyllaDBReferenceKinds, fldPath.Child("scyllaDBClusterRef"))...)

	return allErrs
}

func ValidateLocalScyllaDBReference(localScyllaDBReference *scyllav1alpha1.LocalScyllaDBReference, supportedLocalScyllaDBReferenceKinds []string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(localScyllaDBReference.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	} else {
		for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(localScyllaDBReference.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), localScyllaDBReference.Name, msg))
		}
	}

	if len(localScyllaDBReference.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	} else {
		allErrs = append(allErrs, validateEnum(localScyllaDBReference.Kind, supportedLocalScyllaDBReferenceKinds, fldPath.Child("kind"))...)
	}

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationUpdate(new, old *scyllav1alpha1.ScyllaDBManagerClusterRegistration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistration(new)...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationObjectMetaUpdate(newObjectMeta, oldObjectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newObjectMeta.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation], oldObjectMeta.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation], fldPath.Child("annotations").Key(naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationSpecUpdate(newSpec, oldSpec *scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.ScyllaDBClusterRef.Kind, oldSpec.ScyllaDBClusterRef.Kind, fldPath.Child("scyllaDBClusterRef", "kind"))...)
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.ScyllaDBClusterRef.Name, oldSpec.ScyllaDBClusterRef.Name, fldPath.Child("scyllaDBClusterRef", "name"))...)

	return allErrs
}
