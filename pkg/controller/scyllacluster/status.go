// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
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

func (scmc *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, sdcs map[string]*scyllav1alpha1.ScyllaDBDatacenter, configMaps []*corev1.ConfigMap, services []*corev1.Service) scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()
	if len(sdcs) == 0 {
		return *status
	}

	sdc, ok := sdcs[sc.Name]
	if !ok {
		return *status
	}

	migratedStatus := migrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc, configMaps, services)

	return scyllav1.ScyllaClusterStatus{
		ObservedGeneration: migratedStatus.ObservedGeneration,
		Racks:              migratedStatus.Racks,
		Members:            migratedStatus.Members,
		ReadyMembers:       migratedStatus.ReadyMembers,
		AvailableMembers:   migratedStatus.AvailableMembers,
		RackCount:          migratedStatus.RackCount,
		Upgrade:            migratedStatus.Upgrade,
		Conditions:         migratedStatus.Conditions,
		ManagerID:          status.ManagerID,
		Repairs:            status.Repairs,
		Backups:            status.Backups,
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
