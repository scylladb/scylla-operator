package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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

// getScyllaVersion returns the Scylla version from the ordinal-0 pod owned by sts.
func getScyllaVersion(podLister corev1listers.PodLister, sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", sts.Name)
	firstMember, err := podLister.Pods(sts.Namespace).Get(firstMemberName)
	if err != nil {
		return "", fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sts.Namespace, firstMemberName), err)
	}

	controllerRef := metav1.GetControllerOfNoCopy(firstMember)
	if controllerRef == nil || controllerRef.UID != sts.UID {
		return "", fmt.Errorf("internal error: pod %q is not controlled by us", naming.ManualRef(sts.Namespace, firstMemberName))
	}

	version, err := naming.ScyllaVersion(firstMember.Spec.Containers)
	if err != nil {
		return "", fmt.Errorf("can't get scylla version for pod %q: %w", naming.ManualRef(sts.Namespace, firstMemberName), err)
	}

	return version, nil
}

// calculateRackStatus builds rack status from its StatefulSet.
// If sts is nil, it returns a stale zero status so the rack still appears in status.
// Empty racks report the target image version; non-empty racks report the version from ordinal-0.
func calculateRackStatus(podLister corev1listers.PodLister, sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string, sts *appsv1.StatefulSet) *scyllav1alpha1.RackStatus {
	status := &scyllav1alpha1.RackStatus{
		Name:           rackName,
		Nodes:          new(int32(0)),
		CurrentNodes:   new(int32(0)),
		UpdatedNodes:   new(int32(0)),
		ReadyNodes:     new(int32(0)),
		AvailableNodes: new(int32(0)),
		Stale:          new(true),
	}

	if sts == nil {
		return status
	}

	status.Nodes = new(*sts.Spec.Replicas)
	status.ReadyNodes = new(sts.Status.ReadyReplicas)
	status.AvailableNodes = new(sts.Status.AvailableReplicas)
	status.UpdatedNodes = new(sts.Status.UpdatedReplicas)
	status.CurrentNodes = new(sts.Status.CurrentReplicas)
	status.Stale = new(sts.Status.ObservedGeneration < sts.Generation)

	scyllaDBImageVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		klog.ErrorS(err, "can't get version of image", "Image", sdc.Spec.ScyllaDB.Image)
	}

	status.UpdatedVersion = scyllaDBImageVersion

	// Update Rack Version
	if status.Nodes != nil && *status.Nodes == 0 {
		status.CurrentVersion = scyllaDBImageVersion
	} else {
		version, err := getScyllaVersion(podLister, sts)
		if err != nil {
			klog.ErrorS(err, "can't get scylla version")
		} else {
			status.CurrentVersion = version
		}
	}

	return status
}

func updateAggregatedStatusFields(status *scyllav1alpha1.ScyllaDBDatacenterStatus) {
	status.Nodes = new(int32(0))
	status.ReadyNodes = new(int32(0))
	status.AvailableNodes = new(int32(0))

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
	status.ObservedGeneration = new(sdc.Generation)

	// Clear the previous rack status.
	status.Racks = []scyllav1alpha1.RackStatus{}

	// Calculate the status for racks.
	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		rackStatus := calculateRackStatus(sdcc.podLister, sdc, rack.Name, statefulSetMap[stsName])
		// The record of decommissioning nodes isn't derived from the observed state, it's maintained explicitly by the
		// StatefulSet sync, so carry it over.
		rackStatus.DecommissioningNodes = getRackDecommissioningNodes(&sdc.Status, rack.Name)
		status.Racks = append(status.Racks, *rackStatus)
	}

	updateAggregatedStatusFields(status)

	return status
}
