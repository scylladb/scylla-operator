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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (scc *Controller) syncRemoteEndpointSlices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteEndpointSlices map[string]*discoveryv1.EndpointSlice,
	remoteNamespaces map[string]*corev1.Namespace,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := MakeRemoteEndpointSlices(sc, dc, remoteNamespace, remoteController, remoteNamespaces, scc.remoteServiceLister, scc.remotePodLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive EndpointSlices.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredEndpointSlices,
		remoteEndpointSlices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.DiscoveryV1().EndpointSlices(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpointslices in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, es := range requiredEndpointSlices {
		_, changed, err := resourceapply.ApplyEndpointSlice(ctx, clusterClient.DiscoveryV1(), scc.remoteEndpointSliceLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, es, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteEndpointSliceControllerDatacenterProgressingCondition(dc.Name), es, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply endpointslice: %w", err)
		}
	}

	return progressingConditions, nil
}

func (scc *Controller) syncLocalEndpointSlices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	endpointSlices map[string]*discoveryv1.EndpointSlice,
	remoteNamespaces map[string]*corev1.Namespace,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := makeLocalEndpointSlices(sc, remoteNamespaces, scc.remoteServiceLister, scc.remotePodLister)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredEndpointSlices,
		endpointSlices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scc.kubeClient.DiscoveryV1().EndpointSlices(sc.Namespace).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune endpointslices of %q ScyllaDBCluster: %w", naming.ObjRef(sc), err)
	}

	for _, es := range requiredEndpointSlices {
		_, changed, err := resourceapply.ApplyEndpointSlice(ctx, scc.kubeClient.DiscoveryV1(), scc.endpointSliceLister, scc.eventRecorder, es, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, endpointSliceControllerProgressingCondition, es, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply endpointslice: %w", err)
		}
	}

	return progressingConditions, nil
}
