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
	ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions
}

type ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions struct {
	IsScyllaDBManagerTaskScheduleCronNil              bool
	IsScyllaDBManagerTaskRepairIntensityNil           bool
	IsScyllaDBManagerTaskRepairSmallTableThresholdNil bool
}

type ValidateScyllaDBManagerTaskSpecOptions struct {
	ValidateScyllaDBManagerBackupTaskOptionsOptions ValidateScyllaDBManagerBackupTaskOptionsOptions
}

type ValidateScyllaDBManagerBackupTaskOptionsOptions struct {
	IsLocationValidationDisabled bool
}

func ValidateScyllaDBManagerTask(smt *scyllav1alpha1.ScyllaDBManagerTask) field.ErrorList {
	allErrs := field.ErrorList{}

	validateObjectMetaOptions := makeValidateScyllaDBManagerTaskObjectMetaOptions(smt)
	allErrs = append(allErrs, ValidateScyllaDBManagerTaskObjectMeta(&smt.ObjectMeta, validateObjectMetaOptions, field.NewPath("metadata"))...)

	validateTaskSpecOptions := makeValidateScyllaDBManagerTaskSpecOptions(smt)
	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSpec(&smt.Spec, validateTaskSpecOptions, field.NewPath("spec"))...)

	return allErrs
}

func makeValidateScyllaDBManagerTaskObjectMetaOptions(smt *scyllav1alpha1.ScyllaDBManagerTask) *ValidateScyllaDBManagerTaskObjectMetaOptions {
	isScheduleCronNil := (smt.Spec.Backup == nil || smt.Spec.Backup.Cron == nil) && (smt.Spec.Repair == nil || smt.Spec.Repair.Cron == nil)
	isRepairIntensityNil := smt.Spec.Repair == nil || smt.Spec.Repair.Intensity == nil
	isRepairSmallTableThresholdNil := smt.Spec.Repair == nil || smt.Spec.Repair.SmallTableThreshold == nil

	return &ValidateScyllaDBManagerTaskObjectMetaOptions{
		ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions: ValidateScyllaDBManagerTaskObjectMetaAnnotationsOptions{
			IsScyllaDBManagerTaskScheduleCronNil:              isScheduleCronNil,
			IsScyllaDBManagerTaskRepairIntensityNil:           isRepairIntensityNil,
			IsScyllaDBManagerTaskRepairSmallTableThresholdNil: isRepairSmallTableThresholdNil,
		},
	}
}

func makeValidateScyllaDBManagerTaskSpecOptions(smt *scyllav1alpha1.ScyllaDBManagerTask) *ValidateScyllaDBManagerTaskSpecOptions {
	isBackupLocationValidationDisabled := smt.Annotations[naming.ScyllaDBManagerTaskBackupLocationDisableValidationAnnotation] == naming.AnnotationValueTrue

	return &ValidateScyllaDBManagerTaskSpecOptions{
		ValidateScyllaDBManagerBackupTaskOptionsOptions: ValidateScyllaDBManagerBackupTaskOptionsOptions{
			IsLocationValidationDisabled: isBackupLocationValidationDisabled,
		},
	}
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
	// Due to backwards compatibility guarantees with v1.ScyllaCluster we can only validate the interval override annotation when cron is set.
	if hasIntervalOverrideAnnotation && !options.IsScyllaDBManagerTaskScheduleCronNil {
		intervalDuration, err := duration.ParseDuration(intervalOverrideAnnotation)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation), intervalOverrideAnnotation, "valid units are d, h, m, s"))
		} else if intervalDuration != 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation), "can't be non-zero when cron is specified"))
		}
	}

	timezoneOverrideAnnotation, hasTimezoneOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation]
	if hasTimezoneOverrideAnnotation {
		_, err := time.LoadLocation(timezoneOverrideAnnotation)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation), timezoneOverrideAnnotation, err.Error()))
		}

		if options.IsScyllaDBManagerTaskScheduleCronNil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation), "can't be set when cron is not specified"))
		}
	}

	repairIntensityOverrideAnnotation, hasRepairIntensityOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation]
	if hasRepairIntensityOverrideAnnotation {
		_, err := strconv.ParseFloat(repairIntensityOverrideAnnotation, 64)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation), repairIntensityOverrideAnnotation, "must be a float"))
		}

		if !options.IsScyllaDBManagerTaskRepairIntensityNil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation), "can't be used together with repair options' intensity"))
		}
	}

	_, hasSmallTableThresholdOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation]
	if hasSmallTableThresholdOverrideAnnotation && !options.IsScyllaDBManagerTaskRepairSmallTableThresholdNil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation), "can't be used together with repair options' smallTableThreshold"))
	}

	return allErrs
}

func ValidateScyllaDBManagerTaskSpec(spec *scyllav1alpha1.ScyllaDBManagerTaskSpec, options *ValidateScyllaDBManagerTaskSpecOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateLocalScyllaDBReference(&spec.ScyllaDBClusterRef, fldPath.Child("scyllaDBClusterRef"))...)

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

		allErrs = append(allErrs, ValidateScyllaDBManagerRepairTaskOptions(spec.Repair, fldPath.Child("repair"))...)

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

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSchedule(&backupOptions.ScyllaDBManagerTaskSchedule, fldPath)...)

	if !options.IsLocationValidationDisabled {
		if backupOptions.Location == nil || len(backupOptions.Location) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("location"), "location must not be empty"))
		} else {
			for i := range backupOptions.Location {
				if len(backupOptions.Location[i]) == 0 {
					allErrs = append(allErrs, field.Required(fldPath.Child("location").Index(i), "location must not be empty"))
				}
			}
		}
	}

	return allErrs
}

func ValidateScyllaDBManagerRepairTaskOptions(repairOptions *scyllav1alpha1.ScyllaDBManagerRepairTaskOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSchedule(&repairOptions.ScyllaDBManagerTaskSchedule, fldPath)...)

	if repairOptions.Intensity != nil && *repairOptions.Intensity < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("intensity"), *repairOptions.Intensity, "can't be negative"))
	}

	if repairOptions.SmallTableThreshold != nil {
		allErrs = append(allErrs, corevalidation.ValidatePositiveQuantityValue(*repairOptions.SmallTableThreshold, fldPath.Child("smallTableThreshold"))...)

		if repairOptions.SmallTableThreshold.MilliValue()%int64(1000) != int64(0) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("smallTableThreshold"), repairOptions.SmallTableThreshold.String(), "fractional byte value is invalid, must be an integer"))
		}
	}

	return allErrs
}

func ValidateScyllaDBManagerTaskSchedule(schedule *scyllav1alpha1.ScyllaDBManagerTaskSchedule, fldPath *field.Path) field.ErrorList {
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
	allErrs = append(allErrs, ValidateScyllaDBManagerTaskObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateScyllaDBManagerTaskSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBManagerTaskObjectMetaUpdate(newObjectMeta, oldObjectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateScyllaDBManagerTaskObjectMetaAnnotationsUpdate(newObjectMeta.Annotations, oldObjectMeta.Annotations, fldPath.Child("annotations"))...)

	return allErrs
}

func ValidateScyllaDBManagerTaskObjectMetaAnnotationsUpdate(newAnnotations, oldAnnotations map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newAnnotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation], oldAnnotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation], fldPath.Key(naming.ScyllaDBManagerTaskNameOverrideAnnotation))...)

	return allErrs
}

func ValidateScyllaDBManagerTaskSpecUpdate(newSpec, oldSpec *scyllav1alpha1.ScyllaDBManagerTaskSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)

	return allErrs
}
