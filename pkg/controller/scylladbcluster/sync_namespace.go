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

func (scc *Controller) syncNamespaces(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]map[string]*corev1.Namespace,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredNamespaces, err := MakeRemoteNamespaces(sc, remoteNamespaces, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required namespaces: %w", err)
	}

	// Delete any excessive Namespaces.
	// Delete has to be the first action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		err = controllerhelpers.Prune(ctx,
			requiredNamespaces[dc.RemoteKubernetesClusterName],
			remoteNamespaces[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
					return clusterClient.CoreV1().Namespaces().Delete(ctx, name, opts)
				},
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune namespace(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't delete namespace(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.Name, err)
		}
		lister := scc.remoteNamespaceLister.Cluster(dc.RemoteKubernetesClusterName)

		rnss, ok := requiredNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			continue
		}
		for _, rns := range rnss {
			_, changed, err := resourceapply.ApplyNamespace(ctx, clusterClient.CoreV1(), lister, scc.eventRecorder, rns, resourceapply.ApplyOptions{
				AllowMissingControllerRef: true,
			})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteNamespaceControllerProgressingCondition, rns, "apply", sc.Generation)
			}
			if err != nil {
				return nil, fmt.Errorf("can't apply namespace %q: %w", rns.Name, err)
			}
		}
	}

	return progressingConditions, nil
}
