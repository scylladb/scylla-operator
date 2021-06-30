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
	_, err := scc.scyllaClient.ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	return err
}

func (scc *Controller) getScyllaVersion(sc *scyllav1.ScyllaCluster, rack *scyllav1.RackSpec, sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(*rack, sc))
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

// calculateStatus calculates the ScyllaCluster status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (scc *Controller) calculateStatus(sc *scyllav1.ScyllaCluster, statefulSetMap map[string]*appsv1.StatefulSet, serviceMap map[string]*corev1.Service) *scyllav1.ScyllaClusterStatus {
	status := sc.Status.DeepCopy()

	status.ObservedGeneration = pointer.Int64Ptr(sc.Generation)

	if status.Racks == nil {
		status.Racks = map[string]scyllav1.RackStatus{}
	}

	// Find racks which are no longer specified.
	var unknownRacks []string
	for rackName := range status.Racks {
		found := false
		for _, rack := range sc.Spec.Datacenter.Racks {
			if rack.Name == rackName {
				found = true
				break
			}
		}
		if !found {
			unknownRacks = append(unknownRacks, rackName)
		}
	}
	// Remove unknown rack from status
	for _, rackName := range unknownRacks {
		delete(status.Racks, rackName)
	}

	// Update each rack status
	for _, rack := range sc.Spec.Datacenter.Racks {
		rackStatus := scyllav1.RackStatus{
			ReplaceAddressFirstBoot: make(map[string]string),
		}
		// ReplaceAddress may keep address of non-existing service.
		// Copy it from previous cluster status to persist this state.
		if existingStatus, ok := status.Racks[rack.Name]; ok {
			for k, v := range existingStatus.ReplaceAddressFirstBoot {
				rackStatus.ReplaceAddressFirstBoot[k] = v
			}
		}

		stsName := naming.StatefulSetNameForRack(rack, sc)
		rackStatefulSet := statefulSetMap[stsName]
		if rackStatefulSet == nil {
			continue
		}

		// Update Members
		rackStatus.Members = *rackStatefulSet.Spec.Replicas
		// Update ReadyMembers
		rackStatus.ReadyMembers = rackStatefulSet.Status.ReadyReplicas

		// Update Rack Version
		if rackStatus.Members == 0 {
			rackStatus.Version = sc.Spec.Version
		} else {
			version, err := scc.getScyllaVersion(sc, &rack, rackStatefulSet)
			if err != nil {
				rackStatus.Version = ""
				klog.ErrorS(err, "Can't get scylla version", "ScyllaCluster", klog.KObj(sc))
			} else {
				rackStatus.Version = version
			}
		}

		// Update Upgrading condition
		desiredRackVersion := sc.Spec.Version
		actualRackVersion := rackStatus.Version
		if desiredRackVersion != actualRackVersion {
			controllerhelpers.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeUpgrading)
		}

		// Update Scaling Down condition
		for _, svc := range serviceMap {
			// Check if there is a decommission in progress
			if _, ok := svc.Labels[naming.DecommissionedLabel]; ok {
				// Add MemberLeaving Condition to rack status
				controllerhelpers.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeMemberLeaving)
				// Sanity check. Only the last member should be decommissioning.
				index, err := naming.IndexFromName(svc.Name)
				if err != nil {
					klog.ErrorS(err, "Can't determine service index from its name", "Service", klog.KObj(svc))
					continue
				}
				if index != rackStatus.Members-1 {
					klog.Errorf("only last member of each rack should be decommissioning, but %d-th member of %s found decommissioning while rack had %d members", index, rack.Name, rackStatus.Members)
					continue
				}
			}
		}

		// Update Status for Rack
		status.Racks[rack.Name] = rackStatus
	}

	return status
}
