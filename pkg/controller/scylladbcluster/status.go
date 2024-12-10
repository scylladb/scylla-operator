// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (scc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1alpha1.ScyllaDBCluster, status *scyllav1alpha1.ScyllaDBClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sc := currentSC.DeepCopy()
	sc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaDBCluster", klog.KObj(sc))
	_, err := scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).UpdateStatus(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBCluster", klog.KObj(sc))

	return nil
}

// calculateStatus calculates the ScyllaDBCluster status.
func (scc *Controller) calculateStatus(sc *scyllav1alpha1.ScyllaDBCluster, datacentersMap map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter) *scyllav1alpha1.ScyllaDBClusterStatus {
	status := &scyllav1alpha1.ScyllaDBClusterStatus{}
	status.ObservedGeneration = pointer.Ptr(sc.Generation)

	var nodes, currentNodes, updatedNodes, readyNodes, availableNodes int32
	for _, dc := range sc.Spec.Datacenters {
		scyllaDatacenters := datacentersMap[dc.RemoteKubernetesClusterName]
		sdc := scyllaDatacenters[naming.ScyllaDBDatacenterName(sc, &dc)]
		dcStatus := *scc.calculateDatacenterStatus(&dc, sdc)

		status.Datacenters = append(status.Datacenters, dcStatus)

		if dcStatus.Nodes != nil {
			nodes += *dcStatus.Nodes
		}
		if dcStatus.CurrentNodes != nil {
			currentNodes += *dcStatus.CurrentNodes
		}
		if dcStatus.UpdatedNodes != nil {
			updatedNodes += *dcStatus.UpdatedNodes
		}
		if dcStatus.ReadyNodes != nil {
			readyNodes += *dcStatus.ReadyNodes
		}
		if dcStatus.AvailableNodes != nil {
			availableNodes += *dcStatus.AvailableNodes
		}
	}

	status.Nodes = pointer.Ptr(nodes)
	status.CurrentNodes = pointer.Ptr(currentNodes)
	status.UpdatedNodes = pointer.Ptr(updatedNodes)
	status.ReadyNodes = pointer.Ptr(readyNodes)
	status.AvailableNodes = pointer.Ptr(availableNodes)

	return status
}

func (scc *Controller) calculateDatacenterStatus(dc *scyllav1alpha1.ScyllaDBClusterDatacenter, sdc *scyllav1alpha1.ScyllaDBDatacenter) *scyllav1alpha1.ScyllaDBClusterDatacenterStatus {
	dcStatus := &scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
		Name: dc.Name,
	}

	if sdc == nil {
		return dcStatus
	}

	var nodes, currentNodes, updatedNodes, readyNodes, availableNodes int32
	for _, rackSpec := range sdc.Spec.Racks {
		for _, rs := range sdc.Status.Racks {
			if rs.Name != rackSpec.Name {
				continue
			}

			rackStatus := scyllav1alpha1.ScyllaDBClusterRackStatus{
				Name:           rackSpec.Name,
				CurrentVersion: pointer.Ptr(rs.CurrentVersion),
				UpdatedVersion: pointer.Ptr(rs.UpdatedVersion),
				Nodes:          rs.Nodes,
				CurrentNodes:   rs.CurrentNodes,
				UpdatedNodes:   rs.UpdatedNodes,
				ReadyNodes:     rs.ReadyNodes,
				AvailableNodes: rs.AvailableNodes,
				Stale:          rs.Stale,
			}

			if rs.Nodes != nil {
				nodes += *rs.Nodes
			}
			if rs.CurrentNodes != nil {
				currentNodes += *rs.CurrentNodes
			}
			if rs.UpdatedNodes != nil {
				updatedNodes += *rs.UpdatedNodes
			}
			if rs.ReadyNodes != nil {
				readyNodes += *rs.ReadyNodes
			}
			if rs.AvailableNodes != nil {
				availableNodes += *rs.AvailableNodes
			}

			dcStatus.Racks = append(dcStatus.Racks, rackStatus)
		}
	}

	dcStatus.RemoteNamespaceName = pointer.Ptr(sdc.Namespace)
	dcStatus.Nodes = pointer.Ptr(nodes)
	dcStatus.CurrentNodes = pointer.Ptr(currentNodes)
	dcStatus.UpdatedNodes = pointer.Ptr(updatedNodes)
	dcStatus.ReadyNodes = pointer.Ptr(readyNodes)
	dcStatus.AvailableNodes = pointer.Ptr(availableNodes)

	if sdc.Status.ObservedGeneration != nil {
		dcStatus.Stale = pointer.Ptr(*sdc.Status.ObservedGeneration < sdc.Generation)
	}

	return dcStatus
}
