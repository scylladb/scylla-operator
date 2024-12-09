// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (rkcc *Controller) updateStatus(ctx context.Context, currentRKC *scyllav1alpha1.RemoteKubernetesCluster, status *scyllav1alpha1.RemoteKubernetesClusterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentRKC.Status, status) {
		return nil
	}

	rkc := currentRKC.DeepCopy()
	rkc.Status = *status

	klog.V(2).InfoS("Updating status", "RemoteKubernetesCluster", klog.KObj(rkc))
	_, err := rkcc.scyllaClient.RemoteKubernetesClusters().UpdateStatus(ctx, rkc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "RemoteKubernetesCluster", klog.KObj(rkc))

	return nil
}

func (rkcc *Controller) calculateStatus(rkc *scyllav1alpha1.RemoteKubernetesCluster) *scyllav1alpha1.RemoteKubernetesClusterStatus {
	status := rkc.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(rkc.Generation)

	return status
}
