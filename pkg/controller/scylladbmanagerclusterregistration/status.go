// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (smcrc *Controller) calculateStatus(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus {
	status := smcr.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(smcr.Generation)

	return status
}

func (smcrc *Controller) updateStatus(ctx context.Context, currentSMCR *scyllav1alpha1.ScyllaDBManagerClusterRegistration, status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSMCR.Status, status) {
		return nil
	}

	smcr := currentSMCR.DeepCopy()
	smcr.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))

	_, err := smcrc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(smcr.Namespace).UpdateStatus(ctx, smcr, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))

	return nil
}
