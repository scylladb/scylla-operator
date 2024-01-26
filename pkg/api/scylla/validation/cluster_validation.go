package validation

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/scylladb/go-set/strset"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/semver"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
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

	SupportedBroadcastAddressTypes = []scyllav1.BroadcastAddressType{
		scyllav1.BroadcastAddressTypePodIP,
		scyllav1.BroadcastAddressTypeServiceClusterIP,
		scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
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

	for i, domain := range spec.DNSDomains {
		allErrs = append(allErrs, apimachineryutilvalidation.IsFullyQualifiedName(fldPath.Child("dnsDomains").Index(i), domain)...)
	}

	if len(spec.DNSDomains) == 0 && spec.ExposeOptions != nil {
		if spec.ExposeOptions.CQL != nil && spec.ExposeOptions.CQL.Ingress != nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("dnsDomains"), "at least one domain needs to be provided when exposing CQL via ingresses"))
		}
	}

	if spec.ExposeOptions != nil {
		allErrs = append(allErrs, ValidateExposeOptions(spec.ExposeOptions, fldPath.Child("exposeOptions"))...)
	}

	if spec.MinTerminationGracePeriodSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minTerminationGracePeriodSeconds"), spec.MinTerminationGracePeriodSeconds, "must be non-negative integer"))
	}

	return allErrs
}

func ValidateExposeOptions(options *scyllav1.ExposeOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if options.CQL != nil && options.CQL.Ingress != nil {
		allErrs = append(allErrs, ValidateIngressOptions(options.CQL.Ingress, fldPath.Child("cql", "ingress"))...)
	}

	if options.NodeService != nil {
		allErrs = append(allErrs, ValidateNodeService(options.NodeService, fldPath.Child("nodeService"))...)
	}

	if options.BroadcastOptions != nil {
		allErrs = append(allErrs, ValidateNodeBroadcastOptions(options.BroadcastOptions, options.NodeService, fldPath.Child("broadcastOptions"))...)
	}

	return allErrs
}

func ValidateNodeBroadcastOptions(options *scyllav1.NodeBroadcastOptions, nodeService *scyllav1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateBroadcastOptions(options.Clients, nodeService, fldPath.Child("clients"))...)
	allErrs = append(allErrs, ValidateBroadcastOptions(options.Nodes, nodeService, fldPath.Child("nodes"))...)

	return allErrs
}

func ValidateBroadcastOptions(options scyllav1.BroadcastOptions, nodeService *scyllav1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !slices.ContainsItem(SupportedBroadcastAddressTypes, options.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), options.Type, slices.ConvertSlice(SupportedBroadcastAddressTypes, slices.ToString[scyllav1.BroadcastAddressType])))
	}

	var allowedNodeServiceTypesByBroadcastAddressType = map[scyllav1.BroadcastAddressType][]scyllav1.NodeServiceType{
		scyllav1.BroadcastAddressTypePodIP: {
			scyllav1.NodeServiceTypeHeadless,
			scyllav1.NodeServiceTypeClusterIP,
			scyllav1.NodeServiceTypeLoadBalancer,
		},
		scyllav1.BroadcastAddressTypeServiceClusterIP: {
			scyllav1.NodeServiceTypeClusterIP,
			scyllav1.NodeServiceTypeLoadBalancer,
		},
		scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress: {
			scyllav1.NodeServiceTypeLoadBalancer,
		},
	}

	nodeServiceType := scyllav1.NodeServiceTypeClusterIP
	if nodeService != nil {
		nodeServiceType = nodeService.Type
	}

	// Skipping an error when chosen option type is unsupported as it won't help anyhow users reading it.
	allowedNodeServiceTypes, ok := allowedNodeServiceTypesByBroadcastAddressType[options.Type]
	if ok && !slices.ContainsItem(allowedNodeServiceTypes, nodeServiceType) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), options.Type, fmt.Sprintf("can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: %v", allowedNodeServiceTypes)))
	}

	return allErrs
}

func ValidateNodeService(nodeService *scyllav1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var supportedServiceTypes = []scyllav1.NodeServiceType{
		scyllav1.NodeServiceTypeHeadless,
		scyllav1.NodeServiceTypeClusterIP,
		scyllav1.NodeServiceTypeLoadBalancer,
	}

	if !slices.ContainsItem(supportedServiceTypes, nodeService.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), nodeService.Type, slices.ConvertSlice(supportedServiceTypes, slices.ToString[scyllav1.NodeServiceType])))
	}

	if nodeService.LoadBalancerClass != nil && len(*nodeService.LoadBalancerClass) != 0 {
		for _, msg := range apimachineryutilvalidation.IsQualifiedName(*nodeService.LoadBalancerClass) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("loadBalancerClass"), *nodeService.LoadBalancerClass, msg))
		}
	}

	if len(nodeService.Annotations) != 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(nodeService.Annotations, fldPath.Child("annotations"))...)
	}

	return allErrs
}

func ValidateIngressOptions(options *scyllav1.IngressOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(options.IngressClassName) != 0 {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(options.IngressClassName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingressClassName"), options.IngressClassName, msg))
		}
	}

	if len(options.Annotations) != 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(options.Annotations, fldPath.Child("annotations"))...)
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

	return append(allErrs, ValidateScyllaClusterSpecUpdate(new, old, field.NewPath("spec"))...)
}

func ValidateScyllaClusterSpecUpdate(new, old *scyllav1.ScyllaCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check that the datacenter name didn't change
	if old.Spec.Datacenter.Name != new.Spec.Datacenter.Name {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "name"), "change of datacenter name is currently not supported"))
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
	for _, rackName := range diff.List() {
		for i, rack := range old.Spec.Datacenter.Racks {
			if rack.Name != rackName {
				continue
			}

			if rack.Members != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "racks").Index(i), fmt.Sprintf("rack %q can't be removed because it still has members that have to be scaled down to zero first", rackName)))
				continue
			}

			if old.Status.Racks[rack.Name].Members != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "racks").Index(i), fmt.Sprintf("rack %q can't be removed because the members are being scaled down", rackName)))
				continue
			}

			if !isRackStatusUpToDate(old, rack.Name) {
				allErrs = append(allErrs, field.InternalError(fldPath.Child("datacenter", "racks").Index(i), fmt.Errorf("rack %q can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later", rackName)))
			}
		}
	}

	rackMap := make(map[string]scyllav1.RackSpec)
	for _, oldRack := range old.Spec.Datacenter.Racks {
		rackMap[oldRack.Name] = oldRack
	}
	for i, newRack := range new.Spec.Datacenter.Racks {
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

	var oldClientBroadcastAddressType, newClientBroadcastAddressType *scyllav1.BroadcastAddressType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.BroadcastOptions != nil {
		oldClientBroadcastAddressType = pointer.Ptr(old.Spec.ExposeOptions.BroadcastOptions.Clients.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.BroadcastOptions != nil {
		newClientBroadcastAddressType = pointer.Ptr(new.Spec.ExposeOptions.BroadcastOptions.Clients.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newClientBroadcastAddressType, oldClientBroadcastAddressType, fldPath.Child("exposeOptions", "broadcastOptions", "clients", "type"))...)

	var oldNodesBroadcastAddressType, newNodesBroadcastAddressType *scyllav1.BroadcastAddressType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.BroadcastOptions != nil {
		oldNodesBroadcastAddressType = pointer.Ptr(old.Spec.ExposeOptions.BroadcastOptions.Nodes.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.BroadcastOptions != nil {
		newNodesBroadcastAddressType = pointer.Ptr(new.Spec.ExposeOptions.BroadcastOptions.Nodes.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newNodesBroadcastAddressType, oldNodesBroadcastAddressType, fldPath.Child("exposeOptions", "broadcastOptions", "nodes", "type"))...)

	var oldNodeServiceType, newNodeServiceType *scyllav1.NodeServiceType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.NodeService != nil {
		oldNodeServiceType = pointer.Ptr(old.Spec.ExposeOptions.NodeService.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.NodeService != nil {
		newNodeServiceType = pointer.Ptr(new.Spec.ExposeOptions.NodeService.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newNodeServiceType, oldNodeServiceType, fldPath.Child("exposeOptions", "nodeService", "type"))...)

	return allErrs
}

func isRackStatusUpToDate(sc *scyllav1.ScyllaCluster, rack string) bool {
	return sc.Status.ObservedGeneration != nil &&
		*sc.Status.ObservedGeneration >= sc.Generation &&
		sc.Status.Racks[rack].Stale != nil &&
		!*sc.Status.Racks[rack].Stale
}
