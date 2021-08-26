package validation

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/scylladb/go-set/strset"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	AlternatorWriteIsolationAlways         = "always"
	AlternatorWriteIsolationForbidRMW      = "forbid_rmw"
	AlternatorWriteIsolationOnlyRMWUsesLWT = "only_rmw_uses_lwt"
)

var (
	AlternatorSupportedWriteIsolation = []string{
		AlternatorWriteIsolationAlways,
		AlternatorWriteIsolationForbidRMW,
		AlternatorWriteIsolationOnlyRMWUsesLWT,
	}
)

func ValidateScyllaCluster(c *scyllav1.ScyllaCluster) field.ErrorList {
	return ValidateScyllaClusterSpec(&c.Spec, field.NewPath("spec"))
}

func ValidateScyllaClusterSpec(spec *scyllav1.ScyllaClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	rackNames := sets.NewString()

	if spec.Alternator != nil {
		if spec.Alternator.WriteIsolation != "" {
			found := false
			for _, wi := range AlternatorSupportedWriteIsolation {
				if spec.Alternator.WriteIsolation == wi {
					found = true
				}
			}
			if !found {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("alternator", "writeIsolation"), spec.Alternator.WriteIsolation, AlternatorSupportedWriteIsolation))
			}
		}
	}

	if len(spec.ScyllaArgs) > 0 {
		version := semver.NewScyllaVersion(spec.Version)
		if !version.SupportFeatureUnsafe(semver.ScyllaVersionThatSupportsArgs) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("scyllaArgs"), fmt.Sprintf("ScyllaArgs is only supported starting from %s", semver.ScyllaVersionThatSupportsArgs)))
		}
	}

	for i, rack := range spec.Datacenter.Racks {
		allErrs = append(allErrs, ValidateScyllaClusterRackSpec(rack, rackNames, spec.CpuSet, fldPath.Child("datacenter", "racks").Index(i))...)
	}

	managerTaskNames := strset.New()
	for i, r := range spec.Repairs {
		if managerTaskNames.Has(r.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("repairs").Index(i).Child("name"), r.Name))
		}
		managerTaskNames.Add(r.Name)

		_, err := strconv.ParseFloat(r.Intensity, 64)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("repairs").Index(i).Child("intensity"), r.Intensity, "invalid intensity, it must be a float value"))
		}
	}

	for i, b := range spec.Backups {
		if managerTaskNames.Has(b.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("backups").Index(i).Child("name"), b.Name))
		}
		managerTaskNames.Add(b.Name)
	}

	if spec.GenericUpgrade != nil {
		if spec.GenericUpgrade.FailureStrategy != scyllav1.GenericUpgradeFailureStrategyRetry {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("genericUpgrade", "failureStrategy"), spec.GenericUpgrade.FailureStrategy, []string{string(scyllav1.GenericUpgradeFailureStrategyRetry)}))
		}
	}

	return allErrs
}

func ValidateScyllaClusterRackSpec(rack scyllav1.RackSpec, rackNames sets.String, cpuSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that no two racks have the same name
	if rackNames.Has(rack.Name) {
		allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), rack.Name))
	}
	rackNames.Insert(rack.Name)

	// Check that limits are defined
	limits := rack.Resources.Limits
	if limits == nil || limits.Cpu().Value() == 0 || limits.Memory().Value() == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("resources", "limits"), "set cpu, memory resource limits"))
	}

	// If the cluster has cpuset
	if cpuSet {
		cores := limits.Cpu().MilliValue()

		// CPU limits must be whole cores
		if cores%1000 != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("resources", "limits", "cpu"), cores, "when using cpuset, you must use whole cpu cores"))
		}

		// Requests == Limits and Requests must be set and equal for QOS class guaranteed
		requests := rack.Resources.Requests
		if requests != nil {
			if requests.Cpu().MilliValue() != limits.Cpu().MilliValue() {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("resources", "requests", "cpu"), requests.Cpu().MilliValue(), "when using cpuset, cpu requests must be the same as cpu limits"))
			}
			if requests.Memory().MilliValue() != limits.Memory().MilliValue() {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("resources", "requests", "memory"), requests.Cpu().MilliValue(), "when using cpuset, memory requests must be the same as memory limits"))
			}
		} else {
			// Copy the limits
			rack.Resources.Requests = limits.DeepCopy()
		}
	}

	return allErrs
}

func ValidateScyllaClusterUpdate(new, old *scyllav1.ScyllaCluster) field.ErrorList {
	allErrs := ValidateScyllaCluster(new)

	return append(allErrs, ValidateScyllaClusterSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)
}

func ValidateScyllaClusterSpecUpdate(newSpec, oldSpec *scyllav1.ScyllaClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that the datacenter name didn't change
	if oldSpec.Datacenter.Name != newSpec.Datacenter.Name {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "name"), "change of datacenter name is currently not supported"))
	}

	// Check that all rack names are the same as before
	oldRackNames, newRackNames := sets.NewString(), sets.NewString()
	for _, rack := range oldSpec.Datacenter.Racks {
		oldRackNames.Insert(rack.Name)
	}
	for _, rack := range newSpec.Datacenter.Racks {
		newRackNames.Insert(rack.Name)
	}
	diff := oldRackNames.Difference(newRackNames)
	if diff.Len() != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "racks"), fmt.Sprintf("racks %v not found, you cannot remove racks from the spec", diff.List())))
	}

	rackMap := make(map[string]scyllav1.RackSpec)
	for _, oldRack := range oldSpec.Datacenter.Racks {
		rackMap[oldRack.Name] = oldRack
	}
	for i, newRack := range newSpec.Datacenter.Racks {
		oldRack, exists := rackMap[newRack.Name]
		if !exists {
			continue
		}

		// Check that storage is the same as before.
		// StatefulSet currently forbids the storage update.
		if !reflect.DeepEqual(oldRack.Storage, newRack.Storage) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "racks").Index(i).Child("storage"), "changes in storage are currently not supported"))
		}
	}

	return allErrs
}
