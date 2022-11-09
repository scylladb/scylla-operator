// Copyright (c) 2022 ScyllaDB.

package v1alpha1

import (
	"fmt"
	"reflect"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
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

func ValidateScyllaDatacenter(c *scyllav1alpha1.ScyllaDatacenter) field.ErrorList {
	return ValidateScyllaDatacenterSpec(&c.Spec, field.NewPath("spec"))
}

func ValidateScyllaDatacenterSpec(spec *scyllav1alpha1.ScyllaDatacenterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	rackNames := sets.NewString()

	if spec.Scylla.AlternatorOptions != nil {
		if spec.Scylla.AlternatorOptions.WriteIsolation != "" {
			found := false
			for _, wi := range AlternatorSupportedWriteIsolation {
				if spec.Scylla.AlternatorOptions.WriteIsolation == wi {
					found = true
				}
			}
			if !found {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("alternator", "writeIsolation"), spec.Scylla.AlternatorOptions.WriteIsolation, AlternatorSupportedWriteIsolation))
			}
		}
	}

	for i, rack := range spec.Racks {
		allErrs = append(allErrs, ValidateScyllaDatacenterRackSpec(rack, rackNames, nil, fldPath.Child("racks").Index(i))...)
	}

	return allErrs
}

func ValidateScyllaDatacenterRackSpec(rack scyllav1alpha1.RackSpec, rackNames sets.String, cpuSet *bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that no two racks have the same name
	if rackNames.Has(rack.Name) {
		allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), rack.Name))
	}
	rackNames.Insert(rack.Name)

	return allErrs
}

func ValidateScyllaDatacenterUpdate(new, old *scyllav1alpha1.ScyllaDatacenter) field.ErrorList {
	allErrs := ValidateScyllaDatacenter(new)

	return append(allErrs, ValidateScyllaDatacenterSpecUpdate(new, old, field.NewPath("spec"))...)
}

func ValidateScyllaDatacenterSpecUpdate(new, old *scyllav1alpha1.ScyllaDatacenter, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that the datacenter name didn't change
	if old.Spec.DatacenterName != new.Spec.DatacenterName {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenterName"), "change of datacenter name is currently not supported"))
	}

	// Check that all rack names are the same as before
	oldRackNames, newRackNames := sets.NewString(), sets.NewString()
	for _, rack := range old.Spec.Racks {
		oldRackNames.Insert(rack.Name)
	}
	for _, rack := range new.Spec.Racks {
		newRackNames.Insert(rack.Name)
	}
	diff := oldRackNames.Difference(newRackNames)
	for _, rackName := range diff.List() {
		for i, rack := range old.Spec.Racks {
			if rack.Name != rackName {
				continue
			}

			if rack.Nodes != nil && *rack.Nodes != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i), fmt.Sprintf("rack %q can't be removed because it still has members that have to be scaled down to zero first", rackName)))
				continue
			}

			if old.Status.Racks[rack.Name].Nodes != nil && *old.Status.Racks[rack.Name].Nodes != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i), fmt.Sprintf("rack %q can't be removed because the members are being scaled down", rackName)))
				continue
			}

			if !isScyllaDatacenterRackStatusUpToDate(old, rack.Name) {
				allErrs = append(allErrs, field.InternalError(fldPath.Child("racks").Index(i), fmt.Errorf("rack %q can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later", rackName)))
			}
		}
	}

	// Check that storage is the same as before.
	// StatefulSet currently forbids the storage update.
	for i, newRack := range new.Spec.Racks {
		for _, oldRack := range old.Spec.Racks {
			if oldRack.Name != newRack.Name {
				continue
			}
			if !reflect.DeepEqual(oldRack.Scylla.Storage, newRack.Scylla.Storage) {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i).Child("scylla", "storage"), "changes in storage are currently not supported"))
			}
		}
	}

	return allErrs
}

func isScyllaDatacenterRackStatusUpToDate(sc *scyllav1alpha1.ScyllaDatacenter, rack string) bool {
	return sc.Status.ObservedGeneration != nil &&
		*sc.Status.ObservedGeneration >= sc.Generation &&
		sc.Status.Racks[rack].Stale != nil &&
		!*sc.Status.Racks[rack].Stale
}
