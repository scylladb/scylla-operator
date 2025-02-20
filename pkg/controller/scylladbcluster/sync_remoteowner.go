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

func (scc *Controller) syncRemoteOwners(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteRemoteOwners map[string]map[string]*scyllav1alpha1.RemoteOwner,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredRemoteOwners, err := MakeRemoteRemoteOwners(sc, remoteNamespaces, remoteRemoteOwners, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote owners: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Delete any excessive RemoteOwners.
	// Delete has to be the first action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			continue
		}

		clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		err = controllerhelpers.Prune(ctx,
			requiredRemoteOwners[dc.RemoteKubernetesClusterName],
			remoteRemoteOwners[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune remoteowner(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't prune remote remoteowner(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.Name, err)
		}

		for _, ro := range requiredRemoteOwners[dc.RemoteKubernetesClusterName] {
			_, changed, err := resourceapply.ApplyRemoteOwner(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteRemoteOwnerLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, ro, resourceapply.ApplyOptions{
				AllowMissingControllerRef: true,
			})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteRemoteOwnerControllerProgressingCondition, ro, "apply", sc.Generation)
			}
			if err != nil {
				return nil, fmt.Errorf("can't apply remoteowner: %w", err)
			}
		}
	}

	return progressingConditions, nil
}
