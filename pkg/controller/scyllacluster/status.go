// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

func (scc *Controller) updateStatus(ctx context.Context, currentSC *scyllav2alpha1.ScyllaCluster, status *scyllav2alpha1.ScyllaClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaCluster", klog.KObj(sc))
	_, err := scc.scyllaClient.ScyllaV2alpha1().ScyllaClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaCluster", klog.KObj(sc))

	return nil
}

// calculateStatus calculates the ScyllaCluster status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (scc *Controller) calculateStatus(sc *scyllav2alpha1.ScyllaCluster, datacentersMap map[string]map[string]*scyllav1alpha1.ScyllaDatacenter) *scyllav2alpha1.ScyllaClusterStatus {
	status := &scyllav2alpha1.ScyllaClusterStatus{}
	status.ObservedGeneration = pointer.Int64(sc.Generation)

	var nodes, currentNodes, readyNodes, updatedNodes int32
	for _, dc := range sc.Spec.Datacenters {
		remoteName := ""
		if dc.RemoteKubeClusterConfigRef != nil {
			remoteName = dc.RemoteKubeClusterConfigRef.Name
		}
		scyllaDatacenters := datacentersMap[remoteName]
		sd := scyllaDatacenters[sc.Name]
		dcStatus := *scc.calculateDatacenterStatus(sc, dc, sd)

		status.Datacenters = append(status.Datacenters, dcStatus)

		if dcStatus.Nodes != nil {
			nodes += *dcStatus.Nodes
		}
		if dcStatus.UpdatedNodes != nil {
			updatedNodes += *dcStatus.UpdatedNodes
		}
		if dcStatus.ReadyNodes != nil {
			readyNodes += *dcStatus.ReadyNodes
		}
		if dcStatus.CurrentNodes != nil {
			currentNodes += *dcStatus.CurrentNodes
		}
	}

	status.Nodes = pointer.Int32(nodes)
	status.CurrentNodes = pointer.Int32(currentNodes)
	status.UpdatedNodes = pointer.Int32(updatedNodes)
	status.ReadyNodes = pointer.Int32(readyNodes)

	return status
}

func (scc *Controller) calculateDatacenterStatus(sc *scyllav2alpha1.ScyllaCluster, dc scyllav2alpha1.Datacenter, sd *scyllav1alpha1.ScyllaDatacenter) *scyllav2alpha1.DatacenterStatus {
	dcStatus := &scyllav2alpha1.DatacenterStatus{}

	if sd == nil {
		return dcStatus
	}

	var nodes, readyNodes, updatedNodes int32
	for _, rackSpec := range sd.Spec.Racks {
		for rackName, rack := range sd.Status.Racks {
			if rackName != rackSpec.Name {
				continue
			}

			if rack.Nodes != nil {
				nodes += *rack.Nodes
			}

			if rack.ReadyNodes != nil {
				readyNodes += *rack.ReadyNodes
			}

			if rack.UpdatedNodes != nil {
				updatedNodes += *rack.UpdatedNodes
			}

			rackStatus := &scyllav2alpha1.RackStatus{
				Name:                    rackSpec.Name,
				CurrentVersion:          rack.Image,
				Nodes:                   rack.Nodes,
				ReadyNodes:              rack.ReadyNodes,
				UpdatedNodes:            rack.UpdatedNodes,
				Stale:                   rack.Stale,
				ReplaceAddressFirstBoot: rack.ReplaceAddressFirstBoot,
			}

			for _, rackCondition := range rack.Conditions {
				condition := metav1.Condition{
					Status:  rackCondition.Status,
					Reason:  rackCondition.Reason,
					Message: rackCondition.Message,
				}
				switch rackCondition.Type {
				case scyllav1alpha1.RackConditionTypeUpgrading:
					condition.Type = scyllav2alpha1.RackConditionTypeUpgrading
				case scyllav1alpha1.RackConditionTypeNodeLeaving:
					condition.Type = scyllav2alpha1.RackConditionTypeNodeLeaving
				case scyllav1alpha1.RackConditionTypeNodeReplacing:
					condition.Type = scyllav2alpha1.RackConditionTypeNodeReplacing
				case scyllav1alpha1.RackConditionTypeNodeDecommissioning:
					condition.Type = scyllav2alpha1.RackConditionTypeNodeDecommissioning
				}
				meta.SetStatusCondition(&rackStatus.Conditions, condition)
			}

			dcStatus.Racks = append(dcStatus.Racks, *rackStatus)
		}
	}

	dcStatus.Name = dc.Name
	dcStatus.Nodes = pointer.Int32(nodes)
	dcStatus.ReadyNodes = pointer.Int32(readyNodes)
	dcStatus.UpdatedNodes = pointer.Int32(updatedNodes)

	if len(sd.Spec.Racks) > 0 {
		dcStatus.UpdatedVersion = sd.Status.Racks[sd.Spec.Racks[0].Name].Image
		dcStatus.CurrentVersion = sd.Status.Racks[sd.Spec.Racks[len(sd.Spec.Racks)-1].Name].Image
	}

	if sd.Status.ObservedGeneration != nil {
		dcStatus.Stale = pointer.Bool(*sd.Status.ObservedGeneration < sd.Generation)
	}

	dcStatus.Conditions = make([]metav1.Condition, 0, len(sd.Status.Conditions))
	for _, cond := range sd.Status.Conditions {
		dcStatus.Conditions = append(dcStatus.Conditions, *cond.DeepCopy())
	}

	return dcStatus
}
