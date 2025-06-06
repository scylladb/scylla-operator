// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (smtc *Controller) calculateStatus(smt *scyllav1alpha1.ScyllaDBManagerTask) *scyllav1alpha1.ScyllaDBManagerTaskStatus {
	status := smt.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(smt.Generation)

	return status
}

func (smtc *Controller) updateStatus(ctx context.Context, currentSMT *scyllav1alpha1.ScyllaDBManagerTask, status *scyllav1alpha1.ScyllaDBManagerTaskStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSMT.Status, status) {
		return nil
	}

	smt := currentSMT.DeepCopy()
	smt.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaDBManagerTask", klog.KObj(smt))

	_, err := smtc.scyllaClient.ScyllaDBManagerTasks(smt.Namespace).UpdateStatus(ctx, smt, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBManagerTask", klog.KObj(smt))

	return nil
}
