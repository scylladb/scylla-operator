// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (scc *Controller) syncRemoteEndpoints(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteEndpoints map[string]*corev1.Endpoints,
	remoteNamespaces map[string]*corev1.Namespace,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := MakeRemoteEndpointSlices(sc, dc, remoteNamespace, remoteController, remoteNamespaces, scc.remoteServiceLister, scc.remotePodLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}

	requiredEndpoints := make([]*corev1.Endpoints, 0, len(requiredEndpointSlices))

	re, err := controllerhelpers.ConvertEndpointSlicesToEndpoints(requiredEndpointSlices)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't convert endpointslices to endpoints: %w", err)
	}

	requiredEndpoints = append(requiredEndpoints, re...)

	clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive Endpoints.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredEndpoints,
		remoteEndpoints,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.CoreV1().Endpoints(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpoints in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, e := range requiredEndpoints {
		_, changed, err := resourceapply.ApplyEndpoints(ctx, clusterClient.CoreV1(), scc.remoteEndpointsLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, e, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteEndpointsControllerDatacenterProgressingCondition(dc.Name), e, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply endpoints: %w", err)
		}
	}

	return progressingConditions, nil
}

func (scc *Controller) syncLocalEndpoints(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	endpoints map[string]*corev1.Endpoints,
	remoteNamespaces map[string]*corev1.Namespace,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := makeLocalEndpointSlices(sc, remoteNamespaces, scc.remoteServiceLister, scc.remotePodLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
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
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpoints of %q ScyllaDBCluster: %w", naming.ObjRef(sc), err)
	}

	for _, e := range requiredEndpoints {
		_, changed, err := resourceapply.ApplyEndpoints(ctx, scc.kubeClient.CoreV1(), scc.endpointsLister, scc.eventRecorder, e, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, endpointsControllerProgressingCondition, e, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply endpoints: %w", err)
		}
	}

	return progressingConditions, nil
}
