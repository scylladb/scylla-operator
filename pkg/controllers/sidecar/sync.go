package sidecar

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/util/network"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
)

func (mc *MemberReconciler) sync(ctx context.Context, memberService *corev1.Service) error {
	// Check if member must decommission
	if decommission, ok := memberService.Labels[naming.DecommissionLabel]; ok {
		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			return errors.Wrapf(err, "error during decommission")
		}
		// Check if member has already decommissioned
		if decommission == naming.LabelValueTrue {
			return nil
		}
		// Else, decommission member
		if err := mc.scyllaClient.Decommission(ctx, host.String()); err != nil {
			mc.logger.Error(ctx, "Error during decommission", "error", errors.WithStack(err))
		}
		// Confirm memberService has been decommissioned
		if opMode, err := mc.scyllaClient.OperationMode(ctx, host.String()); err != nil || !opMode.IsDecommisioned() {
			return errors.Wrapf(err, "error during decommission, operation mode: %s", opMode)
		}
		// Update Label to signal that decommission has completed
		old := memberService.DeepCopy()
		memberService.Labels[naming.DecommissionLabel] = naming.LabelValueTrue
		if err := util.PatchService(ctx, old, memberService, mc.kubeClient); err != nil {
			return errors.Wrap(err, "error patching MemberService")
		}
	}

	return nil
}
