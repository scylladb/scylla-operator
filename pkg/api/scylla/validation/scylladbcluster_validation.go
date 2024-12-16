// Copyright (c) 2024 ScyllaDB.

package validation

import (
	"fmt"
	"sort"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	apimachinerymetav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateScyllaDBCluster(sc *scyllav1alpha1.ScyllaDBCluster) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBClusterSpec(&sc.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBClusterSpec(spec *scyllav1alpha1.ScyllaDBClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Metadata != nil {
		allErrs = append(allErrs, apimachinerymetav1validation.ValidateLabels(spec.Metadata.Labels, fldPath.Child("metadata", "labels"))...)
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(spec.Metadata.Annotations, fldPath.Child("metadata", "annotations"))...)
	}

	allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDB(&spec.ScyllaDB, fldPath.Child("scyllaDB"))...)
	allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBManagerAgent(spec.ScyllaDBManagerAgent, fldPath.Child("scyllaDBManagerAgent"))...)

	allErrs = append(allErrs, validateStructSliceFieldUniqueness(spec.Datacenters, func(dcSpec scyllav1alpha1.ScyllaDBClusterDatacenter) string {
		return dcSpec.Name
	}, "name", fldPath.Child("datacenters"))...)

	if spec.DatacenterTemplate != nil {
		allErrs = append(allErrs, ValidateScyllaDBClusterDatacenterTemplate(spec.DatacenterTemplate, fldPath.Child("datacenterTemplate"))...)
	}

	for i, dcSpec := range spec.Datacenters {
		allErrs = append(allErrs, ValidateScyllaDBClusterDatacenter(dcSpec, fldPath.Child("datacenters").Index(i))...)
	}

	if spec.ExposeOptions != nil {
		allErrs = append(allErrs, ValidateScyllaDBClusterSpecExposeOptions(spec.ExposeOptions, fldPath.Child("exposeOptions"))...)
	}

	if spec.MinTerminationGracePeriodSeconds != nil && *spec.MinTerminationGracePeriodSeconds < 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*spec.MinTerminationGracePeriodSeconds), fldPath.Child("minTerminationGracePeriodSeconds"))...)
	}

	if spec.MinReadySeconds != nil && *spec.MinReadySeconds < 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*spec.MinReadySeconds), fldPath.Child("minReadySeconds"))...)
	}

	for i, readinessGate := range spec.ReadinessGates {
		for _, message := range apimachineryutilvalidation.IsQualifiedName(string(readinessGate.ConditionType)) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("readinessGates").Index(i).Child("conditionType"), readinessGate.ConditionType, message))
		}
	}

	return allErrs
}

func ValidateScyllaDBClusterDatacenter(dc scyllav1alpha1.ScyllaDBClusterDatacenter, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(dc.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "datacenter name must not be empty"))
	}

	allErrs = append(allErrs, ValidateScyllaDBClusterDatacenterTemplate(&dc.ScyllaDBClusterDatacenterTemplate, fldPath)...)

	for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(dc.RemoteKubernetesClusterName, false) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("remoteKubernetesClusterName"), dc.RemoteKubernetesClusterName, msg))
	}

	return allErrs
}

func ValidateScyllaDBClusterDatacenterTemplate(dcTemplate *scyllav1alpha1.ScyllaDBClusterDatacenterTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if dcTemplate.Metadata != nil {
		allErrs = append(allErrs, apimachinerymetav1validation.ValidateLabels(dcTemplate.Metadata.Labels, fldPath.Child("metadata", "labels"))...)
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(dcTemplate.Metadata.Annotations, fldPath.Child("metadata", "annotations"))...)
	}

	if dcTemplate.RackTemplate != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterRackTemplate(dcTemplate.RackTemplate, fldPath.Child("rackTemplate"))...)

		if dcTemplate.RackTemplate.Placement != nil {
			allErrs = append(allErrs, ValidateScyllaDBDatacenterPlacement(dcTemplate.RackTemplate.Placement, fldPath.Child("rackTemplate", "placement"))...)
		}
	}

	allErrs = append(allErrs, validateStructSliceFieldUniqueness(dcTemplate.Racks, func(rack scyllav1alpha1.RackSpec) string {
		return rack.Name
	}, "name", fldPath.Child("racks"))...)

	for rackIdx, rack := range dcTemplate.Racks {
		if len(rack.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("racks").Index(rackIdx).Child("name"), "rack name must not be empty"))
		}
		allErrs = append(allErrs, ValidateScyllaDBDatacenterRackTemplate(&rack.RackTemplate, fldPath.Child("racks").Index(rackIdx))...)

		if rack.Placement != nil {
			allErrs = append(allErrs, ValidateScyllaDBDatacenterPlacement(rack.Placement, fldPath.Child("racks").Index(rackIdx).Child("placement"))...)
		}
	}

	if dcTemplate.TopologyLabelSelector != nil {
		allErrs = append(allErrs, apimachinerymetav1validation.ValidateLabels(dcTemplate.TopologyLabelSelector, fldPath.Child("topologyLabelSelector"))...)
	}

	if dcTemplate.ScyllaDB != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBTemplate(dcTemplate.ScyllaDB, fldPath.Child("scyllaDB"))...)
	}

	if dcTemplate.ScyllaDBManagerAgent != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBManagerAgentTemplate(dcTemplate.ScyllaDBManagerAgent, fldPath.Child("scyllaDBManagerAgent"))...)
	}

	if dcTemplate.Placement != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterPlacement(dcTemplate.Placement, fldPath.Child("placement"))...)
	}

	return allErrs
}

func ValidateScyllaDBClusterSpecExposeOptions(options *scyllav1alpha1.ScyllaDBClusterExposeOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if options.NodeService != nil {
		allErrs = append(allErrs, ValidateScyllaDBClusterNodeService(options.NodeService, fldPath.Child("nodeService"))...)
	}

	if options.BroadcastOptions != nil {
		allErrs = append(allErrs, ValidateScyllaDBClusterNodeBroadcastOptions(options.BroadcastOptions, options.NodeService, fldPath.Child("broadcastOptions"))...)
	}

	return allErrs
}

func ValidateScyllaDBClusterNodeService(nodeService *scyllav1alpha1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(nodeService.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), fmt.Sprintf("supported values: %s", strings.Join(slices.ConvertSlice(supportedNodeServiceTypes, slices.ToString[scyllav1alpha1.NodeServiceType]), ", "))))
	} else {
		allErrs = append(allErrs, validateEnum(nodeService.Type, supportedNodeServiceTypes, fldPath.Child("type"))...)
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

func ValidateScyllaDBClusterNodeBroadcastOptions(options *scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions, nodeService *scyllav1alpha1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var nodeServiceType *scyllav1alpha1.NodeServiceType
	if nodeService != nil {
		nodeServiceType = pointer.Ptr(nodeService.Type)
	}

	allErrs = append(allErrs,
		ValidateScyllaDBDatacenterBroadcastOptions(
			options.Clients.Type,
			SupportedScyllaV1Alpha1BroadcastAddressTypes,
			scyllav1alpha1.NodeServiceTypeHeadless,
			nodeServiceType,
			allowedNodeServiceTypesByBroadcastAddressType,
			fldPath.Child("clients"),
		)...,
	)

	allErrs = append(allErrs,
		ValidateScyllaDBDatacenterBroadcastOptions(
			options.Nodes.Type,
			SupportedScyllaV1Alpha1BroadcastAddressTypes,
			scyllav1alpha1.NodeServiceTypeHeadless,
			nodeServiceType,
			allowedNodeServiceTypesByBroadcastAddressType,
			fldPath.Child("nodes"),
		)...,
	)

	return allErrs
}

func ValidateScyllaDBClusterUpdate(new, old *scyllav1alpha1.ScyllaDBCluster) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBCluster(new)...)
	allErrs = append(allErrs, ValidateScyllaDBClusterSpecUpdate(new, old, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBClusterSpecUpdate(new, old *scyllav1alpha1.ScyllaDBCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(new.Spec.ClusterName, old.Spec.ClusterName, fldPath.Child("clusterName"))...)

	oldDatacenterNames := slices.ConvertSlice(old.Spec.Datacenters, func(dc scyllav1alpha1.ScyllaDBClusterDatacenter) string {
		return dc.Name
	})
	newDatacenterNames := slices.ConvertSlice(new.Spec.Datacenters, func(dc scyllav1alpha1.ScyllaDBClusterDatacenter) string {
		return dc.Name
	})

	removedDatacenterNames := sets.New(oldDatacenterNames...).Difference(sets.New(newDatacenterNames...)).UnsortedList()
	sort.Strings(removedDatacenterNames)

	isDatacenterStatusUpToDate := func(sc *scyllav1alpha1.ScyllaDBCluster, dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
		return sc.Status.ObservedGeneration != nil && *sc.Status.ObservedGeneration >= sc.Generation && dcStatus.Stale != nil && !*dcStatus.Stale
	}

	for _, removedDCName := range removedDatacenterNames {
		for i, oldDC := range old.Spec.Datacenters {
			if oldDC.Name != removedDCName {
				continue
			}

			oldDCStatus, _, ok := slices.Find(old.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dcStatus.Name == removedDCName
			})
			if !ok {
				allErrs = append(allErrs, field.InternalError(fldPath.Child("datacenters").Index(i), fmt.Errorf("datacenter %q can't be removed because its status is missing; please retry later", removedDCName)))
				continue
			}

			if oldDCStatus.Nodes != nil && *oldDCStatus.Nodes != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenters").Index(i), fmt.Sprintf("datacenter %q can't be removed because its nodes are being scaled down", removedDCName)))
				continue
			}

			if !isDatacenterStatusUpToDate(old, oldDCStatus) {
				allErrs = append(allErrs, field.InternalError(fldPath.Child("datacenters").Index(i), fmt.Errorf("datacenter %q can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later", removedDCName)))
			}
		}
	}

	type dcRackProperties struct {
		datacenter string
		rack       string
		level      string
		storage    *scyllav1alpha1.StorageOptions
		fieldPath  *field.Path
	}

	collectRacks := func(sc *scyllav1alpha1.ScyllaDBCluster) []dcRackProperties {
		var racks []dcRackProperties
		for dcIdx, dc := range sc.Spec.Datacenters {
			if sc.Spec.DatacenterTemplate != nil {
				for rackIdx, rack := range sc.Spec.DatacenterTemplate.Racks {
					dcr := dcRackProperties{
						datacenter: dc.Name,
						rack:       rack.Name,
						level:      "dcTemplate",
						fieldPath:  fldPath.Child("datacenterTemplate", "racks").Index(rackIdx),
					}
					if rack.ScyllaDB != nil {
						dcr.storage = rack.ScyllaDB.Storage
					}

					racks = append(racks, dcr)
				}
			}
			for rackIdx, rack := range dc.Racks {
				dcr := dcRackProperties{
					datacenter: dc.Name,
					rack:       rack.Name,
					level:      "racks",
					fieldPath:  fldPath.Child("datacenters").Index(dcIdx).Child("racks").Index(rackIdx),
				}
				if rack.ScyllaDB != nil {
					dcr.storage = rack.ScyllaDB.Storage
				}

				racks = append(racks, dcr)
			}
		}
		return racks
	}

	newRacks := collectRacks(new)
	oldRacks := collectRacks(old)

	removedRacks := slices.Filter(oldRacks, func(odr dcRackProperties) bool {
		_, _, ok := slices.Find(newRacks, func(ndr dcRackProperties) bool {
			return ndr.rack == odr.rack && ndr.datacenter == odr.datacenter
		})
		return !ok
	})

	isRackStatusUpToDate := func(sc *scyllav1alpha1.ScyllaDBCluster, rackStatus scyllav1alpha1.ScyllaDBClusterRackStatus) bool {
		return sc.Status.ObservedGeneration != nil && *sc.Status.ObservedGeneration >= sc.Generation && rackStatus.Stale != nil && !*rackStatus.Stale
	}

	for _, removedRack := range removedRacks {
		oldDCStatus, _, ok := slices.Find(old.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == removedRack.datacenter
		})
		if !ok {
			allErrs = append(allErrs, field.InternalError(removedRack.fieldPath, fmt.Errorf("rack %q can't be removed because its status is missing; please retry later", removedRack.rack)))
			continue
		}

		oldRackStatus, _, ok := slices.Find(oldDCStatus.Racks, func(rackStatus scyllav1alpha1.ScyllaDBClusterRackStatus) bool {
			return rackStatus.Name == removedRack.rack
		})

		if oldRackStatus.Nodes != nil && *oldRackStatus.Nodes != 0 {
			allErrs = append(allErrs, field.Forbidden(removedRack.fieldPath, fmt.Sprintf("rack %q can't be removed because its nodes are being scaled down", removedRack.rack)))
			continue
		}

		if !isRackStatusUpToDate(old, oldRackStatus) {
			allErrs = append(allErrs, field.InternalError(removedRack.fieldPath, fmt.Errorf("rack %q can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later", removedRack.rack)))
		}
	}

	var oldDatacenterTemplateStorage, newDatacenterTemplateStorage *scyllav1alpha1.StorageOptions
	if old.Spec.DatacenterTemplate != nil && old.Spec.DatacenterTemplate.ScyllaDB != nil {
		oldDatacenterTemplateStorage = old.Spec.DatacenterTemplate.ScyllaDB.Storage
	}
	if new.Spec.DatacenterTemplate != nil && new.Spec.DatacenterTemplate.ScyllaDB != nil {
		newDatacenterTemplateStorage = new.Spec.DatacenterTemplate.ScyllaDB.Storage
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newDatacenterTemplateStorage, oldDatacenterTemplateStorage, fldPath.Child("datacenterTemplate", "scyllaDB", "storage"))...)

	var oldDatacenterTemplateRackTemplateStorage, newDatacenterTemplateRackTemplateStorage *scyllav1alpha1.StorageOptions
	if old.Spec.DatacenterTemplate != nil && old.Spec.DatacenterTemplate.RackTemplate != nil && old.Spec.DatacenterTemplate.RackTemplate.ScyllaDB != nil {
		oldDatacenterTemplateRackTemplateStorage = old.Spec.DatacenterTemplate.RackTemplate.ScyllaDB.Storage
	}
	if new.Spec.DatacenterTemplate != nil && new.Spec.DatacenterTemplate.RackTemplate != nil && new.Spec.DatacenterTemplate.RackTemplate.ScyllaDB != nil {
		newDatacenterTemplateRackTemplateStorage = new.Spec.DatacenterTemplate.RackTemplate.ScyllaDB.Storage
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newDatacenterTemplateRackTemplateStorage, oldDatacenterTemplateRackTemplateStorage, fldPath.Child("datacenterTemplate", "rackTemplate", "scyllaDB", "storage"))...)

	for _, newRack := range newRacks {
		var oldRackStorage *scyllav1alpha1.StorageOptions
		oldRack, _, ok := slices.Find(oldRacks, func(oldRack dcRackProperties) bool {
			return oldRack.datacenter == newRack.datacenter && oldRack.rack == newRack.rack && oldRack.level == newRack.level
		})
		if ok {
			oldRackStorage = oldRack.storage
		}

		allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newRack.storage, oldRackStorage, newRack.fieldPath.Child("scyllaDB", "storage"))...)
	}

	for dcIdx, newDC := range new.Spec.Datacenters {
		var oldDatacenterStorage, newDatacenterStorage *scyllav1alpha1.StorageOptions
		var oldDatacenterRackTemplateStorage, newDatacenterRackTemplateStorage *scyllav1alpha1.StorageOptions

		oldDC, _, ok := slices.Find(old.Spec.Datacenters, func(oldDC scyllav1alpha1.ScyllaDBClusterDatacenter) bool {
			return oldDC.Name == newDC.Name
		})
		if ok {
			if oldDC.ScyllaDB != nil {
				oldDatacenterStorage = oldDC.ScyllaDB.Storage
			}
			if oldDC.RackTemplate != nil && oldDC.RackTemplate.ScyllaDB != nil {
				oldDatacenterRackTemplateStorage = oldDC.RackTemplate.ScyllaDB.Storage
			}
		}

		if newDC.ScyllaDB != nil {
			newDatacenterStorage = newDC.ScyllaDB.Storage
		}
		if newDC.RackTemplate != nil && newDC.RackTemplate.ScyllaDB != nil {
			newDatacenterRackTemplateStorage = newDC.RackTemplate.ScyllaDB.Storage
		}

		allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newDatacenterStorage, oldDatacenterStorage, fldPath.Child("datacenters").Index(dcIdx).Child("scyllaDB", "storage"))...)
		allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newDatacenterRackTemplateStorage, oldDatacenterRackTemplateStorage, fldPath.Child("datacenters").Index(dcIdx).Child("rackTemplate", "scyllaDB", "storage"))...)
	}

	var oldClientBroadcastAddressType, newClientBroadcastAddressType *scyllav1alpha1.BroadcastAddressType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.BroadcastOptions != nil {
		oldClientBroadcastAddressType = pointer.Ptr(old.Spec.ExposeOptions.BroadcastOptions.Clients.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.BroadcastOptions != nil {
		newClientBroadcastAddressType = pointer.Ptr(new.Spec.ExposeOptions.BroadcastOptions.Clients.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newClientBroadcastAddressType, oldClientBroadcastAddressType, fldPath.Child("exposeOptions", "broadcastOptions", "clients", "type"))...)

	var oldNodesBroadcastAddressType, newNodesBroadcastAddressType *scyllav1alpha1.BroadcastAddressType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.BroadcastOptions != nil {
		oldNodesBroadcastAddressType = pointer.Ptr(old.Spec.ExposeOptions.BroadcastOptions.Nodes.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.BroadcastOptions != nil {
		newNodesBroadcastAddressType = pointer.Ptr(new.Spec.ExposeOptions.BroadcastOptions.Nodes.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newNodesBroadcastAddressType, oldNodesBroadcastAddressType, fldPath.Child("exposeOptions", "broadcastOptions", "nodes", "type"))...)

	var oldNodeServiceType, newNodeServiceType *scyllav1alpha1.NodeServiceType
	if old.Spec.ExposeOptions != nil && old.Spec.ExposeOptions.NodeService != nil {
		oldNodeServiceType = pointer.Ptr(old.Spec.ExposeOptions.NodeService.Type)
	}
	if new.Spec.ExposeOptions != nil && new.Spec.ExposeOptions.NodeService != nil {
		newNodeServiceType = pointer.Ptr(new.Spec.ExposeOptions.NodeService.Type)
	}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newNodeServiceType, oldNodeServiceType, fldPath.Child("exposeOptions", "nodeService", "type"))...)

	return allErrs
}
