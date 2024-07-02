// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) syncDaemonSet(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	daemonSets map[string]*appsv1.DaemonSet,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

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
			DeleteFunc: ncc.kubeClient.AppsV1().DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).Delete,
		},
		ncc.eventRecorder)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune DaemonSet(s): %w", err)
	}

	var errs []error
	for _, ds := range requiredDaemonSets {
		if ds == nil {
			continue
		}

		updated, changed, err := resourceapply.ApplyDaemonSet(ctx, ncc.kubeClient.AppsV1(), ncc.daemonSetLister, ncc.eventRecorder, ds, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, daemonSetControllerProgressingCondition, ds, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply daemonset: %w", err))
			continue
		}

		rolledOut, err := controllerhelpers.IsDaemonSetRolledOut(updated)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't check if daemonset is rolled out: %w", err))
		}

		if !rolledOut {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               daemonSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForDaemonSetRollOut",
				Message:            fmt.Sprintf("Waiting for DaemonSet %q to roll out.", naming.ObjRef(ds)),
				ObservedGeneration: nc.Generation,
			})
		}
	}

	return progressingConditions, utilerrors.NewAggregate(errs)
}
