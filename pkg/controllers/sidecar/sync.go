package sidecar

import (
	"context"
	"os/exec"

	"github.com/scylladb/scylla-operator/pkg/scyllaclient"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	corev1 "k8s.io/api/core/v1"
)

func (mc *MemberReconciler) sync(ctx context.Context, memberService *corev1.Service) (bool, error) {
	// Check if member must decommission
	if decommission, ok := memberService.Labels[naming.DecommissionLabel]; ok {
		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			return true, errors.Wrapf(err, "Can't get node ip")
		}
		// Check if member has already decommissioned
		if decommission == naming.LabelValueTrue {
			mc.logger.Debug(ctx, "Node is already decommissioned")
			return false, nil
		}
		opMode, err := mc.scyllaClient.OperationMode(ctx, host.String())
		if err != nil {
			return true, errors.Wrapf(err, "error on getting node operation mode: %s", errors.WithStack(err))
		}
		switch opMode {
		case scyllaclient.OperationalModeLeaving, scyllaclient.OperationalModeDecommissioning, scyllaclient.OperationalModeDraining:
			// If node is leaving/draining/decommissioning, keep retrying
			return true, nil
		case scyllaclient.OperationalModeDrained:
			mc.logger.Info(ctx, "Node is in DRAINED state, restarting scylla to make it decommissionable")
			// TBD: Label pod/service that it is in restarting state to avoid livenes probe race
			_, err := exec.Command("supervisorctl", "restart", "scylla").Output()
			if err != nil {
				return true, errors.Wrapf(err, "restarting node has failed: %s", err)
			}
			mc.logger.Info(ctx, "Scylla successfully restarted")
			return true, nil
		case scyllaclient.OperationalModeNormal:
			// Node can be in NORMAL mode while still starting up
			// Last thing that scylla is doing as part of startup process is brining native transport up
			// So we check if native port is up as sign that it is not loading
			nativeUp, err := mc.scyllaClient.IsNativeTransportEnabled(ctx, host.String())
			if err != nil {
				return true, errors.Wrapf(err, "failed to get native transport status: %s", errors.WithStack(err))
			}
			if !nativeUp {
				mc.logger.Info(ctx, "Node native transport is down, it is sign that node is starting up")
				return true, nil
			}
			// Decommission node only if it is in normal mode and native transport is up
			if decommissionErr := mc.scyllaClient.Decommission(ctx, host.String()); decommissionErr != nil {
				// Decommission is long running task, so request fails due to the timeout in most cases
				// To do not raise error when it is happening we check opMode
				opMode, err = mc.scyllaClient.OperationMode(ctx, host.String())
				if err == nil && (opMode.IsDecommissioned() || opMode.IsLeaving() || opMode.IsDecommissioning()) {
					return true, nil
				}
				return true, errors.Wrapf(decommissionErr, "error during decommission: %s", errors.WithStack(decommissionErr))
			}
		case scyllaclient.OperationalModeJoining:
			// If node is joining we need to wait till it reaches Normal state and then decommission it
			mc.logger.Info(ctx, "Can't decommission joining node")
			return true, nil
		case scyllaclient.OperationalModeDecommissioned:
			mc.logger.Info(ctx, "Node is decommissioned. Skip decommission")
		default:
			return true, errors.Errorf("Unexpected node operation mode: %s", opMode)
		}
		// Update Label to signal that decommission has completed
		old := memberService.DeepCopy()
		memberService.Labels[naming.DecommissionLabel] = naming.LabelValueTrue
		if err := util.PatchService(ctx, old, memberService, mc.kubeClient); err != nil {
			return true, errors.Wrap(err, "error patching MemberService")
		}
	}
	return false, nil
}
