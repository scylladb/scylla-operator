// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	"fmt"
	"reflect"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
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

func ValidateScyllaCluster(c *scyllav2alpha1.ScyllaCluster) field.ErrorList {
	return ValidateScyllaClusterSpec(&c.Spec, field.NewPath("spec"))
}

func ValidateScyllaClusterSpec(spec *scyllav2alpha1.ScyllaClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	datacenterNames := sets.NewString()

	if spec.Scylla.AlternatorOptions != nil {
		if spec.Scylla.AlternatorOptions.WriteIsolation != "" {
			found := false
			for _, wi := range AlternatorSupportedWriteIsolation {
				if spec.Scylla.AlternatorOptions.WriteIsolation == wi {
					found = true
				}
			}
			if !found {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("scylla", "alternatorOptions", "writeIsolation"), spec.Scylla.AlternatorOptions.WriteIsolation, AlternatorSupportedWriteIsolation))
			}
		}
	}

	if len(spec.Datacenters) > 1 {
		allErrs = append(allErrs, field.TooMany(fldPath.Child("datacenters"), len(spec.Datacenters), 1))
	}

	for i, dc := range spec.Datacenters {
		if datacenterNames.Has(dc.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("datacenters").Index(i).Child("name"), dc.Name))
		}
		datacenterNames.Insert(dc.Name)

		rackNames := sets.NewString()
		for j, rack := range dc.Racks {
			allErrs = append(allErrs, ValidateScyllaClusterRackSpec(rack, rackNames, fldPath.Child("datacenters").Index(i).Child("racks").Index(j))...)
		}

		if dc.Scylla == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("datacenters").Index(i).Child("scylla"), "scylla datacenter properties are required"))
		}

		if dc.Scylla != nil && dc.Scylla.Resources == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("datacenters").Index(i).Child("scylla", "resources"), "scylla resources are required"))
		}

		if spec.ScyllaManagerAgent != nil && dc.ScyllaManagerAgent == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("datacenters").Index(i).Child("scyllaManagerAgent"), "scylla manager agent datacenter properties are required when agent is enabled"))
		}

		if spec.ScyllaManagerAgent != nil && dc.ScyllaManagerAgent != nil && dc.ScyllaManagerAgent.Resources == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("datacenters").Index(i).Child("scyllaManagerAgent", "resources"), "scylla manager agent resources are required"))
		}

		if dc.NodesPerRack == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("datacenters").Index(i).Child("nodesPerRack"), "number of nodes per rack is required"))
		}
	}

	return allErrs
}

func ValidateScyllaClusterRackSpec(rack scyllav2alpha1.RackSpec, rackNames sets.String, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that no two racks have the same name
	if rackNames.Has(rack.Name) {
		allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), rack.Name))
	}
	rackNames.Insert(rack.Name)

	return allErrs
}

func ValidateScyllaClusterUpdate(new, old *scyllav2alpha1.ScyllaCluster) field.ErrorList {
	allErrs := ValidateScyllaCluster(new)

	return append(allErrs, ValidateScyllaClusterSpecUpdate(new, old, field.NewPath("spec"))...)
}

func ValidateScyllaClusterSpecUpdate(new, old *scyllav2alpha1.ScyllaCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	oldDatacenterNames, newDatacenterNames := sets.NewString(), sets.NewString()
	for _, dc := range old.Spec.Datacenters {
		oldDatacenterNames.Insert(dc.Name)
	}
	for _, dc := range new.Spec.Datacenters {
		newDatacenterNames.Insert(dc.Name)
	}
	diff := oldDatacenterNames.Difference(newDatacenterNames)
	for _, dcName := range diff.List() {
		for i, dc := range old.Spec.Datacenters {
			if dc.Name != dcName {
				continue
			}

			if dc.NodesPerRack != nil && *dc.NodesPerRack != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenters").Index(i), fmt.Sprintf("datacenter %q can't be removed because it still has nodes that have to be scaled down to zero first", dcName)))
				continue
			}

			for _, dcStatus := range old.Status.Datacenters {
				if dcStatus.Name != dcName {
					continue
				}

				if dcStatus.Nodes != nil && *dcStatus.Nodes != 0 {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenters").Index(i), fmt.Sprintf("datacenter %q can't be removed because the nodes are being scaled down", dcName)))
					continue
				}

				if !isDatacenterStatusUpToDate(old, dcStatus) {
					allErrs = append(allErrs, field.InternalError(fldPath.Child("datacenters").Index(i), fmt.Errorf("datacenter %q can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later", dcName)))
				}
			}
		}
	}

	dcMap := make(map[string]scyllav2alpha1.Datacenter)
	for _, oldDc := range old.Spec.Datacenters {
		dcMap[oldDc.Name] = oldDc
	}
	for i, newDc := range new.Spec.Datacenters {
		oldDc, exists := dcMap[newDc.Name]
		if !exists {
			continue
		}

		// Check that storage is the same as before.
		// StatefulSet currently forbids the storage update.
		if !reflect.DeepEqual(oldDc.Scylla.Storage, newDc.Scylla.Storage) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenters").Index(i).Child("scylla", "storage"), "changes in storage are currently not supported"))
		}
	}

	return allErrs
}

func isDatacenterStatusUpToDate(sc *scyllav2alpha1.ScyllaCluster, dcStatus scyllav2alpha1.DatacenterStatus) bool {
	return sc.Status.ObservedGeneration != nil &&
		*sc.Status.ObservedGeneration >= sc.Generation &&
		dcStatus.Stale != nil &&
		!*dcStatus.Stale
}
