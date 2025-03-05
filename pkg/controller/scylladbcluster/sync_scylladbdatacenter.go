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
	"k8s.io/klog/v2"
)

func (scc *Controller) syncRemoteScyllaDBDatacenters(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredScyllaDBDatacenters, err := MakeRemoteScyllaDBDatacenters(sc, dc, remoteScyllaDBDatacenters, remoteNamespace, remoteController, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote ScyllaDBDatacenters: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive ScyllaDBDatacenters.
	// Delete has to be the first action to avoid getting stuck on quota.
	// FIXME: This should first scale all racks to 0, only then remove.
	//  	 Without graceful removal, state of other DC might be skewed.
	// Ref: https://github.com/scylladb/scylla-operator-enterprise/issues/59
	err = controllerhelpers.Prune(ctx,
		requiredScyllaDBDatacenters,
		remoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName],
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scylladbdatacenter(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, sdc := range requiredScyllaDBDatacenters {
		sdc, changed, err := resourceapply.ApplyScyllaDBDatacenter(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteScyllaDBDatacenterLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, sdc, resourceapply.ApplyOptions{})
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply scylladbdatacenter: %w", err)
		}

		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteScyllaDBDatacenterControllerProgressingCondition, sdc, "apply", sc.Generation)
		}

		rolledOut, err := controllerhelpers.IsScyllaDBDatacenterRolledOut(sdc)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't check if scylladbdatacenter is rolled out: %w", err)
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for ScyllaDBDatacenter to roll out", "ScyllaDBCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               remoteScyllaDBDatacenterControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForScyllaDBDatacenterRollout",
				Message:            fmt.Sprintf("Waiting for ScyllaDBDatacenter %q to roll out.", naming.ObjRef(sdc)),
				ObservedGeneration: sc.Generation,
			})

			return progressingConditions, nil
		}
	}

	return progressingConditions, nil
}
