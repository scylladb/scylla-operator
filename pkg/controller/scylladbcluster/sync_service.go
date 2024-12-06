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

func (scc *Controller) syncServices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteControllers map[string]metav1.Object,
	remoteServices map[string]map[string]*corev1.Service,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredServices := MakeRemoteServices(sc, remoteNamespaces, remoteControllers, managingClusterDomain)
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Delete any excessive Services.
	// Delete has to be the first action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			continue
		}

		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q region: %w", dc.RemoteKubernetesClusterName, err)
		}

		err = controllerhelpers.Prune(ctx,
			requiredServices[dc.RemoteKubernetesClusterName],
			remoteServices[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.CoreV1().Services(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune service(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't prune remote services(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q region: %w", dc.Name, err)
		}

		for _, svc := range requiredServices[dc.RemoteKubernetesClusterName] {
			_, changed, err := resourceapply.ApplyService(ctx, clusterClient.CoreV1(), scc.remoteServiceLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, svc, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteServiceControllerProgressingCondition, svc, "apply", sc.Generation)
			}
			if err != nil {
				return nil, fmt.Errorf("can't apply service: %w", err)
			}
		}
	}

	return progressingConditions, nil
}
