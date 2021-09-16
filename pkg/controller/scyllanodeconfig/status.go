// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (sncc *Controller) calculateStatus(snc *scyllav1alpha1.ScyllaNodeConfig, daemonSets map[string]*appsv1.DaemonSet) *scyllav1alpha1.ScyllaNodeConfigStatus {
	status := snc.Status.DeepCopy()
	status.ObservedGeneration = snc.Generation

	for _, ds := range daemonSets {
		if snc.Name == ds.Name {
			status.Current.Ready = ds.Status.NumberReady
			status.Current.Desired = ds.Status.DesiredNumberScheduled
			status.Current.Actual = ds.Status.CurrentNumberScheduled

			if nodeConfigAvailable(ds, snc) {
				setCondition(status, newCondition(scyllav1alpha1.ScyllaNodeConfigAvailable, corev1.ConditionTrue, scyllav1alpha1.ScyllaNodeConfigReplicasReadyReason, "All required replicas are ready"))
			} else {
				setCondition(status, newCondition(scyllav1alpha1.ScyllaNodeConfigAvailable, corev1.ConditionFalse, scyllav1alpha1.ScyllaNodeConfigScalingReason, "Not all required replicas are ready"))
			}
		}
	}

	return status
}

func (sncc *Controller) updateStatus(ctx context.Context, currentNodeConfig *scyllav1alpha1.ScyllaNodeConfig, status *scyllav1alpha1.ScyllaNodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(&currentNodeConfig.Status, status) {
		return nil
	}

	soc := currentNodeConfig.DeepCopy()
	soc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaNodeConfig", klog.KObj(soc))

	_, err := sncc.scyllaClient.ScyllaNodeConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaNodeConfig", klog.KObj(soc))

	return nil
}
