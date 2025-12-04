package validation

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
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

	SupportedScyllaV1BroadcastAddressTypes = []scyllav1.BroadcastAddressType{
		scyllav1.BroadcastAddressTypePodIP,
		scyllav1.BroadcastAddressTypeServiceClusterIP,
		scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
	}

	schedulerTaskSpecCronParseOptions = cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
)

func ValidateScyllaCluster(c *scyllav1.ScyllaCluster) field.ErrorList {
	return ValidateScyllaClusterSpec(&c.Spec, field.NewPath("spec"))
}

func ValidateUserManagedTLSCertificateOptions(opts *scyllav1.UserManagedTLSCertificateOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(opts.SecretName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretName"), ""))
	} else {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(opts.SecretName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), opts.SecretName, msg))
		}
	}

	return allErrs
}

func ValidateOperatorManagedTLSCertificateOptions(opts *scyllav1.OperatorManagedTLSCertificateOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for _, dnsName := range opts.AdditionalDNSNames {
		for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(dnsName) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("additionalDNSNames"), opts.AdditionalDNSNames, msg))
		}
	}

	for _, ip := range opts.AdditionalIPAddresses {
		for _, fldErr := range apimachineryutilvalidation.IsValidIP(fldPath.Child("additionalIPAddresses"), ip) {
			allErrs = append(allErrs, fldErr)
		}
	}

	return allErrs
}

func ValidateTLSCertificate(cert *scyllav1.TLSCertificate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	switch cert.Type {
	case scyllav1.TLSCertificateTypeOperatorManaged:
		if cert.OperatorManagedOptions != nil {
			allErrs = append(allErrs, ValidateOperatorManagedTLSCertificateOptions(
				cert.OperatorManagedOptions,
				fldPath.Child("operatorManagedOptions"),
			)...)
		}

	case scyllav1.TLSCertificateTypeUserManaged:
		if cert.UserManagedOptions != nil {
			allErrs = append(allErrs, ValidateUserManagedTLSCertificateOptions(
				cert.UserManagedOptions,
				fldPath.Child("userManagedOptions"),
			)...)
		} else {
			allErrs = append(allErrs, field.Required(fldPath.Child("userManagedOptions"), ""))
		}

	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))

	default:
		allErrs = append(
			allErrs,
			field.NotSupported(
				fldPath.Child("type"),
				cert.Type,
				[]scyllav1.TLSCertificateType{
					scyllav1.TLSCertificateTypeOperatorManaged,
					scyllav1.TLSCertificateTypeUserManaged,
				},
			),
		)

	}

	return allErrs
}

func ValidateAlternatorSpec(alternator *scyllav1.AlternatorSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if alternator.WriteIsolation != "" {
		found := oslices.ContainsItem(AlternatorSupportedWriteIsolation, alternator.WriteIsolation)
		if !found {
			allErrs = append(
				allErrs,
				field.NotSupported(
					fldPath.Child("alternator", "writeIsolation"),
					alternator.WriteIsolation,
					AlternatorSupportedWriteIsolation,
				),
			)
		}
	}

	if alternator.InsecureEnableHTTP != nil && *alternator.InsecureEnableHTTP {
		if alternator.Port != 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("port"), "deprecated port is not allowed to be specified"))
		}
	}

	if alternator.ServingCertificate != nil {
		allErrs = append(allErrs, ValidateTLSCertificate(alternator.ServingCertificate, fldPath.Child("servingCertificate"))...)
	}

	return allErrs
}

func ValidateScyllaClusterSpec(spec *scyllav1.ScyllaClusterSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	rackNames := apimachineryutilsets.NewString()

	if spec.Alternator != nil {
		allErrs = append(allErrs, ValidateAlternatorSpec(spec.Alternator, fldPath.Child("alternator"))...)
	}

	if len(spec.ScyllaArgs) > 0 {
		allErrs = append(allErrs, ValidateScyllaArgsIPFamily(spec.IPFamily, []string{spec.ScyllaArgs}, fldPath.Child("scyllaArgs"))...)
	}

	for i, rack := range spec.Datacenter.Racks {
		allErrs = append(allErrs, ValidateScyllaClusterRackSpec(rack, rackNames, fldPath.Child("datacenter", "racks").Index(i))...)
	}

	managerRepairTaskNames := apimachineryutilsets.New[string]()
	for i, r := range spec.Repairs {
		if managerRepairTaskNames.Has(r.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("repairs").Index(i).Child("name"), r.Name))
		}
		managerRepairTaskNames.Insert(r.Name)

		allErrs = append(allErrs, ValidateRepairTaskSpec(&r, fldPath.Child("repairs").Index(i))...)
	}

	managerBackupTaskNames := apimachineryutilsets.New[string]()
	for i, b := range spec.Backups {
		if managerBackupTaskNames.Has(b.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("backups").Index(i).Child("name"), b.Name))
		}
		managerBackupTaskNames.Insert(b.Name)

		allErrs = append(allErrs, ValidateBackupTaskSpec(&b, fldPath.Child("backups").Index(i))...)
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

	if spec.MinTerminationGracePeriodSeconds != nil && *spec.MinTerminationGracePeriodSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minTerminationGracePeriodSeconds"), *spec.MinTerminationGracePeriodSeconds, "must be non-negative integer"))
	}

	if spec.MinReadySeconds != nil && *spec.MinReadySeconds < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minReadySeconds"), *spec.MinReadySeconds, "must be non-negative integer"))
	}

	return allErrs
}

func ValidateRepairTaskSpec(repairTaskSpec *scyllav1.RepairTaskSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	_, err := strconv.ParseFloat(repairTaskSpec.Intensity, 64)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("intensity"), repairTaskSpec.Intensity, "must be a float"))
	}

	allErrs = append(allErrs, ValidateTaskSpec(&repairTaskSpec.TaskSpec, fldPath)...)

	return allErrs
}

func ValidateBackupTaskSpec(backupTaskSpec *scyllav1.BackupTaskSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateTaskSpec(&backupTaskSpec.TaskSpec, fldPath)...)

	return allErrs
}

func ValidateTaskSpec(taskSpec *scyllav1.TaskSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateSchedulerTaskSpec(&taskSpec.SchedulerTaskSpec, fldPath)...)

	return allErrs
}

func ValidateSchedulerTaskSpec(schedulerTaskSpec *scyllav1.SchedulerTaskSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if schedulerTaskSpec.Cron != nil {
		_, err := cron.NewParser(schedulerTaskSpecCronParseOptions).Parse(*schedulerTaskSpec.Cron)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cron"), schedulerTaskSpec.Cron, err.Error()))
		}

		if strings.Contains(*schedulerTaskSpec.Cron, "TZ") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cron"), schedulerTaskSpec.Cron, "can't use TZ or CRON_TZ in cron, use timezone instead"))
		}
	}

	if schedulerTaskSpec.Timezone != nil {
		_, err := time.LoadLocation(*schedulerTaskSpec.Timezone)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timezone"), schedulerTaskSpec.Timezone, err.Error()))
		}
	}

	// We can only validate interval when cron is non-nil for backwards compatibility.
	if schedulerTaskSpec.Interval != nil && schedulerTaskSpec.Cron != nil {
		intervalDuration, err := duration.ParseDuration(*schedulerTaskSpec.Interval)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("interval"), schedulerTaskSpec.Interval, "valid units are d, h, m, s"))
		} else if intervalDuration != 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("interval"), "can't be non-zero when cron is specified"))
		}
	}

	if schedulerTaskSpec.Timezone != nil && schedulerTaskSpec.Cron == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("timezone"), "can't be set when cron is not specified"))
	}

	return allErrs
}

func ValidateExposeOptions(options *scyllav1.ExposeOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

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
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateBroadcastOptions(options.Clients, nodeService, fldPath.Child("clients"))...)
	allErrs = append(allErrs, ValidateBroadcastOptions(options.Nodes, nodeService, fldPath.Child("nodes"))...)

	return allErrs
}

func ValidateBroadcastOptions(options scyllav1.BroadcastOptions, nodeService *scyllav1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !oslices.ContainsItem(SupportedScyllaV1BroadcastAddressTypes, options.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), options.Type, oslices.ConvertSlice(SupportedScyllaV1BroadcastAddressTypes, oslices.ToString[scyllav1.BroadcastAddressType])))
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
	if ok && !oslices.ContainsItem(allowedNodeServiceTypes, nodeServiceType) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), options.Type, fmt.Sprintf("can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: %v", allowedNodeServiceTypes)))
	}

	return allErrs
}

func ValidateNodeService(nodeService *scyllav1.NodeServiceTemplate, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	var supportedServiceTypes = []scyllav1.NodeServiceType{
		scyllav1.NodeServiceTypeHeadless,
		scyllav1.NodeServiceTypeClusterIP,
		scyllav1.NodeServiceTypeLoadBalancer,
	}

	if !oslices.ContainsItem(supportedServiceTypes, nodeService.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), nodeService.Type, oslices.ConvertSlice(supportedServiceTypes, oslices.ToString[scyllav1.NodeServiceType])))
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
	var allErrs field.ErrorList

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

func ValidateScyllaClusterRackSpec(rack scyllav1.RackSpec, rackNames apimachineryutilsets.String, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

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

	return allErrs
}

func ValidateScyllaClusterUpdate(new, old *scyllav1.ScyllaCluster) field.ErrorList {
	allErrs := ValidateScyllaCluster(new)

	return append(allErrs, ValidateScyllaClusterSpecUpdate(new, old, field.NewPath("spec"))...)
}

func ValidateScyllaClusterSpecUpdate(new, old *scyllav1.ScyllaCluster, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Check that the datacenter name didn't change
	if old.Spec.Datacenter.Name != new.Spec.Datacenter.Name {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("datacenter", "name"), "change of datacenter name is currently not supported"))
	}

	// Check that all rack names are the same as before
	oldRackNames, newRackNames := apimachineryutilsets.NewString(), apimachineryutilsets.NewString()
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

func GetWarningsOnScyllaClusterCreate(sc *scyllav1.ScyllaCluster) []string {
	var warnings []string

	warnings = append(warnings, warningsForScyllaClusterSpec(&sc.Spec, field.NewPath("spec"))...)

	return warnings
}

func warningsForScyllaClusterSpec(spec *scyllav1.ScyllaClusterSpec, fldPath *field.Path) []string {
	var warnings []string

	if len(spec.Sysctls) > 0 {
		warnings = append(warnings, fmt.Sprintf("%s: deprecated; use NodeConfig's .spec.sysctls instead", fldPath.Child("sysctls")))
	}

	return warnings
}

func GetWarningsOnScyllaClusterUpdate(new, old *scyllav1.ScyllaCluster) []string {
	var warnings []string

	warnings = append(warnings, warningsForScyllaClusterSpec(&new.Spec, field.NewPath("spec"))...)

	return warnings
}
