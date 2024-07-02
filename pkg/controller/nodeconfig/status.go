// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (ncc *Controller) calculateStatus(nc *scyllav1alpha1.NodeConfig) (*scyllav1alpha1.NodeConfigStatus, error) {
	status := nc.Status.DeepCopy()
	status.ObservedGeneration = nc.Generation

	return status, nil
}

func (ncc *Controller) updateStatus(ctx context.Context, currentNodeConfig *scyllav1alpha1.NodeConfig, status *scyllav1alpha1.NodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(&currentNodeConfig.Status, status) {
		return nil
	}

	soc := currentNodeConfig.DeepCopy()
	soc.Status = *status

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(soc))

	_, err := ncc.scyllaClient.NodeConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(soc))

	return nil
}
