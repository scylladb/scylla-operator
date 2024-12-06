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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncEndpoints(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteControllers map[string]metav1.Object,
	remoteEndpoints map[string]map[string]*corev1.Endpoints,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredEndpointSlices, err := MakeRemoteEndpointSlices(sc, remoteNamespaces, remoteControllers, scc.remoteServiceLister, scc.remotePodLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make endpointslices: %w", err)
	}

	requiredEndpoints := make(map[string][]*corev1.Endpoints, len(requiredEndpointSlices))

	for _, dc := range sc.Spec.Datacenters {
		re, err := controllerhelpers.ConvertEndpointSlicesToEndpoints(requiredEndpointSlices[dc.RemoteKubernetesClusterName])
		if err != nil {
			return progressingConditions, fmt.Errorf("can't convert endpointslices to endpoints: %w", err)
		}

		requiredEndpoints[dc.RemoteKubernetesClusterName] = re
	}

	// Delete any excessive Endpoints.
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
			requiredEndpoints[dc.RemoteKubernetesClusterName],
			remoteEndpoints[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.CoreV1().Endpoints(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune endpoints in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't prune remote endpoints: %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.Name, err)
		}

		for _, e := range requiredEndpoints[dc.RemoteKubernetesClusterName] {
			_, changed, err := resourceapply.ApplyEndpoints(ctx, clusterClient.CoreV1(), scc.remoteEndpointsLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, e, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteEndpointsControllerProgressingCondition, e, "apply", sc.Generation)
			}
			if err != nil {
				return nil, fmt.Errorf("can't apply endpoints: %w", err)
			}
		}
	}

	return progressingConditions, nil
}
