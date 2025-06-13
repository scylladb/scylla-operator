// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func (smtc *Controller) syncFinalizer(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	if !smtc.hasFinalizer(smt.GetFinalizers()) {
		klog.V(4).InfoS("Object is already finalized", "ScyllaDBManagerTask", klog.KObj(smt), "UID", smt.UID)
		return progressingConditions, nil
	}

	klog.V(4).InfoS("Finalizing object", "ScyllaDBManagerTask", klog.KObj(smt), "UID", smt.UID)

	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	smcr, err := smtc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(smt.Namespace).Get(smcrName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return progressingConditions, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration: %w", err)
		}

		klog.V(4).InfoS("ScyllaDBManagerClusterRegistration referenced by ScyllaDBManagerTask does not exist, removing finalizer.", "ScyllaDBManagerTask", klog.KObj(smt), "UID", smt.UID, "ScyllaDBManagerClusterRegistration", klog.KRef(smt.Namespace, smcrName))

		err = smtc.removeFinalizer(ctx, smt)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't remove finalizer: %w", err)
		}

		return progressingConditions, nil
	}

	if smcr.Status.ClusterID == nil || len(*smcr.Status.ClusterID) == 0 {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               scyllaDBManagerTaskFinalizerProgressingCondition,
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
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager client: %w", err)
	}

	managerTask, found, err := getScyllaDBManagerClientTask(ctx, smt, clusterID, managerClient)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager client task: %w", err)
	}

	if !found {
		klog.V(4).InfoS("ScyllaDB Manager task has already been deleted. Removing the finalizer.", "ScyllaDBManagerTask", klog.KObj(smt), "UID", smt.UID)
		err = smtc.removeFinalizer(ctx, smt)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't remove finalizer: %w", err)
		}

		return progressingConditions, nil
	}

	var managerTaskID uuid.UUID
	managerTaskID, err = uuid.Parse(managerTask.ID)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't parse ScyllaDB Manager client task ID: %w", err)
	}

	err = managerClient.DeleteTask(ctx, clusterID, managerTask.Type, managerTaskID)
	if err != nil {
		klog.V(4).InfoS("Failed to delete ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "ScyllaDBManagerClientClusterID", clusterID, "ScyllaDBManagerClientTaskType", managerTask.Type, "ScyllaDBManagerClientTaskName", managerTask.Name, "ScyllaDBManagerClientTaskID", managerTaskID, "Error", err)
		return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager client task %q (%q): %s", managerTask.Name, managerTask.ID, managerclienterrors.GetPayloadMessage(err))
	}

	klog.V(4).InfoS("Deleted the ScyllaDB Manager client task.", "ScyllaDBManagerTask", klog.KObj(smt), "UID", smt.UID, "ScyllaDBManagerClientTaskName", managerTask.Name, "ScyllaDBManagerClientTaskID", managerTask.ID)
	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               scyllaDBManagerTaskFinalizerProgressingCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: smt.Generation,
		Reason:             "DeletedScyllaDBManagerClientTask",
		Message:            "Deleted the task from ScyllaDB Manager state.",
	})

	return progressingConditions, nil
}

func (smtc *Controller) removeFinalizer(ctx context.Context, smt *scyllav1alpha1.ScyllaDBManagerTask) error {
	patch, err := controllerhelpers.RemoveFinalizerPatch(smt, naming.ScyllaDBManagerTaskFinalizer)
	if err != nil {
		return fmt.Errorf("can't create remove finalizer patch: %w", err)
	}

	_, err = smtc.scyllaClient.ScyllaDBManagerTasks(smt.Namespace).Patch(ctx, smt.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBManagerTask %q: %w", naming.ObjRef(smt), err)
	}

	klog.V(2).InfoS("Removed finalizer from ScyllaDBManagerTask", "ScyllaDBManagerTask", klog.KObj(smt))
	return nil
}
