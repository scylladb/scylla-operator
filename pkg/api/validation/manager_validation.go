package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateScyllaManager(sm *v1alpha1.ScyllaManager) field.ErrorList {
	return ValidateScyllaManagerSpec(&sm.Spec, field.NewPath("spec"))
}

func ValidateScyllaManagerSpec(spec *v1alpha1.ScyllaManagerSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateRepairsTasksSpecs(spec.Repairs, fldPath.Child("repairs"))...)
	allErrs = append(allErrs, ValidateBackupsTasksSpecs(spec.Backups, fldPath.Child("backups"))...)

	return allErrs
}

var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func ValidateRepairsTasksSpecs(specs []v1alpha1.RepairTaskSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	taskNames := strset.New()
	for i, spec := range specs {
		if taskNames.Has(spec.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), spec.Name))
		}
		taskNames.Add(spec.Name)

		if spec.Cron != "" {
			if _, err := cronParser.Parse(spec.Cron); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("cron"), spec.Cron, "failed cron parse"))
			}
		}

		if spec.TimeWindow != "" {
			allErrs = append(allErrs, validateTimeWindow(spec.TimeWindow, fldPath.Index(i).Child("TimeWindow"))...)
		}

		_, err := strconv.ParseFloat(spec.Intensity, 64)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("intensity"), spec.Intensity, "invalid intensity, it must be a float value"))
		}
	}

	return allErrs
}

func ValidateBackupsTasksSpecs(specs []v1alpha1.BackupTaskSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	taskNames := strset.New()
	for i, spec := range specs {
		if taskNames.Has(spec.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), spec.Name))
		}
		taskNames.Add(spec.Name)

		if spec.Cron != "" {
			if _, err := cronParser.Parse(spec.Cron); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("cron"), spec.Cron, "failed cron parse"))
			}
		}

		if spec.TimeWindow != "" {
			allErrs = append(allErrs, validateTimeWindow(spec.TimeWindow, fldPath.Index(i).Child("TimeWindow"))...)
		}
	}

	return allErrs
}

func validateTimeWindow(timeWindow string, fldPath *field.Path) field.ErrorList {
	var (
		weekdayTimeRegexp = regexp.MustCompile("(?i)^((Mon|Tue|Wed|Thu|Fri|Sat|Sun)-)?([0-9]{1,2}):([0-9]{2})$")
		weekdayRev        = map[string]time.Weekday{
			"":    time.Weekday(7), // Match any day.
			"mon": time.Monday,
			"tue": time.Tuesday,
			"wed": time.Wednesday,
			"thu": time.Thursday,
			"fri": time.Friday,
			"sat": time.Saturday,
			"sun": time.Sunday,
		}
	)
	allErrs := field.ErrorList{}
	slots := strings.Split(timeWindow, ",")
	if len(slots)%2 != 0 {
		return field.ErrorList{field.Invalid(fldPath, slots, "The number of slots have to be even.")}
	}
	for i, slot := range slots {
		text := []byte(slot)

		m := weekdayTimeRegexp.FindSubmatch(text)
		if len(m) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("slot").Index(i), string(text), "invalid format of a time window"))
			continue
		}

		_, ok := weekdayRev[strings.ToLower(string(m[2]))]
		if !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("slot").Index(i), string(m[2]), fmt.Sprintf("unknown day of week %q", string(m[2]))))
		}

		hh, _ := strconv.Atoi(string(m[3])) // nolint: errcheck // Validated by regex
		if hh >= 24 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("slot").Index(i), string(m[3]), "invalid hour"))
		}
		mm, _ := strconv.Atoi(string(m[4])) // nolint: errcheck // Validated by regex
		if mm >= 60 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("slot").Index(i), string(m[4]), "invalid minute"))
		}
	}

	return allErrs
}

func ValidateScyllaManagerUpdate(new, old *v1alpha1.ScyllaManager) field.ErrorList {
	allErrs := ValidateScyllaManager(new)

	return append(allErrs, ValidateScyllaManagerSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)
}

func ValidateScyllaManagerSpecUpdate(new, old *v1alpha1.ScyllaManagerSpec, fldPath *field.Path) field.ErrorList {
	return nil
}
