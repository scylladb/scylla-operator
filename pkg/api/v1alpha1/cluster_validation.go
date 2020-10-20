package v1alpha1

import (
	"reflect"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func checkValues(c *Cluster) error {
	rackNames := sets.NewString()

	if len(c.Spec.ScyllaArgs) > 0 {
		version, err := semver.Parse(c.Spec.Version)
		if err == nil && version.LT(ScyllaVersionThatSupportsArgs) {
			return errors.Errorf("ScyllaArgs is only supported starting from %s", ScyllaVersionThatSupportsArgsText)
		}
	}

	for _, rack := range c.Spec.Datacenter.Racks {
		// Check that no two racks have the same name
		if rackNames.Has(rack.Name) {
			return errors.Errorf("two racks have the same name: '%s'", rack.Name)
		}
		rackNames.Insert(rack.Name)

		// Check that limits are defined
		limits := rack.Resources.Limits
		if limits == nil || limits.Cpu().Value() == 0 || limits.Memory().Value() == 0 {
			return errors.Errorf("set cpu, memory resource limits for rack %s", rack.Name)
		}

		// If the cluster has cpuset
		if c.Spec.CpuSet {
			cores := limits.Cpu().MilliValue()

			// CPU limits must be whole cores
			if cores%1000 != 0 {
				return errors.Errorf("when using cpuset, you must use whole cpu cores, but rack %s has %dm", rack.Name, cores)
			}

			// Requests == Limits and Requests must be set and equal for QOS class guaranteed
			requests := rack.Resources.Requests
			if requests != nil {
				if requests.Cpu().MilliValue() != limits.Cpu().MilliValue() {
					return errors.Errorf("when using cpuset, cpu requests must be the same as cpu limits in rack %s", rack.Name)
				}
				if requests.Memory().MilliValue() != limits.Memory().MilliValue() {
					return errors.Errorf("when using cpuset, memory requests must be the same as memory limits in rack %s", rack.Name)
				}
			} else {
				// Copy the limits
				rack.Resources.Requests = limits.DeepCopy()
			}
		}
	}

	return nil
}

func checkTransitions(old, new *Cluster) error {
	oldVersion, err := semver.Parse(old.Spec.Version)
	if err != nil {
		return errors.Errorf("invalid old semantic version, err=%s", err)
	}
	newVersion, err := semver.Parse(new.Spec.Version)
	if err != nil {
		return errors.Errorf("invalid new semantic version, err=%s", err)
	}
	// Check that version remained the same
	if newVersion.Major != oldVersion.Major || newVersion.Minor != oldVersion.Minor {
		return errors.Errorf("only upgrading of patch versions are supported")
	}

	// Check that repository remained the same
	if !reflect.DeepEqual(old.Spec.Repository, new.Spec.Repository) {
		return errors.Errorf("repository change is currently not supported, old=%v, new=%v", *old.Spec.Repository, *new.Spec.Repository)
	}

	// Check that sidecarImage remained the same
	if !reflect.DeepEqual(old.Spec.SidecarImage, new.Spec.SidecarImage) {
		return errors.Errorf("change of sidecarImage is currently not supported")
	}

	// Check that the datacenter name didn't change
	if old.Spec.Datacenter.Name != new.Spec.Datacenter.Name {
		return errors.Errorf("change of datacenter name is currently not supported")
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
		return errors.Errorf("racks %v not found, you cannot remove racks from the spec", diff.List())
	}

	rackMap := make(map[string]RackSpec)
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
			return errors.Errorf("rack %s: changes in placement are not currently supported", oldRack.Name)
		}

		// Check that storage is the same as before
		if !reflect.DeepEqual(oldRack.Storage, newRack.Storage) {
			return errors.Errorf("rack %s: changes in storage are not currently supported", oldRack.Name)
		}

		// Check that resources are the same as before
		if !reflect.DeepEqual(oldRack.Resources, newRack.Resources) {
			return errors.Errorf("rack %s: changes in resources are not currently supported", oldRack.Name)
		}
	}

	return nil
}
