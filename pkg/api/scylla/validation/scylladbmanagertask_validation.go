// Copyright (C) 2025 ScyllaDB

package validation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
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

type validateScyllaDBManagerTaskObjectMetaFlags struct {
	validateScyllaDBManagerTaskObjectMetaAnnotationsFlags validateScyllaDBManagerTaskObjectMetaAnnotationsFlags
}

type validateScyllaDBManagerTaskObjectMetaAnnotationsFlags struct {
	isScyllaDBManagerTaskScheduleCronNil              bool
	isScyllaDBManagerTaskRepairIntensityNil           bool
	isScyllaDBManagerTaskRepairSmallTableThresholdNil bool
}

type validateScyllaDBManagerTaskSpecFlags struct {
	validateScyllaDBManagerBackupTaskOptionsFlags validateScyllaDBManagerBackupTaskOptionsFlags
	validateScyllaDBManagerRepairTaskOptionsFlags validateScyllaDBManagerRepairTaskOptionsFlags
}

type validateScyllaDBManagerBackupTaskOptionsFlags struct {
	isLocationValidationDisabled  bool
	isRetentionValidationDisabled bool
}

type validateScyllaDBManagerRepairTaskOptionsFlags struct {
	isParallelValidationDisabled bool
}

func ValidateScyllaDBManagerTask(smt *scyllav1alpha1.ScyllaDBManagerTask) field.ErrorList {
	allErrs := field.ErrorList{}

	validateObjectMetaFlags := makeValidateScyllaDBManagerTaskObjectMetaFlags(smt)
	allErrs = append(allErrs, validateScyllaDBManagerTaskObjectMeta(&smt.ObjectMeta, validateObjectMetaFlags, field.NewPath("metadata"))...)

	validateSpecFlags := makeValidateScyllaDBManagerTaskSpecFlags(smt)
	allErrs = append(allErrs, validateScyllaDBManagerTaskSpec(&smt.Spec, validateSpecFlags, field.NewPath("spec"))...)

	return allErrs
}

func makeValidateScyllaDBManagerTaskObjectMetaFlags(smt *scyllav1alpha1.ScyllaDBManagerTask) *validateScyllaDBManagerTaskObjectMetaFlags {
	isScheduleCronNil := (smt.Spec.Backup == nil || smt.Spec.Backup.Cron == nil) && (smt.Spec.Repair == nil || smt.Spec.Repair.Cron == nil)
	isRepairIntensityNil := smt.Spec.Repair == nil || smt.Spec.Repair.Intensity == nil
	isRepairSmallTableThresholdNil := smt.Spec.Repair == nil || smt.Spec.Repair.SmallTableThreshold == nil

	return &validateScyllaDBManagerTaskObjectMetaFlags{
		validateScyllaDBManagerTaskObjectMetaAnnotationsFlags: validateScyllaDBManagerTaskObjectMetaAnnotationsFlags{
			isScyllaDBManagerTaskScheduleCronNil:              isScheduleCronNil,
			isScyllaDBManagerTaskRepairIntensityNil:           isRepairIntensityNil,
			isScyllaDBManagerTaskRepairSmallTableThresholdNil: isRepairSmallTableThresholdNil,
		},
	}
}

func makeValidateScyllaDBManagerTaskSpecFlags(smt *scyllav1alpha1.ScyllaDBManagerTask) *validateScyllaDBManagerTaskSpecFlags {
	isBackupLocationValidationDisabled := smt.Annotations[naming.ScyllaDBManagerTaskBackupLocationDisableValidationAnnotation] == naming.AnnotationValueTrue
	isBackupRetentionValidationDisabled := smt.Annotations[naming.ScyllaDBManagerTaskBackupRetentionDisableValidationAnnotation] == naming.AnnotationValueTrue

	isRepairParallelValidationDisabled := smt.Annotations[naming.ScyllaDBManagerTaskRepairParallelDisableValidationAnnotation] == naming.AnnotationValueTrue

	return &validateScyllaDBManagerTaskSpecFlags{
		validateScyllaDBManagerBackupTaskOptionsFlags: validateScyllaDBManagerBackupTaskOptionsFlags{
			isLocationValidationDisabled:  isBackupLocationValidationDisabled,
			isRetentionValidationDisabled: isBackupRetentionValidationDisabled,
		},
		validateScyllaDBManagerRepairTaskOptionsFlags: validateScyllaDBManagerRepairTaskOptionsFlags{
			isParallelValidationDisabled: isRepairParallelValidationDisabled,
		},
	}
}

func validateScyllaDBManagerTaskObjectMeta(meta *metav1.ObjectMeta, flags *validateScyllaDBManagerTaskObjectMetaFlags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateScyllaDBManagerTaskObjectMetaAnnotations(meta.Annotations, &flags.validateScyllaDBManagerTaskObjectMetaAnnotationsFlags, fldPath.Child("annotations"))...)

	return allErrs
}

func validateScyllaDBManagerTaskObjectMetaAnnotations(annotations map[string]string, flags *validateScyllaDBManagerTaskObjectMetaAnnotationsFlags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	nameOverrideAnnotation, hasNameOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(nameOverrideAnnotation, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskNameOverrideAnnotation), nameOverrideAnnotation, msg))
		}
	}

	intervalOverrideAnnotation, hasIntervalOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation]
	// Due to backwards compatibility guarantees with v1.ScyllaCluster we can only validate the interval override annotation when cron is set.
	if hasIntervalOverrideAnnotation && !flags.isScyllaDBManagerTaskScheduleCronNil {
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

		if flags.isScyllaDBManagerTaskScheduleCronNil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation), "can't be set when cron is not specified"))
		}
	}

	repairIntensityOverrideAnnotation, hasRepairIntensityOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation]
	if hasRepairIntensityOverrideAnnotation {
		_, err := strconv.ParseFloat(repairIntensityOverrideAnnotation, 64)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation), repairIntensityOverrideAnnotation, "must be a float"))
		}

		if !flags.isScyllaDBManagerTaskRepairIntensityNil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation), "can't be used together with repair options' intensity"))
		}
	}

	_, hasSmallTableThresholdOverrideAnnotation := annotations[naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation]
	if hasSmallTableThresholdOverrideAnnotation && !flags.isScyllaDBManagerTaskRepairSmallTableThresholdNil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Key(naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation), "can't be used together with repair options' smallTableThreshold"))
	}

	return allErrs
}

func validateScyllaDBManagerTaskSpec(spec *scyllav1alpha1.ScyllaDBManagerTaskSpec, flags *validateScyllaDBManagerTaskSpecFlags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateLocalScyllaDBReference(&spec.ScyllaDBClusterRef, fldPath.Child("scyllaDBClusterRef"))...)

	switch spec.Type {
	case scyllav1alpha1.ScyllaDBManagerTaskTypeBackup:
		if spec.Backup == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("backup"), fmt.Sprintf("backup options are required when task type is %q", scyllav1alpha1.ScyllaDBManagerTaskTypeBackup)))
			break
		}

		allErrs = append(allErrs, validateScyllaDBManagerBackupTaskOptions(spec.Backup, &flags.validateScyllaDBManagerBackupTaskOptionsFlags, fldPath.Child("backup"))...)

	case scyllav1alpha1.ScyllaDBManagerTaskTypeRepair:
		if spec.Repair == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("repair"), fmt.Sprintf("repair options are required when task type is %q", scyllav1alpha1.ScyllaDBManagerTaskTypeRepair)))
			break
		}

		allErrs = append(allErrs, validateScyllaDBManagerRepairTaskOptions(spec.Repair, &flags.validateScyllaDBManagerRepairTaskOptionsFlags, fldPath.Child("repair"))...)

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), spec.Type, oslices.ConvertSlice(supportedScyllaDBManagerTaskTypes, oslices.ToString)))

	}

	if spec.Type != scyllav1alpha1.ScyllaDBManagerTaskTypeBackup && spec.Backup != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("backup"), fmt.Sprintf("backup options are forbidden when task type is not %q", scyllav1alpha1.ScyllaDBManagerTaskTypeBackup)))
	}

	if spec.Type != scyllav1alpha1.ScyllaDBManagerTaskTypeRepair && spec.Repair != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("repair"), fmt.Sprintf("repair options are forbidden when task type is not %q", scyllav1alpha1.ScyllaDBManagerTaskTypeRepair)))
	}

	return allErrs
}

func validateScyllaDBManagerBackupTaskOptions(backupOptions *scyllav1alpha1.ScyllaDBManagerBackupTaskOptions, flags *validateScyllaDBManagerBackupTaskOptionsFlags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateScyllaDBManagerTaskSchedule(&backupOptions.ScyllaDBManagerTaskSchedule, fldPath)...)

	if !flags.isLocationValidationDisabled {
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

	if !flags.isRetentionValidationDisabled && backupOptions.Retention != nil && *backupOptions.Retention < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retention"), *backupOptions.Retention, "can't be negative"))
	}

	return allErrs
}

func validateScyllaDBManagerRepairTaskOptions(repairOptions *scyllav1alpha1.ScyllaDBManagerRepairTaskOptions, flags *validateScyllaDBManagerRepairTaskOptionsFlags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateScyllaDBManagerTaskSchedule(&repairOptions.ScyllaDBManagerTaskSchedule, fldPath)...)

	if repairOptions.Intensity != nil && *repairOptions.Intensity < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("intensity"), *repairOptions.Intensity, "can't be negative"))
	}

	if !flags.isParallelValidationDisabled && repairOptions.Parallel != nil && *repairOptions.Parallel < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("parallel"), *repairOptions.Parallel, "can't be negative"))
	}

	if repairOptions.SmallTableThreshold != nil {
		allErrs = append(allErrs, corevalidation.ValidatePositiveQuantityValue(*repairOptions.SmallTableThreshold, fldPath.Child("smallTableThreshold"))...)

		if repairOptions.SmallTableThreshold.MilliValue()%int64(1000) != int64(0) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("smallTableThreshold"), repairOptions.SmallTableThreshold.String(), "fractional byte value is invalid, must be an integer"))
		}
	}

	return allErrs
}

func validateScyllaDBManagerTaskSchedule(schedule *scyllav1alpha1.ScyllaDBManagerTaskSchedule, fldPath *field.Path) field.ErrorList {
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
	allErrs = append(allErrs, validateScyllaDBManagerTaskObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateScyllaDBManagerTaskSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateScyllaDBManagerTaskObjectMetaUpdate(newObjectMeta, oldObjectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateScyllaDBManagerTaskObjectMetaAnnotationsUpdate(newObjectMeta.Annotations, oldObjectMeta.Annotations, fldPath.Child("annotations"))...)

	return allErrs
}

func validateScyllaDBManagerTaskObjectMetaAnnotationsUpdate(newAnnotations, oldAnnotations map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newAnnotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation], oldAnnotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation], fldPath.Key(naming.ScyllaDBManagerTaskNameOverrideAnnotation))...)

	return allErrs
}

func validateScyllaDBManagerTaskSpecUpdate(newSpec, oldSpec *scyllav1alpha1.ScyllaDBManagerTaskSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)

	return allErrs
}
