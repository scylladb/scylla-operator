// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"slices"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func (scmc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1.ScyllaCluster, status scyllav1.ScyllaClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = status

	klog.V(2).InfoS("Updating status", "ScyllaCluster", klog.KObj(sc))

	_, err := scmc.scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaCluster", klog.KObj(sc))

	return nil
}

func (scmc *Controller) calculateStatus(
	sc *scyllav1.ScyllaCluster,
	sdcs map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	configMaps []*corev1.ConfigMap,
	services []*corev1.Service,
	scyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration,
	scyllaDBManagerTasks map[string]*scyllav1alpha1.ScyllaDBManagerTask,
) scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()
	if len(sdcs) == 0 {
		return *status
	}

	sdc, ok := sdcs[sc.Name]
	if !ok {
		return *status
	}

	migratedStatus := migrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc, configMaps, services)

	if isGlobalScyllaDBManagerIntegrationDisabled(sc) {
		klog.V(4).InfoS("ScyllaDBManager integration is disabled, skipping migration of repair and backup tasks' statuses", "ScyllaCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
		return migratedStatus
	}

	smcr, ok, err := getScyllaDBManagerClusterRegistration(sdc, scyllaDBManagerClusterRegistrations)
	if err != nil {
		klog.ErrorS(err, "Can't get ScyllaDBManagerClusterRegistration for ScyllaDBDatacenter", "ScyllaCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
	}
	if ok {
		migratedStatus.ManagerID = smcr.Status.ClusterID
	} else {
		klog.V(4).InfoS("ScyllaDBManagerClusterRegistration not found for ScyllaDBDatacenter", "ScyllaCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
	}

	migratedStatus.Backups = calculateBackupTaskStatuses(sc, scyllaDBManagerTasks)
	migratedStatus.Repairs = calculateRepairTaskStatuses(sc, scyllaDBManagerTasks)

	return migratedStatus
}

func isOwnedByAnyFunc[T kubeinterfaces.ObjectInterface](allowedOwners []types.UID) func(T) bool {
	return func(obj T) bool {
		controllerRef := metav1.GetControllerOfNoCopy(obj)
		if controllerRef == nil {
			return false
		}

		return oslices.ContainsItem(allowedOwners, controllerRef.UID)
	}
}

func getScyllaDBManagerClusterRegistration(sdc *scyllav1alpha1.ScyllaDBDatacenter, scyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, bool, error) {
	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		return nil, false, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	idx := slices.IndexFunc(scyllaDBManagerClusterRegistrations, func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) bool {
		return smcr.Name == smcrName
	})
	if idx < 0 {
		return nil, false, nil
	}

	return scyllaDBManagerClusterRegistrations[idx], true, nil
}

func calculateBackupTaskStatuses(sc *scyllav1.ScyllaCluster, scyllaDBManagerTasks map[string]*scyllav1alpha1.ScyllaDBManagerTask) []scyllav1.BackupTaskStatus {
	var backupTaskStatuses []scyllav1.BackupTaskStatus
	for _, backupTaskSpec := range sc.Spec.Backups {
		smtName, err := scyllaDBManagerTaskName(sc.Name, scyllav1alpha1.ScyllaDBManagerTaskTypeBackup, backupTaskSpec.Name)
		if err != nil {
			klog.ErrorS(err, "Can't get ScyllaDBManagerTask name for backup task", "ScyllaCluster", klog.KObj(sc), "BackupTask", backupTaskSpec.Name)
			continue
		}

		smt, ok := scyllaDBManagerTasks[smtName]
		if !ok {
			klog.V(4).InfoS("ScyllaDBManagerTask not found for backup task", "ScyllaCluster", klog.KObj(sc), "BackupTask", backupTaskSpec.Name, "ScyllaDBManagerTask", klog.KRef(sc.Namespace, smtName))
			continue
		}

		taskStatus, ok, err := migrateV1Alpha1ScyllaDBManagerTaskStatusToV1BackupTaskStatus(smt, backupTaskSpec.Name)
		if err != nil {
			klog.ErrorS(err, "Can't migrate v1alpha1.ScyllaDBManagerTask status to v1.BackupTask status", "ScyllaCluster", klog.KObj(sc), "BackupTask", backupTaskSpec.Name, "ScyllaDBManagerTask", smtName)
			continue
		}

		if !ok {
			continue
		}

		backupTaskStatuses = append(backupTaskStatuses, taskStatus)
	}

	return backupTaskStatuses
}

func calculateRepairTaskStatuses(sc *scyllav1.ScyllaCluster, scyllaDBManagerTasks map[string]*scyllav1alpha1.ScyllaDBManagerTask) []scyllav1.RepairTaskStatus {
	var repairTaskStatuses []scyllav1.RepairTaskStatus
	for _, repairTaskSpec := range sc.Spec.Repairs {
		smtName, err := scyllaDBManagerTaskName(sc.Name, scyllav1alpha1.ScyllaDBManagerTaskTypeRepair, repairTaskSpec.Name)
		if err != nil {
			klog.ErrorS(err, "Can't get ScyllaDBManagerTask name for repair task", "ScyllaCluster", klog.KObj(sc), "RepairTask", repairTaskSpec.Name)
			continue
		}

		smt, ok := scyllaDBManagerTasks[smtName]
		if !ok {
			klog.V(4).InfoS("ScyllaDBManagerTask not found for repair task", "ScyllaCluster", klog.KObj(sc), "RepairTask", repairTaskSpec.Name, "ScyllaDBManagerTask", klog.KRef(sc.Namespace, smtName))
			continue
		}

		taskStatus, ok, err := migrateV1Alpha1ScyllaDBManagerTaskStatusToV1RepairTaskStatus(smt, repairTaskSpec.Name)
		if err != nil {
			klog.ErrorS(err, "Can't migrate v1alpha1.ScyllaDBManagerTask status to v1.RepairTaskStatus", "ScyllaCluster", klog.KObj(sc), "RepairTask", repairTaskSpec.Name, "ScyllaDBManagerTask", smtName)
			continue
		}

		if !ok {
			continue
		}

		repairTaskStatuses = append(repairTaskStatuses, taskStatus)
	}

	return repairTaskStatuses
}
