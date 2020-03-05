package cluster

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// UpdateStatus updates the status of the given Scylla Cluster.
// It doesn't post the result to the API Server yet.
// That will be done at the end of the sync loop.
func (cc *ClusterController) updateStatus(ctx context.Context, cluster *scyllav1alpha1.Cluster) error {
	clusterStatus := scyllav1alpha1.ClusterStatus{
		Racks: map[string]*scyllav1alpha1.RackStatus{},
	}
	sts := &appsv1.StatefulSet{}

	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := &scyllav1alpha1.RackStatus{}

		// Get corresponding StatefulSet from lister
		err := cc.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(rack, cluster), cluster.Namespace), sts)
		// If it wasn't found, continue
		if apierrors.IsNotFound(err) {
			continue
		}
		// If we got a different error, requeue and log it
		if err != nil {
			return errors.Wrap(err, "failed to get statefulset")
		}

		// Update Members
		rackStatus.Members = *sts.Spec.Replicas
		// Update ReadyMembers
		rackStatus.ReadyMembers = sts.Status.ReadyReplicas

		// Update Scaling Down condition
		svcList, err := util.GetMemberServicesForRack(ctx, rack, cluster, cc.Client)
		if err != nil {
			return fmt.Errorf("error trying to get Pods for rack %s", rack.Name)
		}

		for _, svc := range svcList {
			// Check if there is a decommission in progress
			if _, ok := svc.Labels[naming.DecommissionLabel]; ok {
				// Add MemberLeaving Condition to rack status
				scyllav1alpha1.SetRackCondition(rackStatus, scyllav1alpha1.RackConditionTypeMemberLeaving)
				// Sanity check. Only the last member should be decommissioning.
				index, err := naming.IndexFromName(svc.Name)
				if err != nil {
					return err
				}
				if index != rackStatus.Members-1 {
					return errors.New(fmt.Sprintf("only last member of each rack should be decommissioning, but %d-th member of %s found decommissioning while rack had %d members", index, rack.Name, rackStatus.Members))
				}
			}
		}

		// Update Status for Rack
		clusterStatus.Racks[rack.Name] = rackStatus
	}
	cluster.Status = clusterStatus

	return nil
}
