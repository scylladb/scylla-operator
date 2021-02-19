package cluster

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/actions"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Messages to display when experiencing an error.
	MessageHeadlessServiceSyncFailed = "Failed to sync Headless Service for cluster"
	MessageMemberServicesSyncFailed  = "Failed to sync MemberServices for cluster"
	MessageUpdateStatusFailed        = "Failed to update status for cluster: %+v"
	MessageCleanupFailed             = "Failed to clean up cluster resources"
	MessageClusterSyncFailed         = "Failed to sync cluster, got error: %+v"
)

// sync attempts to sync the given Scylla Cluster.
// NOTE: the Cluster Object is a DeepCopy. Modify at will.
func (cc *ClusterReconciler) sync(c *scyllav1.ScyllaCluster) error {
	ctx := log.WithNewTraceID(context.Background())
	logger := cc.Logger.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)
	logger.Info(ctx, "Starting reconciliation...")
	logger.Debug(ctx, "Cluster State", "object", c)

	// Before syncing, ensure that all StatefulSets are up-to-date
	stale, err := util.AreStatefulSetStatusesStale(ctx, c, cc.Client)
	if err != nil {
		return errors.Wrap(err, "failed to check sts staleness")
	}
	if stale {
		logger.Debug(ctx, "StatefulSets are not ready!")
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
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf(MessageUpdateStatusFailed, err))
		return errors.Wrap(err, "failed to update status")
	}

	// Calculate and execute next action
	if act, err := cc.nextAction(ctx, c); err != nil {
		cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf(MessageUpdateStatusFailed, err))
		return errors.Wrap(err, "failed to determine next action")
	} else if act != nil {
		s := actions.NewState(cc.Client, cc.KubeClient, cc.Recorder)
		logger.Debug(ctx, "New action", "name", act.Name())
		if err := act.Execute(ctx, s); err != nil {
			cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf(MessageClusterSyncFailed, errors.Cause(err)))
			return err
		}
	}

	return nil
}

func (cc *ClusterReconciler) nextAction(ctx context.Context, cluster *scyllav1.ScyllaCluster) (actions.Action, error) {
	logger := cc.Logger.With("cluster", cluster.Namespace+"/"+cluster.Name, "resourceVersion", cluster.ResourceVersion)

	// Check if any rack isn't created
	for _, rack := range cluster.Spec.Datacenter.Racks {
		// For each rack, check if a status entry exists
		if _, ok := cluster.Status.Racks[rack.Name]; !ok {
			logger.Info(ctx, "Next Action: Create rack", "name", rack.Name)
			return actions.NewRackCreateAction(rack, cluster, cc.OperatorImage), nil
		}
	}

	////////////////////////////////////////////
	// Check if there are actions in progress //
	////////////////////////////////////////////

	// Check if there is a scale-down in progress
	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := cluster.Status.Racks[rack.Name]
		if scyllav1.IsRackConditionTrue(&rackStatus, scyllav1.RackConditionTypeMemberReplacing) {
			// Perform node replace
			logger.Info(ctx, "Next Action: Node replace rack", "name", rack.Name)
			return actions.NewRackReplaceNodeAction(rack, cluster, logger.Named("replace")), nil
		}

		if scyllav1.IsRackConditionTrue(&rackStatus, scyllav1.RackConditionTypeMemberLeaving) {
			// Resume scale down
			logger.Info(ctx, "Next Action: Scale-Down rack", "name", rack.Name)
			return actions.NewRackScaleDownAction(rack, cluster), nil
		}
	}

	// Check if there is an upgrade in progress
	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus := cluster.Status.Racks[rack.Name]
		if cluster.Status.Upgrade != nil ||
			scyllav1.IsRackConditionTrue(&rackStatus, scyllav1.RackConditionTypeUpgrading) {
			return actions.NewClusterVersionUpgradeAction(cluster, logger), nil
		}
	}

	// Check if there is an sidecar upgrade in progress
	for _, rack := range cluster.Spec.Datacenter.Racks {
		update, container, err := cc.sidecarUpdateNeeded(ctx, rack, cluster)
		if err != nil {
			return nil, errors.Wrap(err, "check if sidecar update is needed")
		}
		if update {
			return actions.NewSidecarUpgrade(cluster, container, logger), nil
		}
	}

	///////////////////////////////////////////////////////////////////////////////////////
	// At this point, the cluster is in a stable state and ready to start another action //
	///////////////////////////////////////////////////////////////////////////////////////

	// Check if any rack needs to scale down
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if rack.Members < cluster.Status.Racks[rack.Name].Members {
			logger.Info(ctx, "Next Action: Scale-Down rack", "name", rack.Name)
			return actions.NewRackScaleDownAction(rack, cluster), nil
		}
	}

	// Check if any rack needs to scale up
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if rack.Members > cluster.Status.Racks[rack.Name].Members {
			logger.Info(ctx, "Next Action: Scale-Up rack", "name", rack.Name)
			return actions.NewRackScaleUpAction(rack, cluster), nil
		}
	}

	// Check if any rack needs version upgrade
	for _, rack := range cluster.Spec.Datacenter.Racks {
		if cluster.Spec.Version != cluster.Status.Racks[rack.Name].Version {
			logger.Info(ctx, "Next Action: Upgrade rack", "name", rack.Name)
			return actions.NewClusterVersionUpgradeAction(cluster, logger), nil
		}
	}

	// Nothing to do
	return nil, nil
}

func (cc *ClusterReconciler) sidecarUpdateNeeded(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) (bool, corev1.Container, error) {
	desiredSts := resource.StatefulSetForRack(rack, cluster, cc.OperatorImage)
	desiredIdx, err := naming.FindSidecarInjectorContainer(desiredSts.Spec.Template.Spec.InitContainers)
	if err != nil {
		return false, corev1.Container{}, errors.Wrap(err, "find sidecar container in desired sts")
	}

	actualSts, err := util.GetStatefulSetForRack(ctx, rack, cluster, cc.KubeClient)
	if err != nil {
		return false, corev1.Container{}, errors.Wrap(err, "fetch rack sts")
	}

	actualIdx, err := naming.FindSidecarInjectorContainer(actualSts.Spec.Template.Spec.InitContainers)
	if err != nil {
		return false, corev1.Container{}, errors.Wrap(err, "find sidecar container in actual sts")
	}

	actualSidecarContainer := actualSts.Spec.Template.Spec.InitContainers[actualIdx]
	desiredSidecarContainer := desiredSts.Spec.Template.Spec.InitContainers[desiredIdx]

	cc.Logger.Debug(ctx, "Sidecar update", "rack", rack.Name, "desired_image", desiredSidecarContainer.Image, "actual_image", actualSidecarContainer.Image)

	return actualSidecarContainer.Image != desiredSidecarContainer.Image, desiredSidecarContainer, nil

}
