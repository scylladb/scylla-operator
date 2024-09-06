// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (nsc *Controller) calculateStatus(nc *v1alpha1.NodeConfig) *v1alpha1.NodeConfigStatus {
	status := nc.Status.DeepCopy()
	status.ObservedGeneration = nc.Generation

	return status
}

func (nsc *Controller) updateStatus(ctx context.Context, currentNC *v1alpha1.NodeConfig, status *v1alpha1.NodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(currentNC.Status, status) {
		return nil
	}

	nc := currentNC.DeepCopy()
	nc.Status = *status

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	_, err := nsc.scyllaClient.NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update node config status %q: %w", nsc.nodeConfigName, err)
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(currentNC), "Node", nsc.nodeName)

	return nil
}
