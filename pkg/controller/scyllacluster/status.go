package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	controllerhelpers "github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

func (scc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1.ScyllaCluster, status *scyllav1.ScyllaClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaCluster", klog.KObj(sc))

	_, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaCluster", klog.KObj(sc))

	return nil
}

func (scc *Controller) getScyllaVersion(sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", sts.Name)
	firstMember, err := scc.podLister.Pods(sts.Namespace).Get(firstMemberName)
	if err != nil {
		return "", err
	}

	controllerRef := metav1.GetControllerOfNoCopy(firstMember)
	if controllerRef == nil || controllerRef.UID != sts.UID {
		return "", fmt.Errorf("foreign pod")
	}

	version, err := naming.ScyllaVersion(firstMember.Spec.Containers)
	if err != nil {
		return "", err
	}

	return version, nil
}

// calculateRackStatus calculates a status for the rack.
// sts and old status may be nil.
func (scc *Controller) calculateRackStatus(sc *scyllav1.ScyllaCluster, rackName string, sts *appsv1.StatefulSet, oldRackStatus *scyllav1.RackStatus, serviceMap map[string]*corev1.Service) *scyllav1.RackStatus {
	status := &scyllav1.RackStatus{
		ReplaceAddressFirstBoot: map[string]string{},
	}

	// Persist ReplaceAddressFirstBoot.
	if oldRackStatus != nil && oldRackStatus.ReplaceAddressFirstBoot != nil {
		status.ReplaceAddressFirstBoot = oldRackStatus.DeepCopy().ReplaceAddressFirstBoot
	}

	if sts == nil {
		return status
	}

	status.Members = *sts.Spec.Replicas
	status.ReadyMembers = sts.Status.ReadyReplicas
	status.UpdatedMembers = pointer.Int32Ptr(sts.Status.UpdatedReplicas)
	status.Stale = pointer.BoolPtr(sts.Status.ObservedGeneration < sts.Generation)

	// Update Rack Version
	if status.Members == 0 {
		status.Version = sc.Spec.Version
	} else {
		version, err := scc.getScyllaVersion(sts)
		if err != nil {
			status.Version = ""
			klog.ErrorS(err, "Can't get scylla version", "ScyllaCluster", klog.KObj(sc))
		} else {
			status.Version = version
		}
	}

	// Update Upgrading condition
	desiredRackVersion := sc.Spec.Version
	actualRackVersion := status.Version
	if desiredRackVersion != actualRackVersion {
		controllerhelpers.SetRackCondition(status, scyllav1.RackConditionTypeUpgrading)
	}

	// Update Scaling Down condition
	for _, svc := range serviceMap {
		// Check if there is a decommission in progress
		if _, ok := svc.Labels[naming.DecommissionedLabel]; ok {
			// Add MemberLeaving Condition to rack status
			controllerhelpers.SetRackCondition(status, scyllav1.RackConditionTypeMemberLeaving)
			// Sanity check. Only the last member should be decommissioning.
			index, err := naming.IndexFromName(svc.Name)
			if err != nil {
				klog.ErrorS(err, "Can't determine service index from its name", "Service", klog.KObj(svc))
				continue
			}
			if index != status.Members-1 {
				klog.Errorf("only last member of each rack should be decommissioning, but %d-th member of %s found decommissioning while rack had %d members", index, rackName, status.Members)
				continue
			}
		}
	}

	return status
}

// calculateStatus calculates the ScyllaCluster status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (scc *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, statefulSetMap map[string]*appsv1.StatefulSet, serviceMap map[string]*corev1.Service) *scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()
	status.ObservedGeneration = pointer.Int64Ptr(sc.Generation)

	// Clear the previous rack status.
	status.Racks = map[string]scyllav1.RackStatus{}

	// Calculate the status for racks.
	for _, rack := range sc.Spec.Datacenter.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sc)
		oldRackStatus := sc.Status.Racks[rack.Name]
		status.Racks[rack.Name] = *scc.calculateRackStatus(sc, rack.Name, statefulSetMap[stsName], &oldRackStatus, serviceMap)
	}

	return status
}
