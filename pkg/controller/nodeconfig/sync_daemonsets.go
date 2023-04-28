// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
)

func (ncc *Controller) syncDaemonSet(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	daemonSets map[string]*appsv1.DaemonSet,
) error {
	scyllaUtilsImage := soc.Spec.ScyllaUtilsImage
	// FIXME: check that its not empty, emit event
	// FIXME: add webhook validation for the format
	requiredDaemonSets := []*appsv1.DaemonSet{
		makeNodeSetupDaemonSet(nc, ncc.operatorImage, scyllaUtilsImage),
	}

	err := controllerhelpers.Prune(
		ctx,
		requiredDaemonSets,
		daemonSets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ncc.scyllaClient.NodeConfigs().Delete,
		},
		ncc.eventRecorder)
	if err != nil {
		return fmt.Errorf("can't prune DaemonSet(s): %w", err)
	}

	for _, requiredDaemonSet := range requiredDaemonSets {
		if requiredDaemonSet == nil {
			continue
		}

		_, _, err := resourceapply.ApplyDaemonSet(ctx, ncc.kubeClient.AppsV1(), ncc.daemonSetLister, ncc.eventRecorder, requiredDaemonSet, resourceapply.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("can't apply daemonset: %w", err)
		}
	}

	return nil
}
