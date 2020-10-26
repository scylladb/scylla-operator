package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// UpdateStatus updates the status of the given Scylla Cluster.
// It doesn't post the result to the API Server yet.
// That will be done at the end of the sync loop.
func (cc *ClusterReconciler) updateStatus(ctx context.Context, cluster *scyllav1alpha1.ScyllaCluster) error {
	cluster.Status.Racks = map[string]scyllav1alpha1.RackStatus{}

	sts := &appsv1.StatefulSet{}

	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := scyllav1alpha1.RackStatus{}

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

		// Update Rack Version
		if rackStatus.Members == 0 {
			rackStatus.Version = cluster.Spec.Version
		} else {
			firstRackMemberName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(rack, cluster))
			firstRackMember := &corev1.Pod{}
			err := cc.Get(ctx, naming.NamespacedName(firstRackMemberName, cluster.Namespace), firstRackMember)
			if err != nil {
				return errors.WithStack(err)
			}
			idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
			if err != nil {
				return errors.WithStack(err)
			}
			rackStatus.Version, err = naming.ImageToVersion(firstRackMember.Spec.Containers[idx].Image)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		// Update Upgrading condition
		idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
		if err != nil {
			return errors.WithStack(err)
		}
		desiredRackVersion, err := naming.ImageToVersion(sts.Spec.Template.Spec.Containers[idx].Image)
		if err != nil {
			return errors.WithStack(err)
		}
		actualRackVersion := rackStatus.Version
		if desiredRackVersion != actualRackVersion {
			cc.Logger.Info(ctx, "Rack should be upgraded", "actual_version", actualRackVersion, "desired_version", desiredRackVersion, "rack", rack.Name)
			scyllav1alpha1.SetRackCondition(&rackStatus, scyllav1alpha1.RackConditionTypeUpgrading)
		}

		// Update Scaling Down condition
		svcList, err := util.GetMemberServicesForRack(ctx, rack, cluster, cc.Client)
		if err != nil {
			return fmt.Errorf("error trying to get Pods for rack %s", rack.Name)
		}

		for _, svc := range svcList {
			// Check if there is a decommission in progress
			if _, ok := svc.Labels[naming.DecommissionLabel]; ok {
				// Add MemberLeaving Condition to rack status
				scyllav1alpha1.SetRackCondition(&rackStatus, scyllav1alpha1.RackConditionTypeMemberLeaving)
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
		cluster.Status.Racks[rack.Name] = rackStatus
	}

	return nil
}
