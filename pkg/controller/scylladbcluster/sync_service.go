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

func (scc *Controller) syncRemoteServices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteServices map[string]*corev1.Service,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredServices := MakeRemoteServices(sc, dc, remoteNamespace, remoteController, managingClusterDomain)

	clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q region: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive Services.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredServices,
		remoteServices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.CoreV1().Services(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune service(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, svc := range requiredServices {
		_, changed, err := resourceapply.ApplyService(ctx, clusterClient.CoreV1(), scc.remoteServiceLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, svc, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteServiceControllerProgressingCondition, svc, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply service: %w", err)
		}
	}

	return progressingConditions, nil
}
