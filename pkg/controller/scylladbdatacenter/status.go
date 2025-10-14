package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (sdcc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1alpha1.ScyllaDBDatacenter, status *scyllav1alpha1.ScyllaDBDatacenterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sdc := currentSC.DeepCopy()
	sdc.Status = *status

	// Make sure that any "live" updates to the status are always manifested in the aggregated fields.
	updateAggregatedStatusFields(&sdc.Status)

	klog.V(2).InfoS("Updating status", "ScyllaDBDatacenter", klog.KObj(sdc))

	_, err := sdcc.scyllaClient.ScyllaDBDatacenters(sdc.Namespace).UpdateStatus(ctx, sdc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBDatacenter", klog.KObj(sdc))

	return nil
}

func (sdcc *Controller) getScyllaVersion(sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", sts.Name)
	firstMember, err := sdcc.podLister.Pods(sts.Namespace).Get(firstMemberName)
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
func (sdcc *Controller) calculateRackStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string, sts *appsv1.StatefulSet) *scyllav1alpha1.RackStatus {
	status := &scyllav1alpha1.RackStatus{
		Name:           rackName,
		Nodes:          pointer.Ptr(int32(0)),
		CurrentNodes:   pointer.Ptr(int32(0)),
		UpdatedNodes:   pointer.Ptr(int32(0)),
		ReadyNodes:     pointer.Ptr(int32(0)),
		AvailableNodes: pointer.Ptr(int32(0)),
		Stale:          pointer.Ptr(true),
	}

	if sts == nil {
		return status
	}

	status.Nodes = pointer.Ptr(*sts.Spec.Replicas)
	status.ReadyNodes = pointer.Ptr(sts.Status.ReadyReplicas)
	status.AvailableNodes = pointer.Ptr(sts.Status.AvailableReplicas)
	status.UpdatedNodes = pointer.Ptr(sts.Status.UpdatedReplicas)
	status.CurrentNodes = pointer.Ptr(sts.Status.CurrentReplicas)
	status.Stale = pointer.Ptr(sts.Status.ObservedGeneration < sts.Generation)

	scyllaDBImageVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		klog.ErrorS(err, "can't get version of image", "Image", sdc.Spec.ScyllaDB.Image)
	}

	status.UpdatedVersion = scyllaDBImageVersion

	// Update Rack Version
	if status.Nodes != nil && *status.Nodes == 0 {
		status.CurrentVersion = scyllaDBImageVersion
	} else {
		version, err := sdcc.getScyllaVersion(sts)
		if err != nil {
			klog.ErrorS(err, "can't get scylla version")
		} else {
			status.CurrentVersion = version
		}
	}

	return status
}

func updateAggregatedStatusFields(status *scyllav1alpha1.ScyllaDBDatacenterStatus) {
	status.Nodes = pointer.Ptr(int32(0))
	status.ReadyNodes = pointer.Ptr(int32(0))
	status.AvailableNodes = pointer.Ptr(int32(0))

	for rackName := range status.Racks {
		rackStatus := status.Racks[rackName]

		*status.Nodes += *rackStatus.Nodes
		*status.ReadyNodes += *rackStatus.ReadyNodes
		*status.AvailableNodes += *rackStatus.AvailableNodes
	}
}

// calculateStatus calculates the ScyllaCluster status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (sdcc *Controller) calculateStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, statefulSetMap map[string]*appsv1.StatefulSet) *scyllav1alpha1.ScyllaDBDatacenterStatus {
	status := sdc.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(sdc.Generation)

	// Clear the previous rack status.
	status.Racks = []scyllav1alpha1.RackStatus{}

	// Calculate the status for racks.
	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		status.Racks = append(status.Racks, *sdcc.calculateRackStatus(sdc, rack.Name, statefulSetMap[stsName]))
	}

	updateAggregatedStatusFields(status)

	return status
}
