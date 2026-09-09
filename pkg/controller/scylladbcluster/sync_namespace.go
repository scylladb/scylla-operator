// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (scc *Controller) syncRemoteNamespaces(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespaces map[string]*corev1.Namespace,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredNamespaces, err := MakeRemoteNamespaces(sc, dc, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required namespaces: %w", err)
	}

	remoteCluster, err := scc.remoteCluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive Namespaces.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredNamespaces,
		remoteNamespaces,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ctrlclient.DeleteFunc[corev1.Namespace](remoteCluster.GetClient(), ""),
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune namespace(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, rns := range requiredNamespaces {
		_, changed, err := resourceapply.ApplyNamespaceWithControl(ctx, ctrlclient.ApplyControl[corev1.Namespace](ctx, remoteCluster.GetClient(), ""), scc.eventRecorder, rns, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteNamespaceControllerDatacenterProgressingCondition(dc.Name), rns, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply namespace %q: %w", rns.Name, err)
		}
	}

	return progressingConditions, nil
}
