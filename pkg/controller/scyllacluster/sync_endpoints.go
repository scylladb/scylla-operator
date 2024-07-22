package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Endpoints are reconciled additionally to EndpointSlices because Prometheus Operator which we rely on in ScyllaDBMonitoring
// doesn't support EndpointSlices yet.
func (scc *Controller) syncEndpoints(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	endpoints map[string]*corev1.Endpoints,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	// EndpointSlices supersedes Endpoints, so to make sure reconciling logic is the same,
	// convert from one to the other.
	requiredEndpointSlices, err := MakeEndpointSlices(sc, services, scc.podLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}

	requiredEndpoints, err := controllerhelpers.ConvertEndpointSlicesToEndpoints(requiredEndpointSlices)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't convert endpointslices to endpoints: %w", err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredEndpoints,
		endpoints,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scc.kubeClient.CoreV1().Endpoints(sc.Namespace).Delete,
		},
		scc.eventRecorder)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpoints: %w", err)
	}

	for _, rs := range requiredEndpoints {
		updated, changed, err := resourceapply.ApplyEndpoints(ctx, scc.kubeClient.CoreV1(), scc.endpointsLister, scc.eventRecorder, rs, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, endpointsControllerProgressingCondition, rs, "apply", sc.Generation)
		}
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply endpoints: %w", err)
		}

		_, _, hasUnreadySubset := slices.Find(updated.Subsets, func(sn corev1.EndpointSubset) bool {
			return len(sn.NotReadyAddresses) != 0
		})
		if len(updated.Subsets) == 0 || hasUnreadySubset {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               endpointsControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "EndpointsNotReady",
				Message:            fmt.Sprintf("Endpoints %q is not yet ready", naming.ObjRef(updated)),
				ObservedGeneration: sc.Generation,
			})
		}
	}

	return progressingConditions, nil
}
