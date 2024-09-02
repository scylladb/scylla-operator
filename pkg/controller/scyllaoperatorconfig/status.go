package scyllaoperatorconfig

import (
	"context"

	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (opc *Controller) updateStatus(ctx context.Context, currentSOC *scyllav1alpha1.ScyllaOperatorConfig, status *scyllav1alpha1.ScyllaOperatorConfigStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSOC.Status, status) {
		return nil
	}

	soc := currentSOC.DeepCopy()
	soc.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaOperatorConfig", klog.KObj(soc))

	_, err := opc.scyllaClient.ScyllaOperatorConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaOperatorConfig", klog.KObj(soc))

	return nil
}

// calculateStatus calculates the ScyllaOperatorConfig status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (opc *Controller) calculateStatus(soc *scyllav1alpha1.ScyllaOperatorConfig) *scyllav1alpha1.ScyllaOperatorConfigStatus {
	status := soc.Status.DeepCopy()
	status.ObservedGeneration = pointer.Ptr(soc.Generation)

	if len(soc.Spec.ScyllaUtilsImage) != 0 {
		status.ScyllaDBUtilsImage = pointer.Ptr(soc.Spec.ScyllaUtilsImage)
	} else {
		status.ScyllaDBUtilsImage = pointer.Ptr(configassests.Project.Operator.ScyllaDBUtilsImage)
	}

	if soc.Spec.UnsupportedBashToolsImageOverride != nil {
		status.BashToolsImage = pointer.Ptr(*soc.Spec.UnsupportedBashToolsImageOverride)
	} else {
		status.BashToolsImage = pointer.Ptr(configassests.Project.Operator.BashToolsImage)
	}

	if soc.Spec.UnsupportedGrafanaImageOverride != nil {
		status.GrafanaImage = pointer.Ptr(*soc.Spec.UnsupportedGrafanaImageOverride)
	} else {
		status.GrafanaImage = pointer.Ptr(configassests.Project.Operator.GrafanaImage)
	}

	if soc.Spec.UnsupportedPrometheusVersionOverride != nil {
		status.PrometheusVersion = pointer.Ptr(*soc.Spec.UnsupportedPrometheusVersionOverride)
	} else {
		status.PrometheusVersion = pointer.Ptr(configassests.Project.Operator.PrometheusVersion)
	}

	return status
}
