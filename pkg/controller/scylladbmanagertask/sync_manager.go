// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (smtc *Controller) syncManager(
	ctx context.Context,
	smt *scyllav1alpha1.ScyllaDBManagerTask,
	status *scyllav1alpha1.ScyllaDBManagerTaskStatus,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	smcr, err := smtc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(smt.Namespace).Get(smcrName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return progressingConditions, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration: %w", err)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smt.Generation,
			Reason:             "AwaitingScyllaDBManagerClusterRegistrationCreation",
			Message:            fmt.Sprintf("Awaiting creation of ScyllaDBManagerClusterRegistration: %q.", naming.ManualRef(smt.Namespace, smcrName)),
		})

		return progressingConditions, nil
	}

	if smcr.Status.ClusterID == nil || len(*smcr.Status.ClusterID) == 0 {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smt.Generation,
			Reason:             "AwaitingScyllaDBManagerClusterRegistrationClusterIDPropagation",
			Message:            fmt.Sprintf("Awaiting the ScyllaDB Manager's cluster ID to be propagated to the status of ScyllaDBManagerClusterRegistration: %q.", naming.ManualRef(smt.Namespace, smcrName)),
		})

		return progressingConditions, nil
	}

	clusterID := *smcr.Status.ClusterID

	managerClient, err := controllerhelpers.GetScyllaDBManagerClient(ctx, smcr)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get manager client: %w", err)
	}

	managerTask, found, err := getScyllaDBManagerClientTask(ctx, smt, clusterID, managerClient)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager task: %w", err)
	}

	if !found {
		var requiredManagerTask *managerclient.Task
		requiredManagerTask, err = makeScyllaDBManagerClientTask(smt, clusterID)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't make required ScyllaDB Manager task: %w", err)
		}

		klog.V(2).InfoS("Creating ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientTaskName", requiredManagerTask.Name)

		var managerTaskID uuid.UUID
		managerTaskID, err = managerClient.CreateTask(ctx, clusterID, requiredManagerTask)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't create ScyllaDB Manager task: %w", err)
		}

		status.TaskID = pointer.Ptr(managerTaskID.String())
		return progressingConditions, nil
	}

	var managerClientTaskOverrideOptions []ScyllaDBManagerClientTaskOverrideOption
	if managerTask.Schedule != nil && managerTask.Schedule.StartDate != nil {
		managerClientTaskOverrideOptions = append(managerClientTaskOverrideOptions, WithScheduleStartDateNowSyntaxRetention(*managerTask.Schedule.StartDate))
	}

	requiredManagerTask, err := makeScyllaDBManagerClientTask(smt, clusterID, managerClientTaskOverrideOptions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required ScyllaDB Manager task: %w", err)
	}

	var managerTaskID uuid.UUID
	managerTaskID, err = uuid.Parse(managerTask.ID)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't parse ScyllaDB Manager client task ID: %w", err)
	}

	ownerUIDLabelValue, hasOwnerUIDLabel := managerTask.Labels[naming.OwnerUIDLabel]
	if !hasOwnerUIDLabel {
		klog.Warningf("ScyllaDB Manager client task %q is missing the owner UID label. Deleting it to avoid a name collision.", managerTask.Name)

		err = managerClient.DeleteTask(ctx, clusterID, managerTask.Type, managerTaskID)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager task %q: %w", managerTask.Name, err)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "DeletedCollidingScyllaDBManagerTask",
			Message:            "Deleted a colliding ScyllaDB Manager task with no OwnerUID label.",
		})
		return progressingConditions, nil
	}

	if ownerUIDLabelValue == string(smt.UID) && requiredManagerTask.Labels[naming.ManagedHash] == managerTask.Labels[naming.ManagedHash] {
		// Cluster matches the desired state, nothing to do.
		return progressingConditions, nil
	}

	if ownerUIDLabelValue != string(smt.UID) {
		// Ideally, we wouldn't do anything here as this is error-prone and might hinder discovering bugs.
		// However, the task could have been created by the legacy component (manager-controller), so we update it to become a new owner without disrupting the state.
		klog.Warningf("Task %q already exists in ScyllaDB Manager state and has an owner UID label (%q), but it has a different owner. ScyllaDBManagerTask %q will adopt it.", managerTask.Name, ownerUIDLabelValue, klog.KObj(smt))
	}

	requiredManagerTask.ID = managerTask.ID

	klog.V(2).InfoS("Updating ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientTaskName", requiredManagerTask.Name, "ScyllaDBManagerClientTaskID", requiredManagerTask.ID)
	err = managerClient.UpdateTask(ctx, clusterID, requiredManagerTask)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't update ScyllaDB Manager task %q: %w", requiredManagerTask.Name, err)
	}

	// TODO: status

	return progressingConditions, nil
}

func getScyllaDBManagerClientTask(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask, clusterID string, managerClient *managerclient.Client) (*managerclient.TaskListItem, bool, error) {
	taskName := scyllaDBManagerClientTaskName(smt)

	taskType, err := scyllaDBManagerClientTaskType(smt)
	if err != nil {
		return nil, false, fmt.Errorf("can't get ScyllaDB Manager client task type: %w", err)
	}

	var taskID string
	if smt.Status.TaskID != nil {
		taskID = *smt.Status.TaskID
	}

	tasks, err := managerClient.ListTasks(ctx, clusterID, taskType, true, "", taskID)
	if err != nil {
		return nil, false, fmt.Errorf("can't list ScyllaDB Manager client tasks: %w", err)
	}

	if len(tasks.TaskListItemSlice) == 0 {
		return nil, false, nil
	}

	if len(taskID) > 0 && len(tasks.TaskListItemSlice) > 1 {
		return nil, false, fmt.Errorf("more than one task found in ScyllaDB Manager state with taskID: %s", taskID)
	}

	idx := slices.IndexFunc(tasks.TaskListItemSlice, func(item *managerclient.TaskListItem) bool {
		return item.Name == taskName
	})

	if idx >= 0 {
		return tasks.TaskListItemSlice[idx], true, nil
	}

	return nil, false, nil
}

type ScyllaDBManagerClientTaskOverrideOption func(*scyllav1alpha1.ScyllaDBManagerTask, *managerclient.Task)

// TODO: test
// TODO: comment
func WithScheduleStartDateNowSyntaxRetention(existingStartDate strfmt.DateTime) func(*scyllav1alpha1.ScyllaDBManagerTask, *managerclient.Task) {
	return func(smt *scyllav1alpha1.ScyllaDBManagerTask, managerTask *managerclient.Task) {
		startDateOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation]
		if !strings.HasPrefix(startDateOverrideAnnotation, "now") {
			return
		}

		if managerTask.Schedule == nil {
			managerTask.Schedule = &managerclient.Schedule{}
		}

		managerTask.Schedule.StartDate = pointer.Ptr(existingStartDate)
	}
}

func makeScyllaDBManagerClientTask(smt *scyllav1alpha1.ScyllaDBManagerTask, clusterID string, overrideOptions ...ScyllaDBManagerClientTaskOverrideOption) (*managerclient.Task, error) {
	var err error
	var managerClientTaskType string

	managerClientTaskName := scyllaDBManagerClientTaskName(smt)
	managerClientTaskSchedule := &managerclient.Schedule{}
	managerClientTaskProperties := map[string]any{}

	var scheduleOverrideOptions []ScyllaDBManagerClientScheduleOverrideOption

	intervalOverrideAnnotation, hasIntervalOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation]
	if hasIntervalOverrideAnnotation {
		scheduleOverrideOptions = append(scheduleOverrideOptions, withIntervalOverride(intervalOverrideAnnotation))
	}

	startDateOverrideAnnotation, hasStartDateOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation]
	if hasStartDateOverrideAnnotation {
		scheduleOverrideOptions = append(scheduleOverrideOptions, withStartDateOverride(startDateOverrideAnnotation))
	}

	// TODO: validate timezone annotation value?
	timezoneOverrideAnnotation, hasTimezoneOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation]
	if hasTimezoneOverrideAnnotation {
		scheduleOverrideOptions = append(scheduleOverrideOptions, withTimezoneOverride(timezoneOverrideAnnotation))
	}

	switch smt.Spec.Type {
	case scyllav1alpha1.ScyllaDBManagerTaskTypeBackup:
		managerClientTaskType = managerclient.BackupTask

		managerClientTaskSchedule, err = makeScyllaDBManagerClientSchedule(&smt.Spec.Backup.ScyllaDBManagerTaskSchedule, scheduleOverrideOptions...)
		if err != nil {
			return nil, fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", err)
		}

		managerClientTaskProperties, err = makeScyllaDBManagerClientBackupTaskProperties(smt.Spec.Backup)
		if err != nil {
			return nil, fmt.Errorf("can't make ScyllaDB Manager client backup task properties: %w", err)
		}

	case scyllav1alpha1.ScyllaDBManagerTaskTypeRepair:
		managerClientTaskType = managerclient.RepairTask

		managerClientTaskSchedule, err = makeScyllaDBManagerClientSchedule(&smt.Spec.Repair.ScyllaDBManagerTaskSchedule, scheduleOverrideOptions...)
		if err != nil {
			return nil, fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", err)
		}

		var repairTaskOverrideOptions []ScyllaDBManagerClientPropertiesOverrideOption
		intensityOverrideAnnotation, hasIntensityOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation]
		if hasIntensityOverrideAnnotation {
			repairTaskOverrideOptions = append(repairTaskOverrideOptions, withIntensityOverride(intensityOverrideAnnotation))
		}

		managerClientTaskProperties, err = makeScyllaDBManagerClientRepairTaskProperties(smt.Spec.Repair, repairTaskOverrideOptions...)
		if err != nil {
			return nil, fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported ScyllaDBManagerTaskType: %q", smt.Spec.Type)

	}

	requiredManagerTask := &managerclient.Task{
		ClusterID: clusterID,
		Enabled:   true,
		Labels: map[string]string{
			naming.OwnerUIDLabel: string(smt.UID),
		},
		Name:       managerClientTaskName,
		Properties: managerClientTaskProperties,
		Schedule:   managerClientTaskSchedule,
		Type:       managerClientTaskType,
	}

	for _, optionOverrideFunc := range overrideOptions {
		optionOverrideFunc(smt, requiredManagerTask)
	}

	managedHash, err := hashutil.HashObjects(requiredManagerTask)
	if err != nil {
		return nil, fmt.Errorf("can't calculate managed hash: %w", err)
	}
	requiredManagerTask.Labels[naming.ManagedHash] = managedHash

	return requiredManagerTask, nil
}

type ScyllaDBManagerClientScheduleOverrideOption func(*managerclient.Schedule) error

func withIntervalOverride(interval string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		s.Interval = interval
		return nil
	}
}

func withStartDateOverride(startDate string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		parsed, err := parseStartDate(startDate)
		if err != nil {
			return fmt.Errorf("can't parse start date: %w", err)
		}

		s.StartDate = &parsed
		return nil
	}
}

func parseStartDate(value string) (strfmt.DateTime, error) {
	if strings.HasPrefix(value, "now") {
		now := timeutc.Now()
		if value == "now" {
			return strfmt.DateTime{}, nil
		}

		d, err := duration.ParseDuration(value[3:])
		if err != nil {
			return strfmt.DateTime{}, err
		}
		if d == 0 {
			return strfmt.DateTime{}, nil
		}

		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	// No more heuristics, assume the user passed a date formatted string
	t, err := timeutc.Parse(time.RFC3339, value)
	if err != nil {
		return strfmt.DateTime{}, err
	}

	return strfmt.DateTime(t), nil
}

func withTimezoneOverride(timezone string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		s.Timezone = timezone
		return nil
	}
}

func makeScyllaDBManagerClientSchedule(scyllaDBManagerTaskSchedule *scyllav1alpha1.ScyllaDBManagerTaskSchedule, overrideOptions ...ScyllaDBManagerClientScheduleOverrideOption) (*managerclient.Schedule, error) {
	managerClientSchedule := &managerclient.Schedule{}

	if scyllaDBManagerTaskSchedule.Cron != nil {
		managerClientSchedule.Cron = *scyllaDBManagerTaskSchedule.Cron
	}

	if scyllaDBManagerTaskSchedule.StartDate != nil {
		managerClientSchedule.StartDate = pointer.Ptr(strfmt.DateTime(scyllaDBManagerTaskSchedule.StartDate.Time))
	}

	if scyllaDBManagerTaskSchedule.NumRetries != nil {
		managerClientSchedule.NumRetries = *scyllaDBManagerTaskSchedule.NumRetries
	}

	var errs []error
	for _, optionOverrideFunc := range overrideOptions {
		err := optionOverrideFunc(managerClientSchedule)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return managerClientSchedule, nil
}

func makeScyllaDBManagerClientBackupTaskProperties(options *scyllav1alpha1.ScyllaDBManagerBackupTaskOptions) (map[string]any, error) {
	managerClientTaskProperties := map[string]any{
		"location": options.Location,
	}

	if options.DC != nil {
		managerClientTaskProperties["dc"] = unescapeFilters(options.DC)
	}

	if options.Keyspace != nil {
		managerClientTaskProperties["keyspace"] = unescapeFilters(options.Keyspace)
	}

	if options.RateLimit != nil {
		managerClientTaskProperties["rate_limit"] = options.RateLimit
	}

	if options.Retention != nil {
		managerClientTaskProperties["retention"] = options.Retention
	}

	if options.SnapshotParallel != nil {
		managerClientTaskProperties["snapshot_parallel"] = options.SnapshotParallel
	}

	if options.UploadParallel != nil {
		managerClientTaskProperties["upload_parallel"] = options.UploadParallel
	}

	return managerClientTaskProperties, nil
}

type ScyllaDBManagerClientPropertiesOverrideOption func(map[string]any) error

func withIntensityOverride(intensity string) func(map[string]any) error {
	return func(properties map[string]any) error {
		parsed, err := strconv.ParseFloat(intensity, 64)
		if err != nil {
			return fmt.Errorf("can't parse intensity: %w", err)
		}

		properties["intensity"] = parsed

		return nil
	}
}

func makeScyllaDBManagerClientRepairTaskProperties(options *scyllav1alpha1.ScyllaDBManagerRepairTaskOptions, overrideOptions ...ScyllaDBManagerClientPropertiesOverrideOption) (map[string]any, error) {
	managerClientTaskProperties := map[string]any{}

	if options.DC != nil {
		managerClientTaskProperties["dc"] = unescapeFilters(options.DC)
	}

	if options.Keyspace != nil {
		managerClientTaskProperties["keyspace"] = unescapeFilters(options.Keyspace)
	}

	if options.FailFast != nil {
		managerClientTaskProperties["fail_fast"] = *options.FailFast
	}

	if options.Host != nil {
		managerClientTaskProperties["host"] = *options.Host
	}

	if options.Intensity != nil {
		managerClientTaskProperties["intensity"] = *options.Intensity
	}

	if options.Parallel != nil {
		managerClientTaskProperties["parallel"] = *options.Parallel
	}

	if options.SmallTableThreshold != nil {
		// TODO: make sure this is correct
		// TODO: does this need an override?
		managerClientTaskProperties["small_table_threshold"] = options.SmallTableThreshold.Value()
	}

	var errs []error
	for _, optionOverrideFunc := range overrideOptions {
		err := optionOverrideFunc(managerClientTaskProperties)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return managerClientTaskProperties, nil
}

// unescapeFilters handles escaping bash expansions.
// '\' can be removed safely as it's not a valid character in the keyspace or table names.
func unescapeFilters(strs []string) []string {
	for i := range strs {
		strs[i] = strings.ReplaceAll(strs[i], "\\", "")
	}

	return strs
}

func scyllaDBManagerClientTaskType(smt *scyllav1alpha1.ScyllaDBManagerTask) (string, error) {
	switch smt.Spec.Type {
	case scyllav1alpha1.ScyllaDBManagerTaskTypeBackup:
		return managerclient.BackupTask, nil

	case scyllav1alpha1.ScyllaDBManagerTaskTypeRepair:
		return managerclient.RepairTask, nil

	default:
		return "", fmt.Errorf("unsupported ScyllaDBManagerTask type: %q", smt.Spec.Type)

	}
}

// TODO: validate name annotation
func scyllaDBManagerClientTaskName(smt *scyllav1alpha1.ScyllaDBManagerTask) string {
	nameOverrideAnnotationValue, hasNameOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		return nameOverrideAnnotationValue
	}

	return smt.Name
}
