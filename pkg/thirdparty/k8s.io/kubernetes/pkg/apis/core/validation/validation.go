/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNodeName can be used to check whether the given node name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidateNodeName = apimachineryvalidation.NameIsDNSSubdomain

// ValidateNamespaceName can be used to check whether the given namespace name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidateNamespaceName = apimachineryvalidation.ValidateNamespaceName

var nodeFieldSelectorValidators = map[string]func(string, bool) []string{
	metav1.ObjectNameField: ValidateNodeName,
}

// ValidateNodeSelectorRequirement tests that the specified NodeSelectorRequirement fields has valid data
func ValidateNodeSelectorRequirement(rq corev1.NodeSelectorRequirement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	switch rq.Operator {
	case corev1.NodeSelectorOpIn, corev1.NodeSelectorOpNotIn:
		if len(rq.Values) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("values"), "must be specified when `operator` is 'In' or 'NotIn'"))
		}
	case corev1.NodeSelectorOpExists, corev1.NodeSelectorOpDoesNotExist:
		if len(rq.Values) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("values"), "may not be specified when `operator` is 'Exists' or 'DoesNotExist'"))
		}

	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		if len(rq.Values) != 1 {
			allErrs = append(allErrs, field.Required(fldPath.Child("values"), "must be specified single value when `operator` is 'Lt' or 'Gt'"))
		}
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("operator"), rq.Operator, "not a valid selector operator"))
	}

	allErrs = append(allErrs, unversionedvalidation.ValidateLabelName(rq.Key, fldPath.Child("key"))...)

	return allErrs
}

// ValidateNodeFieldSelectorRequirement tests that the specified NodeSelectorRequirement fields has valid data
func ValidateNodeFieldSelectorRequirement(req corev1.NodeSelectorRequirement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch req.Operator {
	case corev1.NodeSelectorOpIn, corev1.NodeSelectorOpNotIn:
		if len(req.Values) != 1 {
			allErrs = append(allErrs, field.Required(fldPath.Child("values"),
				"must be only one value when `operator` is 'In' or 'NotIn' for node field selector"))
		}
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("operator"), req.Operator, "not a valid selector operator"))
	}

	if vf, found := nodeFieldSelectorValidators[req.Key]; !found {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("key"), req.Key, "not a valid field selector key"))
	} else {
		for i, v := range req.Values {
			for _, msg := range vf(v, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("values").Index(i), v, msg))
			}
		}
	}

	return allErrs
}

// ValidateNodeSelectorTerm tests that the specified node selector term has valid data
func ValidateNodeSelectorTerm(term corev1.NodeSelectorTerm, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for j, req := range term.MatchExpressions {
		allErrs = append(allErrs, ValidateNodeSelectorRequirement(req, fldPath.Child("matchExpressions").Index(j))...)
	}

	for j, req := range term.MatchFields {
		allErrs = append(allErrs, ValidateNodeFieldSelectorRequirement(req, fldPath.Child("matchFields").Index(j))...)
	}

	return allErrs
}

// ValidateNodeSelector tests that the specified nodeSelector fields has valid data
func ValidateNodeSelector(nodeSelector *corev1.NodeSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	termFldPath := fldPath.Child("nodeSelectorTerms")
	if len(nodeSelector.NodeSelectorTerms) == 0 {
		return append(allErrs, field.Required(termFldPath, "must have at least one node selector term"))
	}

	for i, term := range nodeSelector.NodeSelectorTerms {
		allErrs = append(allErrs, ValidateNodeSelectorTerm(term, termFldPath.Index(i))...)
	}

	return allErrs
}

// ValidatePreferredSchedulingTerms tests that the specified SoftNodeAffinity fields has valid data
func ValidatePreferredSchedulingTerms(terms []corev1.PreferredSchedulingTerm, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, term := range terms {
		if term.Weight <= 0 || term.Weight > 100 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("weight"), term.Weight, "must be in the range 1-100"))
		}

		allErrs = append(allErrs, ValidateNodeSelectorTerm(term.Preference, fldPath.Index(i).Child("preference"))...)
	}
	return allErrs
}

func ValidatePodAffinityTermSelector(podAffinityTerm corev1.PodAffinityTerm, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	labelSelectorValidationOptions := unversionedvalidation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: allowInvalidLabelValueInSelector}
	allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(podAffinityTerm.LabelSelector, labelSelectorValidationOptions, fldPath.Child("labelSelector"))...)
	allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(podAffinityTerm.NamespaceSelector, labelSelectorValidationOptions, fldPath.Child("namespaceSelector"))...)
	return allErrs
}

// validateMatchLabelKeysAndMismatchLabelKeys checks if both matchLabelKeys and mismatchLabelKeys are valid.
// - validate that all matchLabelKeys and mismatchLabelKeys are valid label names.
// - validate that the user doens't specify the same key in both matchLabelKeys and labelSelector.
// - validate that any matchLabelKeys are not duplicated with mismatchLabelKeys.
func validateMatchLabelKeysAndMismatchLabelKeys(fldPath *field.Path, matchLabelKeys, mismatchLabelKeys []string, labelSelector *metav1.LabelSelector) field.ErrorList {
	var allErrs field.ErrorList
	// 1. validate that all matchLabelKeys and mismatchLabelKeys are valid label names.
	allErrs = append(allErrs, validateLabelKeys(fldPath.Child("matchLabelKeys"), matchLabelKeys, labelSelector)...)
	allErrs = append(allErrs, validateLabelKeys(fldPath.Child("mismatchLabelKeys"), mismatchLabelKeys, labelSelector)...)

	// 2. validate that the user doens't specify the same key in both matchLabelKeys and labelSelector.
	// It doesn't make sense to have the labelselector with the key specified in matchLabelKeys
	// because the matchLabelKeys will be `In` labelSelector which matches with only one value in the key
	// and we cannot make any further filtering with that key.
	// On the other hand, we may want to have labelSelector with the key specified in mismatchLabelKeys.
	// because the mismatchLabelKeys will be `NotIn` labelSelector
	// and we may want to filter Pods further with other labelSelector with that key.

	// labelKeysMap is keyed by label key and valued by the index of label key in labelKeys.
	if labelSelector != nil {
		labelKeysMap := map[string]int{}
		for i, key := range matchLabelKeys {
			labelKeysMap[key] = i
		}
		labelSelectorKeys := sets.New[string]()
		for key := range labelSelector.MatchLabels {
			labelSelectorKeys.Insert(key)
		}
		for _, matchExpression := range labelSelector.MatchExpressions {
			key := matchExpression.Key
			if i, ok := labelKeysMap[key]; ok && labelSelectorKeys.Has(key) {
				// Before validateLabelKeysWithSelector is called, the labelSelector has already got the selector created from matchLabelKeys.
				// Here, we found the duplicate key in labelSelector and the key is specified in labelKeys.
				// Meaning that the same key is specified in both labelSelector and matchLabelKeys/mismatchLabelKeys.
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i), key, "exists in both matchLabelKeys and labelSelector"))
			}

			labelSelectorKeys.Insert(key)
		}
	}

	// 3. validate that any matchLabelKeys are not duplicated with mismatchLabelKeys.
	mismatchLabelKeysSet := sets.New(mismatchLabelKeys...)
	for i, k := range matchLabelKeys {
		if mismatchLabelKeysSet.Has(k) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("matchLabelKeys").Index(i), k, "exists in both matchLabelKeys and mismatchLabelKeys"))
		}
	}

	return allErrs
}

// validateLabelKeys tests that the label keys are a valid label name.
// It's intended to be used for matchLabelKeys or mismatchLabelKeys.
func validateLabelKeys(fldPath *field.Path, labelKeys []string, labelSelector *metav1.LabelSelector) field.ErrorList {
	if len(labelKeys) == 0 {
		return nil
	}

	if labelSelector == nil {
		return field.ErrorList{field.Forbidden(fldPath, "must not be specified when labelSelector is not set")}
	}

	var allErrs field.ErrorList
	for i, key := range labelKeys {
		allErrs = append(allErrs, unversionedvalidation.ValidateLabelName(key, fldPath.Index(i))...)
	}

	return allErrs
}

// validatePodAffinityTerm tests that the specified podAffinityTerm fields have valid data
func validatePodAffinityTerm(podAffinityTerm corev1.PodAffinityTerm, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidatePodAffinityTermSelector(podAffinityTerm, allowInvalidLabelValueInSelector, fldPath)...)
	for _, name := range podAffinityTerm.Namespaces {
		for _, msg := range ValidateNamespaceName(name, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), name, msg))
		}
	}
	allErrs = append(allErrs, validateMatchLabelKeysAndMismatchLabelKeys(fldPath, podAffinityTerm.MatchLabelKeys, podAffinityTerm.MismatchLabelKeys, podAffinityTerm.LabelSelector)...)
	if len(podAffinityTerm.TopologyKey) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("topologyKey"), "can not be empty"))
	}
	return append(allErrs, unversionedvalidation.ValidateLabelName(podAffinityTerm.TopologyKey, fldPath.Child("topologyKey"))...)
}

// validatePodAffinityTerms tests that the specified podAffinityTerms fields have valid data
func validatePodAffinityTerms(podAffinityTerms []corev1.PodAffinityTerm, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, podAffinityTerm := range podAffinityTerms {
		allErrs = append(allErrs, validatePodAffinityTerm(podAffinityTerm, allowInvalidLabelValueInSelector, fldPath.Index(i))...)
	}
	return allErrs
}

// validateWeightedPodAffinityTerms tests that the specified weightedPodAffinityTerms fields have valid data
func validateWeightedPodAffinityTerms(weightedPodAffinityTerms []corev1.WeightedPodAffinityTerm, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for j, weightedTerm := range weightedPodAffinityTerms {
		if weightedTerm.Weight <= 0 || weightedTerm.Weight > 100 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(j).Child("weight"), weightedTerm.Weight, "must be in the range 1-100"))
		}
		allErrs = append(allErrs, validatePodAffinityTerm(weightedTerm.PodAffinityTerm, allowInvalidLabelValueInSelector, fldPath.Index(j).Child("podAffinityTerm"))...)
	}
	return allErrs
}

// validatePodAntiAffinity tests that the specified podAntiAffinity fields have valid data
func validatePodAntiAffinity(podAntiAffinity *corev1.PodAntiAffinity, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO:Uncomment below code once RequiredDuringSchedulingRequiredDuringExecution is implemented.
	// if podAntiAffinity.RequiredDuringSchedulingRequiredDuringExecution != nil {
	//	allErrs = append(allErrs, validatePodAffinityTerms(podAntiAffinity.RequiredDuringSchedulingRequiredDuringExecution, false,
	//		fldPath.Child("requiredDuringSchedulingRequiredDuringExecution"))...)
	// }
	if podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		allErrs = append(allErrs, validatePodAffinityTerms(podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, allowInvalidLabelValueInSelector,
			fldPath.Child("requiredDuringSchedulingIgnoredDuringExecution"))...)
	}
	if podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		allErrs = append(allErrs, validateWeightedPodAffinityTerms(podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, allowInvalidLabelValueInSelector,
			fldPath.Child("preferredDuringSchedulingIgnoredDuringExecution"))...)
	}
	return allErrs
}

// validateNodeAffinity tests that the specified nodeAffinity fields have valid data
func validateNodeAffinity(na *corev1.NodeAffinity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: Uncomment the next three lines once RequiredDuringSchedulingRequiredDuringExecution is implemented.
	// if na.RequiredDuringSchedulingRequiredDuringExecution != nil {
	//	allErrs = append(allErrs, ValidateNodeSelector(na.RequiredDuringSchedulingRequiredDuringExecution, fldPath.Child("requiredDuringSchedulingRequiredDuringExecution"))...)
	// }
	if na.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		allErrs = append(allErrs, ValidateNodeSelector(na.RequiredDuringSchedulingIgnoredDuringExecution, fldPath.Child("requiredDuringSchedulingIgnoredDuringExecution"))...)
	}
	if len(na.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		allErrs = append(allErrs, ValidatePreferredSchedulingTerms(na.PreferredDuringSchedulingIgnoredDuringExecution, fldPath.Child("preferredDuringSchedulingIgnoredDuringExecution"))...)
	}
	return allErrs
}

// validatePodAffinity tests that the specified podAffinity fields have valid data
func validatePodAffinity(podAffinity *corev1.PodAffinity, allowInvalidLabelValueInSelector bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO:Uncomment below code once RequiredDuringSchedulingRequiredDuringExecution is implemented.
	// if podAffinity.RequiredDuringSchedulingRequiredDuringExecution != nil {
	//	allErrs = append(allErrs, validatePodAffinityTerms(podAffinity.RequiredDuringSchedulingRequiredDuringExecution, false,
	//		fldPath.Child("requiredDuringSchedulingRequiredDuringExecution"))...)
	// }
	if podAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		allErrs = append(allErrs, validatePodAffinityTerms(podAffinity.RequiredDuringSchedulingIgnoredDuringExecution, allowInvalidLabelValueInSelector,
			fldPath.Child("requiredDuringSchedulingIgnoredDuringExecution"))...)
	}
	if podAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		allErrs = append(allErrs, validateWeightedPodAffinityTerms(podAffinity.PreferredDuringSchedulingIgnoredDuringExecution, allowInvalidLabelValueInSelector,
			fldPath.Child("preferredDuringSchedulingIgnoredDuringExecution"))...)
	}
	return allErrs
}

// ValidateTolerations tests if given tolerations have valid data.
func ValidateTolerations(tolerations []corev1.Toleration, fldPath *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}
	for i, toleration := range tolerations {
		idxPath := fldPath.Index(i)
		// validate the toleration key
		if len(toleration.Key) > 0 {
			allErrors = append(allErrors, unversionedvalidation.ValidateLabelName(toleration.Key, idxPath.Child("key"))...)
		}

		// empty toleration key with Exists operator and empty value means match all taints
		if len(toleration.Key) == 0 && toleration.Operator != corev1.TolerationOpExists {
			allErrors = append(allErrors, field.Invalid(idxPath.Child("operator"), toleration.Operator,
				"operator must be Exists when `key` is empty, which means \"match all values and all keys\""))
		}

		if toleration.TolerationSeconds != nil && toleration.Effect != corev1.TaintEffectNoExecute {
			allErrors = append(allErrors, field.Invalid(idxPath.Child("effect"), toleration.Effect,
				"effect must be 'NoExecute' when `tolerationSeconds` is set"))
		}

		// validate toleration operator and value
		switch toleration.Operator {
		// empty operator means Equal
		case corev1.TolerationOpEqual, "":
			if errs := validation.IsValidLabelValue(toleration.Value); len(errs) != 0 {
				allErrors = append(allErrors, field.Invalid(idxPath.Child("operator"), toleration.Value, strings.Join(errs, ";")))
			}
		case corev1.TolerationOpExists:
			if len(toleration.Value) > 0 {
				allErrors = append(allErrors, field.Invalid(idxPath.Child("operator"), toleration, "value must be empty when `operator` is 'Exists'"))
			}
		default:
			validValues := []corev1.TolerationOperator{corev1.TolerationOpEqual, corev1.TolerationOpExists}
			allErrors = append(allErrors, field.NotSupported(idxPath.Child("operator"), toleration.Operator, validValues))
		}

		// validate toleration effect, empty toleration effect means match all taint effects
		if len(toleration.Effect) > 0 {
			allErrors = append(allErrors, validateTaintEffect(&toleration.Effect, true, idxPath.Child("effect"))...)
		}
	}
	return allErrors
}

func validateTaintEffect(effect *corev1.TaintEffect, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	if !allowEmpty && len(*effect) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	allErrors := field.ErrorList{}
	switch *effect {
	// TODO: Replace next line with subsequent commented-out line when implement TaintEffectNoScheduleNoAdmit.
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
		// case core.TaintEffectNoSchedule, core.TaintEffectPreferNoSchedule, core.TaintEffectNoScheduleNoAdmit, core.TaintEffectNoExecute:
	default:
		validValues := []corev1.TaintEffect{
			corev1.TaintEffectNoSchedule,
			corev1.TaintEffectPreferNoSchedule,
			corev1.TaintEffectNoExecute,
			// TODO: Uncomment this block when implement TaintEffectNoScheduleNoAdmit.
			// core.TaintEffectNoScheduleNoAdmit,
		}
		allErrors = append(allErrors, field.NotSupported(fldPath, *effect, validValues))
	}
	return allErrors
}
