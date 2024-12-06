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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncEndpointSlices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteControllers map[string]metav1.Object,
	remoteEndpointSlices map[string]map[string]*discoveryv1.EndpointSlice,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := MakeRemoteEndpointSlices(sc, remoteNamespaces, remoteControllers, scc.remoteServiceLister, scc.remotePodLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Delete any excessive EndpointSlices.
	// Delete has to be the first action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			continue
		}

		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		err = controllerhelpers.Prune(ctx,
			requiredEndpointSlices[dc.RemoteKubernetesClusterName],
			remoteEndpointSlices[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.DiscoveryV1().EndpointSlices(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune endpointslices in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't prune remote endpointslice(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q region: %w", dc.Name, err)
		}

		for _, es := range requiredEndpointSlices[dc.RemoteKubernetesClusterName] {
			_, changed, err := resourceapply.ApplyEndpointSlice(ctx, clusterClient.DiscoveryV1(), scc.remoteEndpointSliceLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, es, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteEndpointSliceControllerProgressingCondition, es, "apply", sc.Generation)
			}
			if err != nil {
				return nil, fmt.Errorf("can't apply endpointslice: %w", err)
			}
		}
	}

	return progressingConditions, nil
}
