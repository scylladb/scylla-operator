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
	"k8s.io/klog/v2"
)

func (scc *Controller) syncScyllaDBDatacenters(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteControllers map[string]metav1.Object,
	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredScyllaDBDatacenters, err := MakeRemoteScyllaDBDatacenters(sc, remoteScyllaDBDatacenters, remoteNamespaces, remoteControllers, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote ScyllaDBDatacenters: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Delete any excessive ScyllaDBDatacenters.
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

		// FIXME: This should first scale all racks to 0, only then remove.
		//  	 Without graceful removal, state of other DC might be skewed.
		// Ref: https://github.com/scylladb/scylla-operator/issues/2604
		err = controllerhelpers.Prune(ctx,
			requiredScyllaDBDatacenters[dc.RemoteKubernetesClusterName],
			remoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune scylladbdatacenter(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return nil, fmt.Errorf("can't prune scylladbdatacenter(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't get remote client: %w", err)
		}

		for _, sdc := range requiredScyllaDBDatacenters[dc.RemoteKubernetesClusterName] {
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
	}

	return progressingConditions, nil
}
