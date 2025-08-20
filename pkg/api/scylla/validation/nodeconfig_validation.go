// Copyright (c) 2023 ScyllaDB.

package validation

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateNodeConfig(nc *scyllav1alpha1.NodeConfig) field.ErrorList {
	return ValidateNodeConfigSpec(&nc.Spec, field.NewPath("spec"))
}

func ValidateNodeConfigSpec(spec *scyllav1alpha1.NodeConfigSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.LocalDiskSetup != nil {
		allErrs = append(allErrs, ValidateLocalDiskSetup(spec.LocalDiskSetup, fldPath.Child("localDiskSetup"))...)
	}

	return allErrs
}

func ValidateLocalDiskSetup(lds *scyllav1alpha1.LocalDiskSetup, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateLocalDiskSetupFilesystems(lds.Filesystems, fldPath.Child("filesystems"))...)

	allErrs = append(allErrs, ValidateLocalDiskSetupRAIDs(lds.RAIDs, fldPath.Child("raids"))...)

	allErrs = append(allErrs, ValidateLocalDiskSetupMounts(lds.Mounts, fldPath.Child("mounts"))...)

	return allErrs
}

func ValidateLocalDiskSetupFilesystems(fcs []scyllav1alpha1.FilesystemConfiguration, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	return allErrs
}

func ValidateLocalDiskSetupMounts(mcs []scyllav1alpha1.MountConfiguration, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	mountPoints := map[string]struct{}{}

	for i, mc := range mcs {
		_, ok := mountPoints[mc.MountPoint]
		if ok {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("mountPoint"), mc.MountPoint))
		}
		mountPoints[mc.MountPoint] = struct{}{}
	}

	return allErrs
}

func ValidateLocalDiskSetupRAIDs(rcs []scyllav1alpha1.RAIDConfiguration, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	names := map[string]struct{}{}

	for i, rc := range rcs {
		_, ok := names[rc.Name]
		if ok {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), rc.Name))
		}
		names[rc.Name] = struct{}{}

		if rc.Type == scyllav1alpha1.RAID0Type && rc.RAID0 == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("RAID0"), "", "RAID0 options must be provided when RAID0 type is set"))
		}

		if rc.RAID0 != nil {
			if len(rc.RAID0.Devices.NameRegex) == 0 && len(rc.RAID0.Devices.ModelRegex) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("RAID0").Child("devices"), "", "nameRegex or modelRegex must be provided"))
			}
		}
	}

	return allErrs
}

func ValidateNodeConfigUpdate(new, old *scyllav1alpha1.NodeConfig) field.ErrorList {
	allErrs := ValidateNodeConfig(new)

	return append(allErrs, ValidateNodeConfigSpecUpdate(new, old, field.NewPath("spec"))...)
}

func ValidateNodeConfigSpecUpdate(new, old *scyllav1alpha1.NodeConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	oldLoopDevicesSizes := make(map[string]resource.Quantity)
	if old.Spec.LocalDiskSetup != nil {
		for _, ldc := range old.Spec.LocalDiskSetup.LoopDevices {
			oldLoopDevicesSizes[ldc.Name] = ldc.Size
		}
	}

	if new.Spec.LocalDiskSetup != nil {
		for i, ld := range new.Spec.LocalDiskSetup.LoopDevices {
			oldSize, ok := oldLoopDevicesSizes[ld.Name]
			if ok && !ld.Size.Equal(oldSize) {
				allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(ld.Size.String(), oldSize.String(), fldPath.Child("localDiskSetup", "loopDevices").Index(i).Child("size"))...)
			}
		}
	}

	return allErrs
}

func GetWarningsOnNodeConfigCreate(nc *scyllav1alpha1.NodeConfig) []string {
	return nil
}

func GetWarningsOnNodeConfigUpdate(new, old *scyllav1alpha1.NodeConfig) []string {
	return nil
}
