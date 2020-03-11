package cluster

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/actions"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Messages to display when experiencing an error.
	MessageHeadlessServiceSyncFailed = "Failed to sync Headless Service for cluster"
	MessageMemberServicesSyncFailed  = "Failed to sync MemberServices for cluster"
	MessageUpdateStatusFailed        = "Failed to update status for cluster"
	MessageCleanupFailed             = "Failed to clean up cluster resources"
	MessageClusterSyncFailed         = "Failed to sync cluster, got error: %+v"
)

// sync attempts to sync the given Scylla Cluster.
// NOTE: the Cluster Object is a DeepCopy. Modify at will.
func (cc *ClusterController) sync(c *scyllav1alpha1.Cluster) error {
	ctx := log.WithNewTraceID(context.Background())
	logger := cc.logger.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)
	logger.Info(ctx, "Starting reconciliation...")
	logger.Debug(ctx, "Cluster State", "object", c)

	// Before syncing, ensure that all StatefulSets are up-to-date
	stale, err := util.AreStatefulSetStatusesStale(ctx, c, cc.Client)
	if err != nil {
		return errors.Wrap(err, "failed to check sts staleness")
	}
	if stale {
		return nil
	}
	logger.Debug(ctx, "All StatefulSets are up-to-date!")

	// Cleanup Cluster resources
	if err := cc.cleanup(ctx, c); err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, MessageCleanupFailed)
	}

	// Sync Headless Service for Cluster
	if err := cc.syncClusterHeadlessService(ctx, c); err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, MessageHeadlessServiceSyncFailed)
		return errors.Wrap(err, "failed to sync headless service")
	}

	// Sync Cluster Member Services
	if err := cc.syncMemberServices(ctx, c); err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, MessageMemberServicesSyncFailed)
		return errors.Wrap(err, "failed to sync member service")
	}

	// Update Status
	logger.Info(ctx, "Calculating cluster status...")
	if err := cc.updateStatus(ctx, c); err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, MessageUpdateStatusFailed)
		return errors.Wrap(err, "failed to update status")
	}

	// Calculate and execute next action
	if act := cc.nextAction(ctx, c); act != nil {
		s := actions.NewState(cc.Client, cc.KubeClient, cc.Recorder)
		logger.Debug(ctx, "New action", "name", act.Name())
		err = act.Execute(ctx, s)
	}

	if err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf(MessageClusterSyncFailed, errors.Cause(err)))
	}

	return nil
}

func (cc *ClusterController) nextAction(ctx context.Context, cluster *scyllav1alpha1.Cluster) actions.Action {
	logger := cc.logger.With("cluster", cluster.Namespace+"/"+cluster.Name, "resourceVersion", cluster.ResourceVersion)

	// Check if any rack isn't created
	for _, rack := range cluster.Spec.Datacenter.Racks {
		// For each rack, check if a status entry exists
		if _, ok := cluster.Status.Racks[rack.Name]; !ok {
			logger.Info(ctx, "Next Action: Create rack", "name", rack.Name)
			return actions.NewRackCreateAction(rack, cluster, cc.OperatorImage)
		}
	}

	////////////////////////////////////////////
	// Check if there are actions in progress //
	////////////////////////////////////////////

	// Check if there is a scale-down in progress
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if scyllav1alpha1.IsRackConditionTrue(cluster.Status.Racks[rack.Name], scyllav1alpha1.RackConditionTypeMemberLeaving) {
			// Resume scale down
			logger.Info(ctx, "Next Action: Scale-Down rack", "name", rack.Name)
			return actions.NewRackScaleDownAction(rack, cluster)
		}
	}

	// Check if there is an upgrade in progress
	differentVersionRacks := 0
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if scyllav1alpha1.IsRackConditionTrue(cluster.Status.Racks[rack.Name], scyllav1alpha1.RackConditionTypeUpgrading) {
			logger.Info(ctx, "Rack is upgrading. Waiting until the upgrade is complete.", "name", rack.Name)
			return nil
		}
		if cluster.Status.Racks[rack.Name].Version != cluster.Spec.Version {
			differentVersionRacks += 1
		}
	}

	logger.Debug(ctx, "Different version racks", "count", differentVersionRacks)

	// Check that all racks are ready before taking any action
	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := cluster.Status.Racks[rack.Name]
		if rackStatus.Members != rackStatus.ReadyMembers {
			logger.Info(ctx, "Rack is not ready", "name", rack.Name,
				"members", rackStatus.Members, "ready_members", rackStatus.ReadyMembers)
			return nil
		}
	}

	// Continue the cluster upgrade if there is one in progress.
	if differentVersionRacks != 0 && differentVersionRacks != len(cluster.Spec.Datacenter.Racks) {
		logger.Info(ctx, "Only %d/%d racks are upgraded, continuing with version upgrade",
			differentVersionRacks, len(cluster.Spec.Datacenter.Racks))
		return actions.NewClusterVersionUpgradeAction(cluster, logger)
	}

	///////////////////////////////////////////////////////////////////////////////////////
	// At this point, the cluster is in a stable state and ready to start another action //
	///////////////////////////////////////////////////////////////////////////////////////

	// Check if any rack needs to scale down
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if rack.Members < cluster.Status.Racks[rack.Name].Members {
			logger.Info(ctx, "Next Action: Scale-Down rack", "name", rack.Name)
			return actions.NewRackScaleDownAction(rack, cluster)
		}
	}

	// Check if any rack needs to scale up
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if rack.Members > cluster.Status.Racks[rack.Name].Members {
			logger.Info(ctx, "Next Action: Scale-Up rack", "name", rack.Name)
			return actions.NewRackScaleUpAction(rack, cluster)
		}
	}

	// Check if any rack needs version upgrade
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if cluster.Spec.Version != cluster.Status.Racks[rack.Name].Version {
			logger.Info(ctx, "Next Action: Upgrade rack", "name", rack.Name)
			return actions.NewClusterVersionUpgradeAction(cluster, logger)
		}
	}

	// Nothing to do
	return nil
}
