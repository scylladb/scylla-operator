package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// UpdateStatus updates the status of the given Scylla Cluster.
// It doesn't post the result to the API Server yet.
// That will be done at the end of the sync loop.
func (cc *ClusterReconciler) updateStatus(ctx context.Context, cluster *scyllav1.ScyllaCluster) error {
	if cluster.Status.Racks == nil {
		cluster.Status.Racks = map[string]scyllav1.RackStatus{}
	}

	sts := &appsv1.StatefulSet{}

	// Find rack which are no longer specified
	var unknownRacks []string
	for rackName := range cluster.Status.Racks {
		found := false
		for _, rack := range cluster.Spec.Datacenter.Racks {
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
		delete(cluster.Status.Racks, rackName)
	}

	// Update each rack status
	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := scyllav1.RackStatus{
			ReplaceAddressFirstBoot: make(map[string]string),
		}
		// ReplaceAddress may keep address of non-existing service.
		// Copy it from previous cluster status to persist this state.
		if existingStatus, ok := cluster.Status.Racks[rack.Name]; ok {
			for k, v := range existingStatus.ReplaceAddressFirstBoot {
				rackStatus.ReplaceAddressFirstBoot[k] = v
			}
		}
		// Get corresponding StatefulSet from lister
		err := cc.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(rack, cluster), cluster.Namespace), sts)
		// If it wasn't found, clear rack status and continue
		if apierrors.IsNotFound(err) {
			delete(cluster.Status.Racks, rack.Name)
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
			rackStatus.Version, err = naming.ScyllaVersion(sts.Spec.Template.Spec.Containers)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		// Update Upgrading condition
		desiredRackVersion := cluster.Spec.Version
		actualRackVersion := rackStatus.Version
		if desiredRackVersion != actualRackVersion {
			cc.Logger.Info(ctx, "Rack should be upgraded", "actual_version", actualRackVersion, "desired_version", desiredRackVersion, "rack", rack.Name)
			scyllav1.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeUpgrading)
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
				scyllav1.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeMemberLeaving)
				// Sanity check. Only the last member should be decommissioning.
				index, err := naming.IndexFromName(svc.Name)
				if err != nil {
					return err
				}
				if index != rackStatus.Members-1 {
					return errors.New(fmt.Sprintf("only last member of each rack should be decommissioning, but %d-th member of %s found decommissioning while rack had %d members", index, rack.Name, rackStatus.Members))
				}
			}
			if replaceAddr, ok := svc.Labels[naming.ReplaceLabel]; ok {
				if _, ok := svc.Labels[naming.SeedLabel]; ok {
					return errors.New("seed node replace is not supported")
				}
				if replaceAddr == "" {
					rackStatus.ReplaceAddressFirstBoot[svc.Name] = svc.Spec.ClusterIP
				}
				cc.Logger.Info(ctx, "Rack member is being replaced", "rack", rack.Name, "member", svc.Name)
				scyllav1.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeMemberReplacing)
			}
		}

		// Update Status for Rack
		cluster.Status.Racks[rack.Name] = rackStatus
	}

	return nil
}
