// Copyright (C) 2025 ScyllaDB

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
	progressingConditions, requiredScyllaDBDatacenterNodesStatusReports, err := makeRemoteScyllaDBDatacenterNodesStatusReports(sc, dc, remoteNamespace, remoteController, remoteNamespaces, remoteScyllaDBDatacenters, scc.remoteScyllaDBDatacenterNodesStatusReportLister(ctx), managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make scyllaDBDatacenterNodesStatusReports: %w", err)
	}

	remoteCluster, err := scc.remoteCluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredScyllaDBDatacenterNodesStatusReports,
		remoteScyllaDBDatacenterNodesStatusReports,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ctrlclient.DeleteFunc[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport](remoteCluster.GetClient(), remoteNamespace.Name),
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scyllaDBDatacenterNodesStatusReports in %q Datacenter of ScyllaDBCluster %q: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, ssr := range requiredScyllaDBDatacenterNodesStatusReports {
		_, changed, err := resourceapply.ApplyScyllaDBDatacenterNodesStatusReportWithControl(ctx, ctrlclient.ApplyControl[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport](ctx, remoteCluster.GetClient(), remoteNamespace.Name), scc.eventRecorder, ssr, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterProgressingCondition(dc.Name), ssr, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply scyllaDBDatacenterNodesStatusReport: %w", err)
		}
	}

	return progressingConditions, nil
}
