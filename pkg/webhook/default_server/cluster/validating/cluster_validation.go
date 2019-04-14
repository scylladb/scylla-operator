package validating

import (
	"fmt"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"reflect"
)

func checkValues(c *scyllav1alpha1.Cluster) (allowed bool, msg string) {
	rackNames := sets.NewString()
	for _, rack := range c.Spec.Datacenter.Racks {
		// Check that no two racks have the same name
		if rackNames.Has(rack.Name) {
			return false, fmt.Sprintf("two racks have the same name: '%s'", rack.Name)
		}
		rackNames.Insert(rack.Name)

		// Check that limits are defined
		limits := rack.Resources.Limits
		if limits == nil || limits.Cpu().Value() == 0 || limits.Memory().Value() == 0 {
			return false, fmt.Sprintf("set cpu, memory resource limits for rack %s", rack.Name)
		}

		// If the cluster has cpuset
		if c.Spec.CpuSet {
			cores := limits.Cpu().MilliValue()

			// CPU limits must be whole cores
			if cores%1000 != 0 {
				return false, fmt.Sprintf("when using cpuset, you must use whole cpu cores, but rack %s has %dm", rack.Name, cores)
			}

			// Requests == Limits or only Limits must be set for QOS class guaranteed
			requests := rack.Resources.Requests
			if requests != nil {
				if requests.Cpu().MilliValue() == 0 {
					return false, fmt.Sprintf("when using cpuset, cpu requests must be the same as cpu limits in rack %s", rack.Name)
				}
				if requests.Memory().MilliValue() == 0 {
					return false, fmt.Sprintf("when using cpuset, memory requests must be the same as memory limits in rack %s", rack.Name)
				}
				return false, fmt.Sprintf("when using cpuset, requests must be the same as limits in rack %s", rack.Name)
			}
		}
	}

	return true, ""
}

func checkTransitions(old, new *scyllav1alpha1.Cluster) (allowed bool, msg string) {
	// Check that version remained the same
	if old.Spec.Version != new.Spec.Version {
		return false, "change of version is currently not supported"
	}

	// Check that repository remained the same
	if old.Spec.Repository != new.Spec.Repository {
		return false, "repository change is currently not supported"
	}

	// Check that sidecarImage remained the same
	if !reflect.DeepEqual(old.Spec.SidecarImage, new.Spec.SidecarImage) {
		return false, "change of sidecarImage is currently not supported"
	}

	// Check that the datacenter name didn't change
	if old.Spec.Datacenter.Name != new.Spec.Datacenter.Name {
		return false, "change of datacenter name is currently not supported"
	}

	// Check that all rack names are the same as before
	oldRackNames, newRackNames := sets.NewString(), sets.NewString()
	for _, rack := range old.Spec.Datacenter.Racks {
		oldRackNames.Insert(rack.Name)
	}
	for _, rack := range new.Spec.Datacenter.Racks {
		newRackNames.Insert(rack.Name)
	}
	diff := oldRackNames.Difference(newRackNames)
	if diff.Len() != 0 {
		return false, fmt.Sprintf("racks %v not found, you cannot remove racks from the spec", diff.List())
	}

	rackMap := make(map[string]scyllav1alpha1.RackSpec)
	for _, oldRack := range old.Spec.Datacenter.Racks {
		rackMap[oldRack.Name] = oldRack
	}
	for _, newRack := range new.Spec.Datacenter.Racks {
		oldRack, exists := rackMap[newRack.Name]
		if !exists {
			continue
		}

		// Check that placement is the same as before
		if !reflect.DeepEqual(oldRack.Placement, newRack.Placement) {
			return false, fmt.Sprintf("rack %s: changes in placement are not currently supported", oldRack.Name)
		}

		// Check that storage is the same as before
		if !reflect.DeepEqual(oldRack.Storage, newRack.Storage) {
			return false, fmt.Sprintf("rack %s: changes in storage are not currently supported", oldRack.Name)
		}

		// Check that resources are the same as before
		if !reflect.DeepEqual(oldRack.Resources, newRack.Resources) {
			return false, fmt.Sprintf("rack %s: changes in resources are not currently supported", oldRack.Name)
		}
	}

	return true, ""
}
