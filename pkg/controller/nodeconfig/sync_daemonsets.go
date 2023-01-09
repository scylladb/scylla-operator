// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) pruneDaemonSets(ctx context.Context, requiredDaemonSet *appsv1.DaemonSet, daemonSets map[string]*appsv1.DaemonSet) error {
	var errs []error
	for _, ds := range daemonSets {
		if ds.DeletionTimestamp != nil {
			continue
		}

		if requiredDaemonSet != nil && ds.Name == requiredDaemonSet.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.AppsV1().DaemonSets(ds.Namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &ds.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncDaemonSet(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	status *scyllav1alpha1.NodeConfigStatus,
	daemonSets map[string]*appsv1.DaemonSet,
) error {
	scyllaUtilsImage := soc.Spec.ScyllaUtilsImage
	// FIXME: check that its not empty, emit event
	// FIXME: add webhook validation for the format
	requiredDaemonSet := makeNodeConfigDaemonSet(nc, ncc.operatorImage, scyllaUtilsImage)

	// Delete any excessive DaemonSets.
	// Delete has to be the first action to avoid getting stuck on quota.
	err := ncc.pruneDaemonSets(ctx, requiredDaemonSet, daemonSets)
	if err != nil {
		return fmt.Errorf("can't delete DaemonSet(s): %w", err)
	}

	if requiredDaemonSet != nil {
		updatedDaemonSet, _, err := resourceapply.ApplyDaemonSet(ctx, ncc.kubeClient.AppsV1(), ncc.daemonSetLister, ncc.eventRecorder, requiredDaemonSet, resourceapply.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("can't apply statefulset update: %w", err)
		}

		status, err = ncc.calculateStatus(
			nc,
			map[string]*appsv1.DaemonSet{
				updatedDaemonSet.Name: updatedDaemonSet,
			},
			scyllaUtilsImage,
		)
		if err != nil {
			return fmt.Errorf("can't calculate updated status: %w", err)
		}
	}

	return nil
}
