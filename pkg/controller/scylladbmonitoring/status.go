package scylladbmonitoring

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// calculateStatus calculates the ScyllaDBMonitoring status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (smc *Controller) calculateStatus(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.ScyllaDBMonitoringStatus {
	status := sm.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(sm.Generation)

	return status
}

func (smc *Controller) updateStatus(ctx context.Context, currentSM *scyllav1alpha1.ScyllaDBMonitoring, status *scyllav1alpha1.ScyllaDBMonitoringStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSM.Status, status) {
		return nil
	}

	sm := currentSM.DeepCopy()
	sm.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaDBMonitoring", klog.KObj(sm))

	_, err := smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).UpdateStatus(ctx, sm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBMonitoring", klog.KObj(sm))

	return nil
}
