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

func (scc *Controller) syncRemoteScyllaDBStatusReports(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteScyllaDBStatusReports map[string]*scyllav1alpha1.ScyllaDBStatusReport,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredScyllaDBStatusReports, err := makeRemoteScyllaDBStatusReports(sc, dc, remoteNamespace, remoteController, remoteNamespaces, remoteScyllaDBDatacenters, scc.remoteScyllaDBStatusReportLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make scylladbstatusreports: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	err = controllerhelpers.Prune(
		ctx,
		requiredScyllaDBStatusReports,
		remoteScyllaDBStatusReports,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.ScyllaV1alpha1().ScyllaDBStatusReports(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scylladbstatusreports in %q Datacenter of ScyllaDBCluster %q: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, ssr := range requiredScyllaDBStatusReports {
		_, changed, err := resourceapply.ApplyScyllaDBStatusReport(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteScyllaDBStatusReportLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, ssr, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteScyllaDBStatusReportControllerDatacenterProgressingCondition(dc.Name), ssr, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply scylladbstatusreport: %w", err)
		}
	}

	return progressingConditions, nil
}

//func (scc *Controller) syncLocalScyllaDBStatusReports(
//	ctx context.Context,
//	sc *scyllav1alpha1.ScyllaDBCluster,
//	localScyllaDBStatusReports map[string]*scyllav1alpha1.ScyllaDBStatusReport,
//	remoteNamespaces map[string]*corev1.Namespace,
//	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
//) ([]metav1.Condition, error) {
//	progressingConditions, requiredScyllaDBStatusReports, err := makeLocalScyllaDBStatusReports(sc, remoteNamespaces, remoteScyllaDBDatacenters, scc.remoteScyllaDBStatusReportLister)
//	if err != nil {
//		return progressingConditions, fmt.Errorf("can't make scylladbstatusreports: %w", err)
//	}
//	if len(progressingConditions) > 0 {
//		return progressingConditions, nil
//	}
//
//	err = controllerhelpers.Prune(
//		ctx,
//		requiredScyllaDBStatusReports,
//		localScyllaDBStatusReports,
//		&controllerhelpers.PruneControlFuncs{
//			DeleteFunc: scc.scyllaClient.ScyllaV1alpha1().ScyllaDBStatusReports(sc.Namespace).Delete,
//		},
//		scc.eventRecorder,
//	)
//	if err != nil {
//		return progressingConditions, fmt.Errorf("can't prune scylladbstatusreports of ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
//	}
//
//	for _, ssr := range requiredScyllaDBStatusReports {
//		_, changed, err := resourceapply.ApplyScyllaDBStatusReport(ctx, scc.scyllaClient.ScyllaV1alpha1(), scc.scyllaDBStatusReportLister, scc.eventRecorder, ssr, resourceapply.ApplyOptions{})
//		if changed {
//			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBStatusReportControllerProgressingCondition, ssr, "apply", sc.Generation)
//		}
//		if err != nil {
//			return nil, fmt.Errorf("can't apply scylladbstatusreport: %w", err)
//		}
//	}
//
//	return progressingConditions, nil
//}
