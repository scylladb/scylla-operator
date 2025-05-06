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

func (scc *Controller) syncRemoteRemoteOwners(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteRemoteOwners map[string]*scyllav1alpha1.RemoteOwner,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	requiredRemoteOwners, err := MakeRemoteRemoteOwners(sc, dc, remoteNamespace, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote owners: %w", err)
	}

	clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive RemoteOwners.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredRemoteOwners,
		remoteRemoteOwners,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.ScyllaV1alpha1().RemoteOwners(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune remoteowner(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, ro := range requiredRemoteOwners {
		_, changed, err := resourceapply.ApplyRemoteOwner(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteRemoteOwnerLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, ro, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteRemoteOwnerControllerDatacenterProgressingCondition(dc.Name), ro, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply remoteowner: %w", err)
		}
	}

	return progressingConditions, nil
}
