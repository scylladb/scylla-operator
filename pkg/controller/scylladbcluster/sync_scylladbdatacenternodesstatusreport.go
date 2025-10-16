// Copyright (C) 2025 ScyllaDB

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

func (scc *Controller) syncRemoteScyllaDBDatacenterNodesStatusReports(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteScyllaDBDatacenterNodesStatusReports map[string]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredScyllaDBDatacenterNodesStatusReports, err := makeRemoteScyllaDBDatacenterNodesStatusReports(sc, dc, remoteNamespace, remoteController, remoteNamespaces, remoteScyllaDBDatacenters, scc.remoteScyllaDBDatacenterNodesStatusReportLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make scyllaDBDatacenterNodesStatusReports: %w", err)
	}

	clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredScyllaDBDatacenterNodesStatusReports,
		remoteScyllaDBDatacenterNodesStatusReports,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.ScyllaV1alpha1().ScyllaDBDatacenterNodesStatusReports(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scyllaDBDatacenterNodesStatusReports in %q Datacenter of ScyllaDBCluster %q: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, ssr := range requiredScyllaDBDatacenterNodesStatusReports {
		_, changed, err := resourceapply.ApplyScyllaDBDatacenterNodesStatusReport(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteScyllaDBDatacenterNodesStatusReportLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, ssr, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterProgressingCondition(dc.Name), ssr, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply scyllaDBDatacenterNodesStatusReport: %w", err)
		}
	}

	return progressingConditions, nil
}
