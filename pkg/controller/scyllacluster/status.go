// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
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

func (scmc *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, sdcs map[string]*scyllav1alpha1.ScyllaDBDatacenter, configMaps []*corev1.ConfigMap, services []*corev1.Service, scyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration, scyllaDBManagerTasks map[string]*scyllav1alpha1.ScyllaDBManagerTask) scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()
	if len(sdcs) == 0 {
		return *status
	}

	sdc, ok := sdcs[sc.Name]
	if !ok {
		return *status
	}

	migratedStatus := migrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc, configMaps, services)

	// TODO: move to func (or to the SDC func as this is "internal")
	managerID := status.ManagerID
	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		klog.ErrorS(err, "Can't get ScyllaDBManagerClusterRegistration name for ScyllaDBDatacenter", "ScyllaCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
	} else {
		idx := slices.IndexFunc(scyllaDBManagerClusterRegistrations, func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) bool {
			return smcr.Name == smcrName
		})
		if idx >= 0 {
			smcr := scyllaDBManagerClusterRegistrations[idx]
			managerID = smcr.Status.ClusterID
		} else {
			klog.V(4).InfoS("ScyllaDBManagerClusterRegistration not found for ScyllaDBDatacenter", "ScyllaCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc), "ScyllaDBManagerClusterRegistration", smcrName)
		}
	}

	// TODO: move to func
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

		rts := scyllav1.BackupTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: backupTaskSpec.Name,
				ID:   smt.Status.TaskID,
			},
		}

		degradedCondition := meta.FindStatusCondition(smt.Status.Conditions, scyllav1alpha1.DegradedCondition)
		if degradedCondition != nil && degradedCondition.Status == metav1.ConditionTrue {
			rts.Error = pointer.Ptr(degradedCondition.Message)
		}

		scyllav1TaskStatusAnnotation, ok := smt.Annotations[naming.ScyllaDBManagerTaskStatusAnnotation]
		if ok {
			err = json.NewDecoder(strings.NewReader(scyllav1TaskStatusAnnotation)).Decode(&rts)
			if err != nil {
				klog.ErrorS(err, "Can't decode ScyllaDBManagerTask status annotation", "ScyllaCluster", klog.KObj(sc), "BackupTask", backupTaskSpec.Name, "ScyllaDBManagerTask", smtName)
				continue
			}
		} else if rts.Error == nil {
			// Do not append status if neither task's status nor error have been propagated yet.
			continue
		}

		backupTaskStatuses = append(backupTaskStatuses, rts)
	}

	// TODO: move to func
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

		rts := scyllav1.RepairTaskStatus{
			TaskStatus: scyllav1.TaskStatus{
				Name: repairTaskSpec.Name,
				ID:   smt.Status.TaskID,
			},
		}

		degradedCondition := meta.FindStatusCondition(smt.Status.Conditions, scyllav1alpha1.DegradedCondition)
		if degradedCondition != nil && degradedCondition.Status == metav1.ConditionTrue {
			rts.Error = pointer.Ptr(degradedCondition.Message)
		}

		scyllav1TaskStatusAnnotation, ok := smt.Annotations[naming.ScyllaDBManagerTaskStatusAnnotation]
		if ok {
			err = json.NewDecoder(strings.NewReader(scyllav1TaskStatusAnnotation)).Decode(&rts)
			if err != nil {
				klog.ErrorS(err, "Can't decode ScyllaDBManagerTask status annotation", "ScyllaCluster", klog.KObj(sc), "RepairTask", repairTaskSpec.Name, "ScyllaDBManagerTask", smtName)
				continue
			}
		} else if rts.Error == nil {
			// Do not append status if neither task's status nor error have been propagated yet.
			continue
		}

		repairTaskStatuses = append(repairTaskStatuses, rts)
	}

	return scyllav1.ScyllaClusterStatus{
		ObservedGeneration: migratedStatus.ObservedGeneration,
		Racks:              migratedStatus.Racks,
		Members:            migratedStatus.Members,
		ReadyMembers:       migratedStatus.ReadyMembers,
		AvailableMembers:   migratedStatus.AvailableMembers,
		RackCount:          migratedStatus.RackCount,
		Upgrade:            migratedStatus.Upgrade,
		Conditions:         migratedStatus.Conditions,
		ManagerID:          managerID,
		Repairs:            repairTaskStatuses,
		Backups:            backupTaskStatuses,
	}
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
