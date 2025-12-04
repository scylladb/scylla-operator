// Copyright (c) 2024 ScyllaDB.

package validation

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	imgreference "github.com/containers/image/v5/docker/reference"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corevalidation "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/apis/core/validation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	SupportedScyllaV1Alpha1BroadcastAddressTypes = []scyllav1alpha1.BroadcastAddressType{
		scyllav1alpha1.BroadcastAddressTypePodIP,
		scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
		scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
	}

	allowedNodeServiceTypesByBroadcastAddressType = map[scyllav1alpha1.BroadcastAddressType][]scyllav1alpha1.NodeServiceType{
		scyllav1alpha1.BroadcastAddressTypeServiceClusterIP: {
			scyllav1alpha1.NodeServiceTypeClusterIP,
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
		scyllav1alpha1.BroadcastAddressTypePodIP: {
			scyllav1alpha1.NodeServiceTypeHeadless,
			scyllav1alpha1.NodeServiceTypeClusterIP,
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
		scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress: {
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
	}

	supportedNodeServiceTypes = []scyllav1alpha1.NodeServiceType{
		scyllav1alpha1.NodeServiceTypeHeadless,
		scyllav1alpha1.NodeServiceTypeClusterIP,
		scyllav1alpha1.NodeServiceTypeLoadBalancer,
	}
)

func ValidateScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBDatacenterSpec(&sdc.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBDatacenterSpec(spec *scyllav1alpha1.ScyllaDBDatacenterSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDB(&spec.ScyllaDB, fldPath.Child("scyllaDB"))...)
	allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBManagerAgent(spec.ScyllaDBManagerAgent, fldPath.Child("scyllaDBManagerAgent"))...)

	if spec.ScyllaDB.AdditionalScyllaDBArguments != nil {
		allErrs = append(allErrs, ValidateScyllaArgsIPFamily(spec.IPFamily, spec.ScyllaDB.AdditionalScyllaDBArguments, fldPath.Child("scyllaDB", "additionalScyllaDBArguments"))...)
	}

	allErrs = append(allErrs, validateStructSliceFieldUniqueness(spec.Racks, func(rackSpec scyllav1alpha1.RackSpec) string {
		return rackSpec.Name
	}, "name", fldPath.Child("racks"))...)

	if spec.RackTemplate != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterRackTemplate(spec.RackTemplate, fldPath.Child("rackTemplate"))...)
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
		allErrs = append(allErrs, ValidateScyllaDBDatacenterSpecExposeOptions(spec.ExposeOptions, fldPath.Child("exposeOptions"))...)
	}

	if spec.MinTerminationGracePeriodSeconds != nil && *spec.MinTerminationGracePeriodSeconds < 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*spec.MinTerminationGracePeriodSeconds), fldPath.Child("minTerminationGracePeriodSeconds"))...)
	}

	if spec.MinReadySeconds != nil && *spec.MinReadySeconds < 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*spec.MinReadySeconds), fldPath.Child("minReadySeconds"))...)
	}

	return allErrs
}

func ValidateScyllaDBDatacenterRackTemplate(rackTemplate *scyllav1alpha1.RackTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if rackTemplate.Nodes != nil && *rackTemplate.Nodes < 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*rackTemplate.Nodes), fldPath.Child("nodes"))...)
	}

	if rackTemplate.TopologyLabelSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabels(rackTemplate.TopologyLabelSelector, fldPath.Child("topologyLabelSelector"))...)
	}

	if rackTemplate.ScyllaDB != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBTemplate(rackTemplate.ScyllaDB, fldPath.Child("scyllaDB"))...)
	}

	if rackTemplate.ScyllaDBManagerAgent != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterScyllaDBManagerAgentTemplate(rackTemplate.ScyllaDBManagerAgent, fldPath.Child("scyllaDBManagerAgent"))...)
	}

	// TODO: Add placement validation in >=v1alpha2

	return allErrs
}

func ValidateScyllaDBDatacenterScyllaDBManagerAgentTemplate(scyllaDBManagerAgentTemplate *scyllav1alpha1.ScyllaDBManagerAgentTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if scyllaDBManagerAgentTemplate.CustomConfigSecretRef != nil {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(*scyllaDBManagerAgentTemplate.CustomConfigSecretRef, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("customConfigSecretRef"), *scyllaDBManagerAgentTemplate.CustomConfigSecretRef, msg))
		}
	}

	return allErrs
}

func ValidateScyllaDBDatacenterScyllaDBTemplate(scyllaDBTemplate *scyllav1alpha1.ScyllaDBTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if scyllaDBTemplate.Storage != nil {
		if scyllaDBTemplate.Storage.Metadata != nil {
			allErrs = append(allErrs, metav1validation.ValidateLabels(scyllaDBTemplate.Storage.Metadata.Labels, fldPath.Child("storage", "metadata", "labels"))...)
			allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(scyllaDBTemplate.Storage.Metadata.Annotations, fldPath.Child("storage", "metadata", "annotations"))...)
		}

		storageCapacity, err := resource.ParseQuantity(scyllaDBTemplate.Storage.Capacity)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("storage", "capacity"), scyllaDBTemplate.Storage.Capacity, fmt.Sprintf("unable to parse capacity: %v", err)))
		} else if storageCapacity.CmpInt64(0) <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("storage", "capacity"), scyllaDBTemplate.Storage.Capacity, "must be greater than zero"))
		}

		if scyllaDBTemplate.Storage.StorageClassName != nil {
			for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(*scyllaDBTemplate.Storage.StorageClassName, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("storage", "storageClassName"), *scyllaDBTemplate.Storage.StorageClassName, msg))
			}
		}
	}

	if scyllaDBTemplate.CustomConfigMapRef != nil {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(*scyllaDBTemplate.CustomConfigMapRef, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("customConfigMapRef"), *scyllaDBTemplate.CustomConfigMapRef, msg))
		}
	}

	return allErrs
}

func ValidateScyllaDBDatacenterScyllaDB(scyllaDB *scyllav1alpha1.ScyllaDB, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(scyllaDB.Image) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("image"), "must not be empty"))
	} else {
		_, err := imgreference.Parse(scyllaDB.Image)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("image"), scyllaDB.Image, fmt.Sprintf("unable to parse image: %v", err)))
		}
	}

	if scyllaDB.AlternatorOptions != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterAlternatorOptions(scyllaDB.AlternatorOptions, fldPath.Child("alternator"))...)
	}

	return allErrs
}

func ValidateScyllaDBDatacenterScyllaDBManagerAgent(scyllaDBManagerAgent *scyllav1alpha1.ScyllaDBManagerAgent, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if scyllaDBManagerAgent == nil || scyllaDBManagerAgent.Image == nil || len(*scyllaDBManagerAgent.Image) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("image"), "must not be empty"))
	} else {
		_, err := imgreference.Parse(*scyllaDBManagerAgent.Image)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("image"), *scyllaDBManagerAgent.Image, fmt.Sprintf("unable to parse image: %v", err)))
		}
	}

	return allErrs
}

func ValidateScyllaDBDatacenterSpecExposeOptions(options *scyllav1alpha1.ExposeOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if options.CQL != nil && options.CQL.Ingress != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterIngressOptions(options, fldPath)...)
	}

	if options.NodeService != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterNodeService(options, fldPath)...)
	}

	if options.BroadcastOptions != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterSpecExposeOptionsNodeBroadcastOptions(options.BroadcastOptions, options.NodeService, fldPath.Child("broadcastOptions"))...)
	}

	return allErrs
}

func ValidateScyllaDBDatacenterIngressOptions(options *scyllav1alpha1.ExposeOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(options.CQL.Ingress.IngressClassName) != 0 {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(options.CQL.Ingress.IngressClassName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cql", "ingress", "ingressClassName"), options.CQL.Ingress.IngressClassName, msg))
		}
	}

	if len(options.CQL.Ingress.Annotations) != 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(options.CQL.Ingress.Annotations, fldPath.Child("cql", "ingress", "annotations"))...)
	}
	return allErrs
}

func ValidateScyllaDBDatacenterNodeService(options *scyllav1alpha1.ExposeOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(options.NodeService.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodeService", "type"), fmt.Sprintf("supported values: %s", strings.Join(oslices.ConvertSlice(supportedNodeServiceTypes, oslices.ToString[scyllav1alpha1.NodeServiceType]), ", "))))
	} else {
		allErrs = append(allErrs, validateEnum(options.NodeService.Type, supportedNodeServiceTypes, fldPath.Child("nodeService", "type"))...)
	}

	if options.NodeService.LoadBalancerClass != nil && len(*options.NodeService.LoadBalancerClass) != 0 {
		for _, msg := range apimachineryutilvalidation.IsQualifiedName(*options.NodeService.LoadBalancerClass) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("nodeService", "loadBalancerClass"), *options.NodeService.LoadBalancerClass, msg))
		}
	}

	if len(options.NodeService.Annotations) != 0 {
		allErrs = append(allErrs, apimachineryvalidation.ValidateAnnotations(options.NodeService.Annotations, fldPath.Child("nodeService", "annotations"))...)
	}
	return allErrs
}

func ValidateScyllaDBDatacenterSpecExposeOptionsNodeBroadcastOptions(options *scyllav1alpha1.NodeBroadcastOptions, nodeService *scyllav1alpha1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	var nodeServiceType *scyllav1alpha1.NodeServiceType
	if nodeService != nil {
		nodeServiceType = pointer.Ptr(nodeService.Type)
	}

	var allowedNodeServiceTypesByBroadcastAddressType = map[scyllav1alpha1.BroadcastAddressType][]scyllav1alpha1.NodeServiceType{
		scyllav1alpha1.BroadcastAddressTypePodIP: {
			scyllav1alpha1.NodeServiceTypeHeadless,
			scyllav1alpha1.NodeServiceTypeClusterIP,
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
		scyllav1alpha1.BroadcastAddressTypeServiceClusterIP: {
			scyllav1alpha1.NodeServiceTypeClusterIP,
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
		scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress: {
			scyllav1alpha1.NodeServiceTypeLoadBalancer,
		},
	}

	allErrs = append(allErrs,
		ValidateScyllaDBDatacenterBroadcastOptions(
			options.Clients.Type,
			SupportedScyllaV1Alpha1BroadcastAddressTypes,
			scyllav1alpha1.NodeServiceTypeClusterIP,
			nodeServiceType,
			allowedNodeServiceTypesByBroadcastAddressType,
			fldPath.Child("clients"),
		)...,
	)

	allErrs = append(allErrs,
		ValidateScyllaDBDatacenterBroadcastOptions(
			options.Nodes.Type,
			SupportedScyllaV1Alpha1BroadcastAddressTypes,
			scyllav1alpha1.NodeServiceTypeClusterIP,
			nodeServiceType,
			allowedNodeServiceTypesByBroadcastAddressType,
			fldPath.Child("nodes"),
		)...,
	)

	return allErrs
}

func ValidateScyllaDBDatacenterBroadcastOptions[BT ~string, ST ~string](broadcastAddressType BT, supportedBroadcastedTypes []BT, defaultNodeServiceType ST, nodeServiceType *ST, allowedNodeServiceTypesByBroadcastAddressType map[BT][]ST, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateEnum(broadcastAddressType, supportedBroadcastedTypes, fldPath.Child("type"))...)

	serviceType := defaultNodeServiceType
	if nodeServiceType != nil {
		serviceType = *nodeServiceType
	}

	// Skipping an error when chosen option type is unsupported as it won't help anyhow users reading it.
	allowedNodeServiceTypes, ok := allowedNodeServiceTypesByBroadcastAddressType[broadcastAddressType]
	if ok && !oslices.ContainsItem(allowedNodeServiceTypes, serviceType) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), broadcastAddressType, fmt.Sprintf("can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: %v", allowedNodeServiceTypes)))
	}

	return allErrs
}

func ValidateScyllaDBDatacenterAlternatorOptions(alternator *scyllav1alpha1.AlternatorOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if alternator.WriteIsolation != "" {
		found := oslices.ContainsItem(AlternatorSupportedWriteIsolation, alternator.WriteIsolation)
		if !found {
			allErrs = append(allErrs, field.NotSupported(fldPath, alternator.WriteIsolation, AlternatorSupportedWriteIsolation))
		}
	}

	if alternator.ServingCertificate != nil {
		allErrs = append(allErrs, ValidateScyllaDBDatacenterTLSCertificate(alternator.ServingCertificate, fldPath.Child("servingCertificate"))...)
	}

	return allErrs
}

func ValidateScyllaDBDatacenterTLSCertificate(servingCertificate *scyllav1alpha1.TLSCertificate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	switch servingCertificate.Type {
	case scyllav1alpha1.TLSCertificateTypeOperatorManaged:
		if servingCertificate.OperatorManagedOptions != nil {
			allErrs = append(allErrs, ValidateScyllaDBDatacenterOperatorManagedTLSCertificateOptions(
				servingCertificate.OperatorManagedOptions,
				fldPath.Child("operatorManagedOptions"),
			)...)
		}

	case scyllav1alpha1.TLSCertificateTypeUserManaged:
		if servingCertificate.UserManagedOptions != nil {
			allErrs = append(allErrs, ValidateScyllaDBDatacenterUserManagedTLSCertificateOptions(
				servingCertificate.UserManagedOptions,
				fldPath.Child("userManagedOptions"),
			)...)
		} else {
			allErrs = append(allErrs, field.Required(fldPath.Child("userManagedOptions"), ""))
		}

	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))

	default:
		allErrs = append(allErrs, field.NotSupported(
			fldPath.Child("type"),
			servingCertificate.Type,
			[]scyllav1alpha1.TLSCertificateType{
				scyllav1alpha1.TLSCertificateTypeOperatorManaged,
				scyllav1alpha1.TLSCertificateTypeUserManaged,
			},
		))
	}
	return allErrs
}

func ValidateScyllaDBDatacenterUserManagedTLSCertificateOptions(options *scyllav1alpha1.UserManagedTLSCertificateOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(options.SecretName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretName"), ""))
	} else {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(options.SecretName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), options.SecretName, msg))
		}
	}
	return allErrs
}

func ValidateScyllaDBDatacenterOperatorManagedTLSCertificateOptions(options *scyllav1alpha1.OperatorManagedTLSCertificateOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for _, dnsName := range options.AdditionalDNSNames {
		for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(dnsName) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("additionalDNSNames"), options.AdditionalDNSNames, msg))
		}
	}

	for _, ip := range options.AdditionalIPAddresses {
		for _, fldErr := range apimachineryutilvalidation.IsValidIP(fldPath.Child("additionalIPAddresses"), ip) {
			allErrs = append(allErrs, fldErr)
		}
	}
	return allErrs
}

func ValidateScyllaDBDatacenterPlacement(placement *scyllav1alpha1.Placement, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if placement.NodeAffinity != nil {
		allErrs = append(allErrs, corevalidation.ValidateNodeAffinity(placement.NodeAffinity, fldPath.Child("nodeAffinity"))...)
	}

	if placement.PodAntiAffinity != nil {
		allErrs = append(allErrs, corevalidation.ValidatePodAntiAffinity(placement.PodAntiAffinity, false, fldPath.Child("podAntiAffinity"))...)
	}

	if placement.PodAffinity != nil {
		allErrs = append(allErrs, corevalidation.ValidatePodAffinity(placement.PodAffinity, false, fldPath.Child("podAffinity"))...)
	}

	if placement.Tolerations != nil {
		allErrs = append(allErrs, corevalidation.ValidateTolerations(placement.Tolerations, fldPath.Child("tolerations"))...)
	}

	return allErrs
}

func ValidateScyllaDBDatacenterUpdate(new, old *scyllav1alpha1.ScyllaDBDatacenter) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBDatacenter(new)...)
	allErrs = append(allErrs, ValidateScyllaDBDatacenterSpecUpdate(new, old, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBDatacenterSpecUpdate(new, old *scyllav1alpha1.ScyllaDBDatacenter, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(new.Spec.ClusterName, old.Spec.ClusterName, fldPath.Child("clusterName"))...)

	oldRackNames := oslices.ConvertSlice(old.Spec.Racks, func(rackSpec scyllav1alpha1.RackSpec) string {
		return rackSpec.Name
	})
	newRackNames := oslices.ConvertSlice(new.Spec.Racks, func(rackSpec scyllav1alpha1.RackSpec) string {
		return rackSpec.Name
	})

	removedRackNames := apimachineryutilsets.New(oldRackNames...).Difference(apimachineryutilsets.New(newRackNames...)).UnsortedList()
	sort.Strings(removedRackNames)

	isRackStatusUpToDate := func(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackStatus scyllav1alpha1.RackStatus) bool {
		return sdc.Status.ObservedGeneration != nil && *sdc.Status.ObservedGeneration >= sdc.Generation && rackStatus.Stale != nil && !*rackStatus.Stale
	}

	for _, removedRackName := range removedRackNames {
		for i, oldRack := range old.Spec.Racks {
			if oldRack.Name != removedRackName {
				continue
			}

			oldRackNodeCount := int32(0)
			if old.Spec.RackTemplate != nil && old.Spec.RackTemplate.Nodes != nil {
				oldRackNodeCount = *old.Spec.RackTemplate.Nodes
			}
			if oldRack.Nodes != nil {
				oldRackNodeCount = *oldRack.Nodes
			}

			if oldRackNodeCount != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i), fmt.Sprintf("rack %q can't be removed because it still has members that have to be scaled down to zero first", removedRackName)))
				continue
			}

			oldRackStatus, _, ok := oslices.Find(old.Status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
				return rackStatus.Name == removedRackName
			})
			if !ok {
				continue
			}

			if oldRackStatus.Nodes != nil && *oldRackStatus.Nodes != 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i), fmt.Sprintf("rack %q can't be removed because the members are being scaled down", removedRackName)))
				continue
			}

			if !isRackStatusUpToDate(old, oldRackStatus) {
				allErrs = append(allErrs, field.InternalError(fldPath.Child("racks").Index(i), fmt.Errorf("rack %q can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later", removedRackName)))
			}
		}
	}

	for i, newRack := range new.Spec.Racks {
		oldRack, _, ok := oslices.Find(old.Spec.Racks, func(spec scyllav1alpha1.RackSpec) bool {
			return spec.Name == newRack.Name
		})
		if !ok {
			continue
		}

		var newRackStorage scyllav1alpha1.StorageOptions
		if new.Spec.RackTemplate != nil && new.Spec.RackTemplate.ScyllaDB != nil && new.Spec.RackTemplate.ScyllaDB.Storage != nil {
			newRackStorage = *new.Spec.RackTemplate.ScyllaDB.Storage
		}
		if newRack.ScyllaDB != nil && newRack.ScyllaDB.Storage != nil {
			newRackStorage = *newRack.ScyllaDB.Storage
		}

		var oldRackStorage scyllav1alpha1.StorageOptions
		if old.Spec.RackTemplate != nil && old.Spec.RackTemplate.ScyllaDB != nil && old.Spec.RackTemplate.ScyllaDB.Storage != nil {
			oldRackStorage = *old.Spec.RackTemplate.ScyllaDB.Storage
		}
		if oldRack.ScyllaDB != nil && oldRack.ScyllaDB.Storage != nil {
			oldRackStorage = *oldRack.ScyllaDB.Storage
		}

		if !reflect.DeepEqual(oldRackStorage, newRackStorage) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("racks").Index(i).Child("scyllaDB", "storage"), "changes in storage are currently not supported"))
		}
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

func validateStructSliceFieldUniqueness[E any, F comparable](s []E, mapFunc func(E) F, fieldSubPath string, structPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	set := apimachineryutilsets.New[F]()
	for i, e := range s {
		f := mapFunc(e)

		if set.Has(f) {
			allErrs = append(allErrs, field.Duplicate(structPath.Index(i).Child(fieldSubPath), f))
		}
		set.Insert(f)
	}

	return allErrs
}

func validateEnum[E ~string](value E, supported []E, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !oslices.ContainsItem(supported, value) {
		allErrs = append(allErrs, field.NotSupported(fldPath, value, oslices.ConvertSlice(supported, oslices.ToString[E])))
	}

	return allErrs
}

func ValidateScyllaArgsIPFamily(ipFamily *corev1.IPFamily, scyllaArgs []string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(scyllaArgs) == 0 {
		return allErrs
	}

	effectiveIPFamily := corev1.IPv4Protocol
	if ipFamily != nil {
		effectiveIPFamily = *ipFamily
	}

	argsStr := strings.Join(scyllaArgs, " ")

	allErrs = append(allErrs, validateScyllaArgIPAddress("--rpc-address", effectiveIPFamily, argsStr, fldPath)...)
	allErrs = append(allErrs, validateScyllaArgIPAddress("--listen-address", effectiveIPFamily, argsStr, fldPath)...)

	return allErrs
}

func validateScyllaArgIPAddress(argName string, expectedIPFamily corev1.IPFamily, argsStr string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	idx := strings.Index(argsStr, argName)
	if idx == -1 {
		return allErrs
	}

	value := extractArgValue(argsStr[idx+len(argName):])
	if value == "" || value == "0.0.0.0" || value == "::" {
		return allErrs
	}

	valueIsIPv6 := strings.Contains(value, ":")
	expectedIsIPv6 := expectedIPFamily == corev1.IPv6Protocol

	if valueIsIPv6 != expectedIsIPv6 {
		expectedFamily := "IPv4"
		gotFamily := "IPv4"
		if expectedIsIPv6 {
			expectedFamily = "IPv6"
		}
		if valueIsIPv6 {
			gotFamily = "IPv6"
		}
		allErrs = append(allErrs, field.Invalid(
			fldPath,
			argsStr,
			fmt.Sprintf("%s '%s' IP family (%s) must match spec.ipFamily (%s)", argName, value, gotFamily, expectedFamily),
		))
	}

	return allErrs
}

func extractArgValue(remaining string) string {
	remaining = strings.TrimLeft(remaining, "= ")

	if strings.HasPrefix(remaining, "\"") {
		if endIdx := strings.Index(remaining[1:], "\""); endIdx != -1 {
			return remaining[1 : endIdx+1]
		}
		return ""
	}

	if spaceIdx := strings.IndexAny(remaining, " \t"); spaceIdx != -1 {
		return remaining[:spaceIdx]
	}

	return remaining
}

func GetWarningsOnScyllaDBDatacenterCreate(sdc *scyllav1alpha1.ScyllaDBDatacenter) []string {
	return nil
}

func GetWarningsOnScyllaDBDatacenterUpdate(new, old *scyllav1alpha1.ScyllaDBDatacenter) []string {
	return nil
}
