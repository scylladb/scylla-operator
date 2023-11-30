package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (scc *Controller) syncEndpointSlices(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	endpointSlices map[string]*discoveryv1.EndpointSlice,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredEndpointSlices, err := MakeEndpointSlices(sc, services, scc.podLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredEndpointSlices,
		endpointSlices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scc.kubeClient.DiscoveryV1().EndpointSlices(sc.Namespace).Delete,
		},
		scc.eventRecorder)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpointslice(s): %w", err)
	}

	for _, requiredEndpointSlice := range requiredEndpointSlices {
		_, changed, err := resourceapply.ApplyEndpointSlice(ctx, scc.kubeClient.DiscoveryV1(), scc.endpointSliceLister, scc.eventRecorder, requiredEndpointSlice, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, endpointSliceControllerProgressingCondition, requiredEndpointSlice, "apply", sc.Generation)
		}
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply endpointslice: %w", err)
		}
	}

	return progressingConditions, nil
}
