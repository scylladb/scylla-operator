// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
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
		var notFoundProgressingConditions []metav1.Condition
		notFoundProgressingConditions, err = smtc.syncManagerClientTaskNotFound(ctx, smt, status, managerClient, clusterID)
		progressingConditions = append(progressingConditions, notFoundProgressingConditions...)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't sync ScyllaDB Manager client task not found: %w", err)
		}

		return progressingConditions, nil
	}

	var managerClientTaskOverrideOptions []scyllaDBManagerClientTaskOverrideOption
	if managerTask.Schedule != nil && managerTask.Schedule.StartDate != nil {
		managerClientTaskOverrideOptions = append(managerClientTaskOverrideOptions, withScheduleStartDateNowSyntaxRetention(*managerTask.Schedule.StartDate))
	}

	requiredManagerTask, err := makeScyllaDBManagerClientTask(smt, clusterID, managerClientTaskOverrideOptions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required ScyllaDB Manager task: %w", err)
	}

	ownerUIDLabelValue, hasOwnerUIDLabel := managerTask.Labels[naming.OwnerUIDLabel]
	_, hasMissingOwnerUIDForceAdoptAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskMissingOwnerUIDForceAdoptAnnotation]
	if !hasOwnerUIDLabel && hasMissingOwnerUIDForceAdoptAnnotation {
		// The task could have been created by the legacy component (manager-controller), in which case it does not have the owner UID label.
		// For backward compatibility, we adopt it instead of recreating it if the internal annotation forcing adoption is set.
		klog.Warningf("Task %q (%q) already exists in ScyllaDB Manager state with no owner UID label. ScyllaDBManagerTask %q will adopt it as it has the annotation forcing its adoption: %q.", managerTask.Name, managerTask.ID, klog.KObj(smt), naming.ScyllaDBManagerTaskMissingOwnerUIDForceAdoptAnnotation)
	} else if !hasOwnerUIDLabel {
		var missingOwnerUIDProgressingConditions []metav1.Condition
		missingOwnerUIDProgressingConditions, err = smtc.syncManagerClientTaskMissingOwnerUID(ctx, smt, managerClient, clusterID, managerTask)
		progressingConditions = append(progressingConditions, missingOwnerUIDProgressingConditions...)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't sync ScyllaDB Manager client task not found: %w", err)
		}

		return progressingConditions, nil
	} else if ownerUIDLabelValue != string(smt.UID) {
		// The task already exists in ScyllaDB Manager state, but it has a different owner.
		// This can happen if the task was created by a different ScyllaDBManagerTask instance and a name collision occurred.
		// Adopting the task could hinder bugs, so we return an error instead.
		klog.Warningf("Task %q (%q) already exists in ScyllaDB Manager state with a mismatching owner UID label value: %q. ScyllaDBManagerTask %q will not adopt it.", managerTask.Name, managerTask.ID, ownerUIDLabelValue, klog.KObj(smt))
		return progressingConditions, controllertools.NonRetriable(fmt.Errorf("task %q already exists in ScyllaDB Manager state with a mismatching owner UID label value (%q)", managerTask.Name, ownerUIDLabelValue))
	}

	status.TaskID = &managerTask.ID

	err = smtc.syncScyllaV1TaskStatusAnnotation(ctx, smt, managerTask)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't sync scyllav1 task status annotation for ScyllaDBManagerTask %q: %w", naming.ObjRef(smt), err)
	}

	if ownerUIDLabelValue == string(smt.UID) && requiredManagerTask.Labels[naming.ManagedHash] == managerTask.Labels[naming.ManagedHash] {
		// Cluster matches the desired state, nothing to do.
		return progressingConditions, nil
	}

	requiredManagerTask.ID = managerTask.ID

	klog.V(2).InfoS("Updating ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", managerTask.Type, "ScyllaDBManagerClientTaskID", managerTask.ID)
	err = managerClient.UpdateTask(ctx, clusterID, requiredManagerTask)
	if err != nil {
		// Ideally, all invalid tasks should have been caught by the validation webhook, but we can't catch all types of misconfigurations.
		// To avoid unnecessary retries, we return a non-retriable error.
		if managerclienterrors.IsBadRequest(err) {
			klog.V(4).InfoS("Failed to create ScyllaDB Manager client task due to misconfiguration. The operation will not be retried.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", requiredManagerTask.Type, "ScyllaDBManagerClientTaskName", requiredManagerTask.Name, "Error", err)
			return progressingConditions, controllertools.NonRetriable(fmt.Errorf("can't update ScyllaDB Manager client task %q due to misconfiguration: %s", requiredManagerTask.Name, managerclienterrors.GetPayloadMessage(err)))
		}

		klog.V(4).InfoS("Failed to update ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", managerTask.Type, "ScyllaDBManagerClientTaskName", requiredManagerTask.Name, "ScyllaDBManagerClientTaskID", managerTask.ID, "Error", err)
		return progressingConditions, fmt.Errorf("can't update ScyllaDB Manager client task %q: %s", requiredManagerTask.Name, managerclienterrors.GetPayloadMessage(err))
	}

	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               managerControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: smt.Generation,
		Reason:             "UpdatedScyllaDBManagerTask",
		Message:            fmt.Sprintf("Updated a ScyllaDB Manager task: %s (%s).", managerTask.Name, managerTask.ID),
	})

	return progressingConditions, nil
}

func (smtc *Controller) syncManagerClientTaskNotFound(
	ctx context.Context,
	smt *scyllav1alpha1.ScyllaDBManagerTask,
	status *scyllav1alpha1.ScyllaDBManagerTaskStatus,
	managerClient *managerclient.Client,
	clusterID string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredManagerTask, err := makeScyllaDBManagerClientTask(smt, clusterID)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required ScyllaDB Manager task: %w", err)
	}

	klog.V(2).InfoS("Creating ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientTaskName", requiredManagerTask.Name)

	var managerTaskID uuid.UUID
	managerTaskID, err = managerClient.CreateTask(ctx, clusterID, requiredManagerTask)
	if err != nil {
		// Ideally, all invalid tasks should have been caught by the validation webhook, but we can't catch all types of misconfigurations.
		// To avoid unnecessary retries, we return a non-retriable error.
		if managerclienterrors.IsBadRequest(err) {
			klog.V(4).InfoS("Failed to create ScyllaDB Manager client task due to misconfiguration. The operation will not be retried.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", requiredManagerTask.Type, "ScyllaDBManagerClientTaskName", requiredManagerTask.Name, "Error", err)
			return progressingConditions, controllertools.NonRetriable(fmt.Errorf("can't create ScyllaDB Manager client task due to misconfiguration: %s", managerclienterrors.GetPayloadMessage(err)))
		}

		klog.V(4).InfoS("Failed to create ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", requiredManagerTask.Type, "ScyllaDBManagerClientTaskName", requiredManagerTask.Name, "Error", err)
		return progressingConditions, fmt.Errorf("can't create ScyllaDB Manager client task: %s", managerclienterrors.GetPayloadMessage(err))
	}

	status.TaskID = pointer.Ptr(managerTaskID.String())
	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               managerControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: smt.Generation,
		Reason:             "CreatedScyllaDBManagerTask",
		Message:            fmt.Sprintf("Created a ScyllaDB Manager task: %s (%s).", requiredManagerTask.Name, managerTaskID),
	})
	return progressingConditions, nil
}

func (smtc *Controller) syncManagerClientTaskMissingOwnerUID(
	ctx context.Context,
	smt *scyllav1alpha1.ScyllaDBManagerTask,
	managerClient *managerclient.Client,
	clusterID string,
	managerTask *managerclient.TaskListItem,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	managerTaskID, err := uuid.Parse(managerTask.ID)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't parse ScyllaDB Manager client task ID: %w", err)
	}

	klog.Warningf("ScyllaDB Manager client task %q is missing the owner UID label. Deleting it to avoid a name collision.", managerTask.Name)
	err = managerClient.DeleteTask(ctx, clusterID, managerTask.Type, managerTaskID)
	if err != nil {
		klog.V(4).InfoS("Failed to delete ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", managerTask.Type, "ScyllaDBManagerClientTaskName", managerTask.Name, "ScyllaDBManagerClientTaskID", managerTaskID, "Error", err)
		return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager task %q: %s", managerTask.Name, managerclienterrors.GetPayloadMessage(err))
	}

	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               managerControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: smt.Generation,
		Reason:             "DeletedCollidingScyllaDBManagerTask",
		Message:            fmt.Sprintf("Deleted a colliding ScyllaDB Manager task with no OwnerUID label: %s (%s).", managerTask.Name, managerTaskID),
	})
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
		klog.V(4).InfoS("Failed to list ScyllaDB Manager client tasks.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", taskType, "ScyllaDBManagerClientTaskID", taskID, "Error", err)
		return nil, false, fmt.Errorf("can't list ScyllaDB Manager client tasks: %s", managerclienterrors.GetPayloadMessage(err))
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
	if idx < 0 {
		return nil, false, nil
	}

	return tasks.TaskListItemSlice[idx], true, nil
}

type scyllaDBManagerClientTaskOverrideOption func(*scyllav1alpha1.ScyllaDBManagerTask, *managerclient.Task)

// withScheduleStartDateNowSyntaxRetention ensures backward compatibility for tasks created with the deprecated "now" syntax in their start date override annotation.
// On an update, tasks with an already set start date retain it, rather than have a new, recalculated date time set, to prevent the schedule from being restarted continuously.
func withScheduleStartDateNowSyntaxRetention(existingStartDate strfmt.DateTime) func(*scyllav1alpha1.ScyllaDBManagerTask, *managerclient.Task) {
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

func makeScyllaDBManagerClientTask(smt *scyllav1alpha1.ScyllaDBManagerTask, clusterID string, overrideOptions ...scyllaDBManagerClientTaskOverrideOption) (*managerclient.Task, error) {
	getManagedHash := func(t *managerclient.Task) (string, error) {
		return hashutil.HashObjects(t)
	}

	requiredManagerTask, err := makeScyllaDBManagerClientTaskWithManagedHashFunc(smt, clusterID, getManagedHash, overrideOptions...)
	if err != nil {
		return nil, fmt.Errorf("can't make ScyllaDB Manager client task without operator metadata: %w", err)
	}

	return requiredManagerTask, nil
}

func makeScyllaDBManagerClientTaskWithManagedHashFunc(smt *scyllav1alpha1.ScyllaDBManagerTask, clusterID string, getManagedHash func(*managerclient.Task) (string, error), overrideOptions ...scyllaDBManagerClientTaskOverrideOption) (*managerclient.Task, error) {
	var err error
	var managerClientTaskType string

	managerClientTaskName := scyllaDBManagerClientTaskName(smt)
	managerClientTaskSchedule := &managerclient.Schedule{}
	managerClientTaskProperties := map[string]any{}

	var scheduleOverrideOptions []scyllaDBManagerClientScheduleOverrideOption

	intervalOverrideAnnotation, hasIntervalOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation]
	if hasIntervalOverrideAnnotation {
		scheduleOverrideOptions = append(scheduleOverrideOptions, withIntervalOverride(intervalOverrideAnnotation))
	}

	startDateOverrideAnnotation, hasStartDateOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation]
	if hasStartDateOverrideAnnotation {
		scheduleOverrideOptions = append(scheduleOverrideOptions, withStartDateOverride(startDateOverrideAnnotation))
	}

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

		var repairTaskOverrideOptions []scyllaDBManagerClientPropertiesOverrideOption

		intensityOverrideAnnotation, hasIntensityOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation]
		if hasIntensityOverrideAnnotation {
			repairTaskOverrideOptions = append(repairTaskOverrideOptions, withIntensityOverride(intensityOverrideAnnotation))
		}

		smallTableThresholdOverrideAnnotation, hasSmallTableThresholdOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation]
		if hasSmallTableThresholdOverrideAnnotation {
			repairTaskOverrideOptions = append(repairTaskOverrideOptions, withSmallTableThresholdOverride(smallTableThresholdOverrideAnnotation))
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

	managedHash, err := getManagedHash(requiredManagerTask)
	if err != nil {
		return nil, fmt.Errorf("can't calculate managed hash: %w", err)
	}
	requiredManagerTask.Labels[naming.ManagedHash] = managedHash

	return requiredManagerTask, nil
}

type scyllaDBManagerClientScheduleOverrideOption func(*managerclient.Schedule) error

func withIntervalOverride(interval string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		s.Interval = interval
		return nil
	}
}

func withStartDateOverride(startDate string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		parsed, err := parseStartDate(startDate, timeutc.Now)
		if err != nil {
			return fmt.Errorf("can't parse start date: %w", err)
		}

		s.StartDate = &parsed
		return nil
	}
}

// parseStartDate parses the start date string into a strfmt.DateTime.
// The function handles two formats:
// 1. "now[+duration]", e.g. "now+1h", "now+2d". For "now" or "now+duration", where duration is equivalent to 0, it returns an empty DateTime deliberately to indicate the current time should be resolved by the server.
// 2. RFC3339 formatted timestamp (e.g., "2023-05-08T17:24:00Z").
// The nowFunc parameter is a function that returns the current time.
func parseStartDate(s string, nowFunc func() time.Time) (strfmt.DateTime, error) {
	if strings.HasPrefix(s, "now") {
		now := nowFunc().UTC()

		if s == "now" {
			return strfmt.DateTime{}, nil
		}

		d, err := duration.ParseDuration(s[3:])
		if err != nil {
			return strfmt.DateTime{}, fmt.Errorf("can't parse duration: %w", err)
		}

		if d == 0 {
			return strfmt.DateTime{}, nil
		}

		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	t, err := timeutc.Parse(time.RFC3339, s)
	if err != nil {
		return strfmt.DateTime{}, fmt.Errorf("can't parse time: %w", err)
	}

	return strfmt.DateTime(t), nil
}

func withTimezoneOverride(timezone string) func(*managerclient.Schedule) error {
	return func(s *managerclient.Schedule) error {
		s.Timezone = timezone
		return nil
	}
}

func makeScyllaDBManagerClientSchedule(scyllaDBManagerTaskSchedule *scyllav1alpha1.ScyllaDBManagerTaskSchedule, overrideOptions ...scyllaDBManagerClientScheduleOverrideOption) (*managerclient.Schedule, error) {
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

	err := apimachineryutilerrors.NewAggregate(errs)
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

type scyllaDBManagerClientPropertiesOverrideOption func(map[string]any) error

func withIntensityOverride(intensity string) func(map[string]any) error {
	return func(properties map[string]any) error {
		parsed, err := strconv.ParseFloat(intensity, 64)
		if err != nil {
			return fmt.Errorf("can't parse intensity override: %w", err)
		}

		properties["intensity"] = parsed

		return nil
	}
}

func withSmallTableThresholdOverride(smallTableThreshold string) func(map[string]any) error {
	return func(properties map[string]any) error {
		parsed, err := parseByteCount(smallTableThreshold)
		if err != nil {
			return fmt.Errorf("can't parse small table threshold override: %w", err)
		}

		properties["small_table_threshold"] = parsed

		return nil
	}
}

var (
	byteCountRe          = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)(B|[KMGTPE]iB)`)
	byteCountReValueIdx  = 1
	byteCountReSuffixIdx = 2
)

func parseByteCount(s string) (int64, error) {
	const unit = 1024
	var exps = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	parts := byteCountRe.FindStringSubmatch(s)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", s, strings.Join(exps, ","))
	}

	v, err := strconv.ParseFloat(parts[byteCountReValueIdx], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing value for byte size string %q: %w", s, err)
	}

	pow := 0
	for i, e := range exps {
		if e == parts[byteCountReSuffixIdx] {
			pow = i
		}
	}

	mul := math.Pow(unit, float64(pow))

	return int64(v * mul), nil
}

func makeScyllaDBManagerClientRepairTaskProperties(options *scyllav1alpha1.ScyllaDBManagerRepairTaskOptions, overrideOptions ...scyllaDBManagerClientPropertiesOverrideOption) (map[string]any, error) {
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

	if options.IgnoreDownHosts != nil {
		managerClientTaskProperties["ignore_down_hosts"] = *options.IgnoreDownHosts
	}

	if options.Intensity != nil {
		managerClientTaskProperties["intensity"] = *options.Intensity
	}

	if options.Parallel != nil {
		managerClientTaskProperties["parallel"] = *options.Parallel
	}

	if options.SmallTableThreshold != nil {
		managerClientTaskProperties["small_table_threshold"] = options.SmallTableThreshold.Value()
	}

	var errs []error
	for _, optionOverrideFunc := range overrideOptions {
		err := optionOverrideFunc(managerClientTaskProperties)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := apimachineryutilerrors.NewAggregate(errs)
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

func scyllaDBManagerClientTaskName(smt *scyllav1alpha1.ScyllaDBManagerTask) string {
	nameOverrideAnnotationValue, hasNameOverrideAnnotation := smt.Annotations[naming.ScyllaDBManagerTaskNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		return nameOverrideAnnotationValue
	}

	return smt.Name
}

func (smtc *Controller) syncScyllaV1TaskStatusAnnotation(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask, managerClientTask *managerclient.TaskListItem) error {
	scyllaV1TaskStatusAnnotationValue, err := makeScyllaV1TaskStatusAnnotationValue(managerClientTask)
	if err != nil {
		return fmt.Errorf("can't make scyllav1 task status annotation value: %w", err)
	}

	if controllerhelpers.HasMatchingAnnotation(smt, naming.ScyllaDBManagerTaskStatusAnnotation, scyllaV1TaskStatusAnnotationValue) {
		return nil
	}

	patch, err := controllerhelpers.PrepareSetAnnotationPatch(smt, naming.ScyllaDBManagerTaskStatusAnnotation, &scyllaV1TaskStatusAnnotationValue)
	if err != nil {
		return fmt.Errorf("can't prepare patch setting annotation: %w", err)
	}

	_, err = smtc.scyllaClient.ScyllaDBManagerTasks(smt.Namespace).Patch(ctx, smt.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBManagerTask: %w", err)
	}

	return nil
}

func makeScyllaV1TaskStatusAnnotationValue(t *managerclient.TaskListItem) (string, error) {
	buf := bytes.Buffer{}

	switch t.Type {
	case managerclient.BackupTask:
		backupTaskStatus, err := newScyllaV1BackupTaskStatus(t)
		if err != nil {
			return "", fmt.Errorf("can't make scyllav1.BackupTaskStatus: %w", err)
		}

		err = json.NewEncoder(&buf).Encode(backupTaskStatus)
		if err != nil {
			return "", fmt.Errorf("can't encode scyllav1.BackupTaskStatus: %w", err)
		}

	case managerclient.RepairTask:
		repairTaskStatus, err := newScyllaV1RepairTaskStatus(t)
		if err != nil {
			return "", fmt.Errorf("can't make scyllav1.RepairTaskStatus: %w", err)
		}

		err = json.NewEncoder(&buf).Encode(repairTaskStatus)
		if err != nil {
			return "", fmt.Errorf("can't encode scyllav1.RepairTaskStatus: %w", err)
		}

	default:
		return "", fmt.Errorf("unsupported manager client task type: %q", t.Type)

	}

	return buf.String(), nil
}

func newScyllaV1BackupTaskStatus(t *managerclient.TaskListItem) (*scyllav1.BackupTaskStatus, error) {
	bts := &scyllav1.BackupTaskStatus{}

	bts.TaskStatus = newScyllaV1TaskStatus(t)

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, bts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return bts, nil
}

func newScyllaV1RepairTaskStatus(t *managerclient.TaskListItem) (*scyllav1.RepairTaskStatus, error) {
	rts := &scyllav1.RepairTaskStatus{}

	rts.TaskStatus = newScyllaV1TaskStatus(t)

	if t.Properties != nil {
		props := t.Properties.(map[string]interface{})
		if err := mapstructure.Decode(props, rts); err != nil {
			return nil, fmt.Errorf("can't decode properties: %w", err)
		}
	}

	return rts, nil
}

func newScyllaV1TaskStatus(t *managerclient.TaskListItem) scyllav1.TaskStatus {
	taskStatus := scyllav1.TaskStatus{
		SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{},
		Name:                t.Name,
		Labels:              maps.Clone(t.Labels),
		ID:                  pointer.Ptr(t.ID),
	}

	taskStatus.SchedulerTaskStatus = newScyllaV1SchedulerTaskStatus(t.Schedule)

	return taskStatus
}

func newScyllaV1SchedulerTaskStatus(schedule *managerclient.Schedule) scyllav1.SchedulerTaskStatus {
	return scyllav1.SchedulerTaskStatus{
		StartDate:  pointer.Ptr(schedule.StartDate.String()),
		Interval:   pointer.Ptr(schedule.Interval),
		NumRetries: pointer.Ptr(schedule.NumRetries),
		Cron:       pointer.Ptr(schedule.Cron),
		Timezone:   pointer.Ptr(schedule.Timezone),
	}
}
