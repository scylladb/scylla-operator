// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (ncc *Controller) calculateStatus(nc *scyllav1alpha1.NodeConfig, daemonSets map[string]*appsv1.DaemonSet, scyllaUtilsImage string) (*scyllav1alpha1.NodeConfigStatus, error) {
	status := nc.Status.DeepCopy()
	status.ObservedGeneration = nc.Generation

	// TODO: We need to restructure the pattern so status update already know the desired object and we don't
	// 		 construct it twice.
	requiredDaemonSet := makeNodeConfigDaemonSet(nc, ncc.operatorImage, scyllaUtilsImage)
	ds, found := daemonSets[requiredDaemonSet.Name]
	if !found {
		klog.V(4).InfoS("Existing DaemonSet not found", "DaemonSet", klog.KObj(ds))
		return status, nil
	}

	reconciled, err := controllerhelpers.IsDaemonSetRolledOut(ds)
	if err != nil {
		return nil, fmt.Errorf("can't determine is a daemonset %q is reconiled: %w", naming.ObjRef(ds), err)
	}

	cond := &scyllav1alpha1.NodeConfigCondition{
		Type: scyllav1alpha1.NodeConfigReconciledConditionType,
	}

	if reconciled {
		cond.Status = corev1.ConditionTrue
		cond.Reason = "FullyReconciledAndUp"
		cond.Message = "All operands are reconciled and available."
	} else {
		cond.Status = corev1.ConditionFalse
		cond.Reason = "DaemonSetNotRolledOut"
		cond.Message = "DaemonSet isn't reconciled and fully rolled out yet."
	}

	controllerhelpers.EnsureNodeConfigCondition(status, cond)

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
