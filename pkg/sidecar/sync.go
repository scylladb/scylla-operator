package sidecar

import (
	"context"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/yanniszark/go-nodetool/nodetool"
	corev1 "k8s.io/api/core/v1"
)

func (mc *MemberController) sync(ctx context.Context, memberService *corev1.Service) error {
	// Check if member must decommission
	if decommission, ok := memberService.Labels[naming.DecommissionLabel]; ok {
		// Check if member has already decommissioned
		if decommission == naming.LabelValueTrue {
			return nil
		}
		// Else, decommission member
		if err := mc.nodetool.Decommission(); err != nil {
			mc.logger.Error(ctx, "Error during decommission", "error", errors.WithStack(err))
		}
		// Confirm memberService has been decommissioned
		if opMode, err := mc.nodetool.OperationMode(); err != nil || opMode != nodetool.NodeOperationModeDecommissioned {
			return errors.Wrapf(err, "error during decommission, operation mode: %s", opMode)
		}
		// Update Label to signal that decommission has completed
		old := memberService.DeepCopy()
		memberService.Labels[naming.DecommissionLabel] = naming.LabelValueTrue
		if err := util.PatchService(old, memberService, mc.kubeClient); err != nil {
			return errors.Wrap(err, "error patching MemberService")
		}
	}

	return nil
}
