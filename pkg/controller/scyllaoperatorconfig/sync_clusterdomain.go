package scyllaoperatorconfig

import (
	"context"
	"fmt"

	osscyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (socc *Controller) syncClusterDomain(
	ctx context.Context,
	soc *osscyllav1alpha1.ScyllaOperatorConfig,
	status *osscyllav1alpha1.ScyllaOperatorConfigStatus,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	if soc.Spec.ConfiguredClusterDomain != nil {
		if len(*soc.Spec.ConfiguredClusterDomain) == 0 {
			return progressingConditions, fmt.Errorf("unexpected empty cluster domain")
		}

		status.ClusterDomain = pointer.Ptr(*soc.Spec.ConfiguredClusterDomain)
	} else {
		clusterDomain, err := socc.getClusterDomainFunc(ctx)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't get cluster domain: %w", err)
		}

		status.ClusterDomain = pointer.Ptr(clusterDomain)
	}

	return progressingConditions, nil
}
