// Copyright (C) 2025 ScyllaDB

package validation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corevalidation "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/apis/core/validation"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	supportedScyllaDBManagerTaskTypes = []scyllav1alpha1.ScyllaDBManagerTaskType{
		scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
		scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
	}
)

type ValidateScyllaDBManagerTaskObjectMetaOptions struct {
	ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions
}

type ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions struct {
	// TODO: change this to IsScheduleCronDefined
	ValidateIntervalOverride func(string, bool, *field.Path) field.ErrorList
	ValidateTimezoneOverride func(string, bool, *field.Path) field.ErrorList
}

// TODO: fix nested embedding conflicts
type ValidateScyllaDBManagerTaskSpecOptions struct {
	ValidateScyllaDBManagerBackupTaskOptionsOptions
	ValidateScyllaDBManagerRepairTaskOptionsOptions
}

type ValidateScyllaDBManagerBackupTaskOptionsOptions struct {
	ValidateScyllaDBManagerTaskScheduleOptions
}

type ValidateScyllaDBManagerRepairTaskOptionsOptions struct {
	ValidateScyllaDBManagerTaskScheduleOptions
}

type ValidateScyllaDBManagerTaskScheduleOptions struct {
}

func ValidateScyllaDBManagerTask(smt *scyllav1alpha1.ScyllaDBManagerTask) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: make object meta validation options
	validateObjectMetaOptions := &ValidateScyllaDBManagerTaskObjectMetaOptions{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskObjectMeta(&smt.ObjectMeta, validateObjectMetaOptions, field.NewPath("metadata"))...)

	// TODO: make task spec validation options
	validateTaskSpecOptions := &ValidateScyllaDBManagerTaskSpecOptions{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSpec(&smt.Spec, validateTaskSpecOptions, field.NewPath("spec"))...)

	return allErrs
}

func makeValidateScyllaDBManagerTaskObjectMetaOptions(smt *scyllav1alpha1.ScyllaDBManagerTask) *ValidateScyllaDBManagerTaskObjectMetaOptions {
	options := &ValidateScyllaDBManagerTaskObjectMetaOptions{
		ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions: ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions{
			ValidateIntervalOverride: nil,
			ValidateTimezoneOverride: nil,
		},
	}

	isScheduleCronDefined := (smt.Spec.Backup != nil && smt.Spec.Backup.Cron != nil) || (smt.Spec.Repair != nil && smt.Spec.Repair.Cron != nil)
	if isScheduleCronDefined {
		options.ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions.ValidateIntervalOverride =
			func(interval string, hasInterval bool, fldPath *field.Path) field.ErrorList {
				allErrs := field.ErrorList{}

				if !hasInterval {
					return allErrs
				}

				intervalDuration, err := duration.ParseDuration(interval)
				if err != nil {
					allErrs = append(allErrs, field.Invalid(fldPath, interval, "valid units are d, h, m, s"))
				} else if intervalDuration != 0 {
					allErrs = append(allErrs, field.Forbidden(fldPath, "can't be non-zero when cron is specified"))
				}

				return allErrs
			}

		options.ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions.ValidateTimezoneOverride =
			func(timezone string, hasTimezone bool, fldPath *field.Path) field.ErrorList {
				allErrs := field.ErrorList{}

				if !hasTimezone {
					return allErrs
				}

				_, err := time.LoadLocation(timezone)
				if err != nil {
					allErrs = append(allErrs, field.Invalid(fldPath, timezone, err.Error()))
				}

				return allErrs
			}
	}

	return options
}

func ValidateScyllaDBManagerTaskObjectMeta(meta *metav1.ObjectMeta, options *ValidateScyllaDBManagerTaskObjectMetaOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskObjectMetaAnnotations(meta.Annotations, &options.ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions, fldPath.Child("annotations"))...)

	return allErrs
}

func ValidateScyllaDBManagerTaskObjectMetaAnnotations(annotations map[string]string, options *ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	nameOverrideAnnotation, hasNameOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(nameOverrideAnnotation, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskNameOverrideAnnotation), nameOverrideAnnotation, msg))
		}
	}

	intervalOverrideAnnotation, hasIntervalOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation]
	if hasIntervalOverrideAnnotation && options.ValidateIntervalOverride != nil {
		allErrs = append(allErrs, options.ValidateIntervalOverride(intervalOverrideAnnotation, hasIntervalOverrideAnnotation, fldPath.Key(naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation))...)
	}

	timezoneOverrideAnnotation, hasTimezoneOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation]
	if hasTimezoneOverrideAnnotation {
		_, err := time.LoadLocation(timezoneOverrideAnnotation)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation), timezoneOverrideAnnotation, err.Error()))
		}

		if options.ValidateTimezoneOverride != nil {
			allErrs = append(allErrs, options.ValidateTimezoneOverride(timezoneOverrideAnnotation, hasTimezoneOverrideAnnotation, fldPath.Key(naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation))...)
		}
	}

	repairIntensityOverrideAnnotation, hasRepairIntensityOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation]
	if hasRepairIntensityOverrideAnnotation {
		_, err := strconv.ParseFloat(repairIntensityOverrideAnnotation, 64)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation), repairIntensityOverrideAnnotation, "must be a float"))
		}
	}

	/*
		TODO:
		OK naming.ScyllaDBManagerTaskNameOverrideAnnotation
		OK, HAVE TO ADD CRON naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation
		CAN'T STRENGTHEN naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation
		HAVE TO ADD CRON naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation
		OK naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation

		backup location
		repair smallTableThreshold?
	*/

	return allErrs
}

func ValidateScyllaDBManagerTaskSpec(spec *scyllav1alpha1.ScyllaDBManagerTaskSpec, options *ValidateScyllaDBManagerTaskSpecOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateLocalScyllaDBReference(&spec.ScyllaDBClusterRef, fldPath.Child("scyllaDBClusterRef"))...)

	// TODO: switch to enum?
	switch spec.Type {
	case scyllav1alpha1.ScyllaDBManagerTaskTypeBackup:
		if spec.Backup == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("backup"), fmt.Sprintf("backup options are required when task type is %q", scyllav1alpha1.ScyllaDBManagerTaskTypeBackup)))
			break
		}

		allErrs = append(allErrs, ValidateScyllaDBManagerBackupTaskOptions(spec.Backup, &options.ValidateScyllaDBManagerBackupTaskOptionsOptions, fldPath.Child("backup"))...)

	case scyllav1alpha1.ScyllaDBManagerTaskTypeRepair:
		if spec.Repair == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("repair"), fmt.Sprintf("repair options are required when task type is %q", scyllav1alpha1.ScyllaDBManagerTaskTypeRepair)))
			break
		}

		allErrs = append(allErrs, ValidateScyllaDBManagerRepairTaskOptions(spec.Repair, &options.ValidateScyllaDBManagerRepairTaskOptionsOptions, fldPath.Child("repair"))...)

	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), spec.Type, slices.ConvertSlice(supportedScyllaDBManagerTaskTypes, slices.ToString)))

	}

	if spec.Type != scyllav1alpha1.ScyllaDBManagerTaskTypeBackup && spec.Backup != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("backup"), fmt.Sprintf("backup options are forbidden when task type is not %q", scyllav1alpha1.ScyllaDBManagerTaskTypeBackup)))
	}

	if spec.Type != scyllav1alpha1.ScyllaDBManagerTaskTypeRepair && spec.Repair != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("repair"), fmt.Sprintf("repair options are forbidden when task type is not %q", scyllav1alpha1.ScyllaDBManagerTaskTypeRepair)))
	}

	return allErrs
}

func ValidateScyllaDBManagerBackupTaskOptions(backupOptions *scyllav1alpha1.ScyllaDBManagerBackupTaskOptions, options *ValidateScyllaDBManagerBackupTaskOptionsOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSchedule(&backupOptions.ScyllaDBManagerTaskSchedule, &options.ValidateScyllaDBManagerTaskScheduleOptions, fldPath)...)

	// TODO: annotation override
	if backupOptions.Location == nil || len(backupOptions.Location) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("location"), "location must not be empty"))
	}

	return allErrs
}

func ValidateScyllaDBManagerRepairTaskOptions(repairOptions *scyllav1alpha1.ScyllaDBManagerRepairTaskOptions, options *ValidateScyllaDBManagerRepairTaskOptionsOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSchedule(&repairOptions.ScyllaDBManagerTaskSchedule, &options.ValidateScyllaDBManagerTaskScheduleOptions, fldPath)...)

	if repairOptions.Intensity != nil && *repairOptions.Intensity < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("intensity"), *repairOptions.Intensity, "can't be negative"))
	}

	if repairOptions.SmallTableThreshold != nil {
		allErrs = append(allErrs, corevalidation.ValidateResourceQuantityValue(corev1.ResourceStorage, *repairOptions.SmallTableThreshold, fldPath.Child("smallTableThreshold"))...)
	}

	return allErrs
}

func ValidateScyllaDBManagerTaskSchedule(schedule *scyllav1alpha1.ScyllaDBManagerTaskSchedule, options *ValidateScyllaDBManagerTaskScheduleOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schedule.Cron != nil {
		_, err := cron.NewParser(schedulerTaskSpecCronParseOptions).Parse(*schedule.Cron)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cron"), schedule.Cron, err.Error()))
		}

		if strings.Contains(*schedule.Cron, "TZ") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cron"), schedule.Cron, "TZ and CRON_TZ prefixes are forbidden"))
		}
	}

	return allErrs
}

func ValidateScyllaDBManagerTaskUpdate(new, old *scyllav1alpha1.ScyllaDBManagerTask) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTask(new)...)

	// TODO: update

	return allErrs
}
