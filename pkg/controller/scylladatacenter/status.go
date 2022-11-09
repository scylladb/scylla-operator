// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

func (sdc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1alpha1.ScyllaDatacenter, status *scyllav1alpha1.ScyllaDatacenterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaDatacenter", klog.KObj(sc))

	_, err := sdc.scyllaClient.ScyllaDatacenters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDatacenter", klog.KObj(sc))

	return nil
}

func (sdc *Controller) getScyllaImage(sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", sts.Name)
	firstMember, err := sdc.podLister.Pods(sts.Namespace).Get(firstMemberName)
	if err != nil {
		return "", err
	}

	controllerRef := metav1.GetControllerOfNoCopy(firstMember)
	if controllerRef == nil || controllerRef.UID != sts.UID {
		return "", fmt.Errorf("foreign pod")
	}

	image, err := naming.ScyllaImage(firstMember.Spec.Containers)
	if err != nil {
		return "", err
	}

	return image, nil
}

// calculateRackStatus calculates a status for the rack.
// sts and old status may be nil.
func (sdc *Controller) calculateRackStatus(sc *scyllav1alpha1.ScyllaDatacenter, rackName string, sts *appsv1.StatefulSet, oldRackStatus *scyllav1alpha1.RackStatus, serviceMap map[string]*corev1.Service) *scyllav1alpha1.RackStatus {
	status := &scyllav1alpha1.RackStatus{
		ReplaceAddressFirstBoot: map[string]string{},
	}

	// Persist ReplaceAddressFirstBoot.
	if oldRackStatus != nil && oldRackStatus.ReplaceAddressFirstBoot != nil {
		status.ReplaceAddressFirstBoot = oldRackStatus.DeepCopy().ReplaceAddressFirstBoot
	}

	if sts == nil {
		return status
	}

	status.Nodes = sts.Spec.Replicas
	status.ReadyNodes = pointer.Int32Ptr(sts.Status.ReadyReplicas)
	status.UpdatedNodes = pointer.Int32Ptr(sts.Status.UpdatedReplicas)
	status.Stale = pointer.BoolPtr(sts.Status.ObservedGeneration < sts.Generation)

	// Update Rack Version
	if status.Nodes == nil && *status.Nodes == 0 {
		status.Image = sc.Spec.Scylla.Image
	} else {
		image, err := sdc.getScyllaImage(sts)
		if err != nil {
			status.Image = ""
			klog.ErrorS(err, "Can't get scylla image", "ScyllaDatacenter", klog.KObj(sc))
		} else {
			status.Image = image
		}
	}

	// Update Upgrading condition
	desiredRackVersion := sc.Spec.Scylla.Image
	actualRackVersion := status.Image
	if desiredRackVersion != actualRackVersion {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:    scyllav1alpha1.RackConditionTypeUpgrading,
			Status:  metav1.ConditionTrue,
			Reason:  "VersionsDiffer",
			Message: "Desired and actual versions are different, rack is upgrading",
		})
	}

	// Update Scaling Down condition
	for _, svc := range serviceMap {
		// Check if there is a decommission in progress
		if _, ok := svc.Labels[naming.DecommissionedLabel]; ok {
			// Add MemberLeaving Condition to rack status
			meta.SetStatusCondition(&status.Conditions, metav1.Condition{
				Status:  metav1.ConditionTrue,
				Type:    scyllav1alpha1.RackConditionTypeNodeLeaving,
				Reason:  "NodeLeaving",
				Message: "Node is leaving a rack",
			})
			// Sanity check. Only the last member should be decommissioning.
			index, err := naming.IndexFromName(svc.Name)
			if err != nil {
				klog.ErrorS(err, "Can't determine service index from its name", "Service", klog.KObj(svc))
				continue
			}
			if index != *status.Nodes-1 {
				klog.Errorf("only last member of each rack should be decommissioning, but %d-th member of %s found decommissioning while rack had %d members", index, rackName, status.Nodes)
				continue
			}
		}
	}

	return status
}

// calculateStatus calculates the ScyllaDatacenter status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (sdc *Controller) calculateStatus(sc *scyllav1alpha1.ScyllaDatacenter, statefulSetMap map[string]*appsv1.StatefulSet, serviceMap map[string]*corev1.Service) *scyllav1alpha1.ScyllaDatacenterStatus {
	status := sc.Status.DeepCopy()
	status.ObservedGeneration = pointer.Int64Ptr(sc.Generation)

	// Clear the previous rack status.
	status.Racks = map[string]scyllav1alpha1.RackStatus{}

	// Calculate the status for racks.
	for _, rack := range sc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sc)
		oldRackStatus := sc.Status.Racks[rack.Name]
		status.Racks[rack.Name] = *sdc.calculateRackStatus(sc, rack.Name, statefulSetMap[stsName], &oldRackStatus, serviceMap)
	}

	return status
}
