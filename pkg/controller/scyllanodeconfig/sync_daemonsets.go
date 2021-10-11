// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sncc *Controller) pruneDaemonSets(ctx context.Context, requiredDaemonSet *appsv1.DaemonSet, daemonSets map[string]*appsv1.DaemonSet) error {
	var errs []error
	for _, ds := range daemonSets {
		if ds.DeletionTimestamp != nil {
			continue
		}

		if ds.Name == requiredDaemonSet.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := sncc.kubeClient.AppsV1().DaemonSets(ds.Namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{
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

func (sncc *Controller) syncDaemonSets(
	ctx context.Context,
	snc *scyllav1alpha1.ScyllaNodeConfig,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	status *scyllav1alpha1.ScyllaNodeConfigStatus,
	daemonSets map[string]*appsv1.DaemonSet,
) error {
	requiredDaemonSet := resource.ScyllaNodeConfigDaemonSet(snc, sncc.operatorImage, soc.Spec.ScyllaUtilsImage)

	// Delete any excessive DaemonSets.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := sncc.pruneDaemonSets(ctx, requiredDaemonSet, daemonSets); err != nil {
		return fmt.Errorf("can't delete DaemonSet(s): %w", err)
	}

	updatedDaemonSet, _, err := resourceapply.ApplyDaemonSet(ctx, sncc.kubeClient.AppsV1(), sncc.daemonSetLister, sncc.eventRecorder, requiredDaemonSet)
	if err != nil {
		return fmt.Errorf("can't apply statefulset update: %w", err)
	}

	status.Updated.Desired = updatedDaemonSet.Status.DesiredNumberScheduled
	status.Updated.Actual = updatedDaemonSet.Status.CurrentNumberScheduled
	status.Updated.Ready = updatedDaemonSet.Status.NumberReady

	if nodeConfigAvailable(updatedDaemonSet, snc) {
		setCondition(status, newCondition(scyllav1alpha1.ScyllaNodeConfigAvailable, corev1.ConditionTrue, scyllav1alpha1.ScyllaNodeConfigReplicasReadyReason, "All required replicas are ready"))
	} else {
		setCondition(status, newCondition(scyllav1alpha1.ScyllaNodeConfigAvailable, corev1.ConditionFalse, scyllav1alpha1.ScyllaNodeConfigScalingReason, "Not all required replicas are ready"))
	}

	return nil
}
