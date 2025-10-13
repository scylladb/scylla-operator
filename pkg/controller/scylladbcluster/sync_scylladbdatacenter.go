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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (scc *Controller) syncRemoteScyllaDBDatacenters(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	status *scyllav1alpha1.ScyllaDBClusterStatus,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	requiredScyllaDBDatacenter, err := MakeRemoteScyllaDBDatacenters(sc, dc, remoteScyllaDBDatacenters, remoteNamespace, remoteController, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote ScyllaDBDatacenters: %w", err)
	}

	clusterClient, err := scc.scyllaRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive ScyllaDBDatacenters.
	// Delete has to be the first action to avoid getting stuck on quota.
	// FIXME: This should first scale all racks to 0, only then remove.
	//  	 Without graceful removal, state of other DC might be skewed.
	// Ref: https://github.com/scylladb/scylla-operator/issues/2604
	err = controllerhelpers.Prune(ctx,
		[]*scyllav1alpha1.ScyllaDBDatacenter{requiredScyllaDBDatacenter},
		remoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName],
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune scylladbdatacenter(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	existingSDC, sdcExists := remoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName][requiredScyllaDBDatacenter.Name]
	if !sdcExists {
		klog.V(4).InfoS("Required ScyllaDBDatacenter doesn't exists, awaiting all previous DC to finish bootstrapping", "ScyllaDBCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(requiredScyllaDBDatacenter))
		for i := range sc.Spec.Datacenters {
			if sc.Spec.Datacenters[i].Name == dc.Name {
				break
			}

			previousDCSpec := sc.Spec.Datacenters[i]
			isScyllaDBDatacenterControllerProgressing := meta.IsStatusConditionTrue(status.Conditions, makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(previousDCSpec.Name))
			isScyllaDBDatacenterControllerDegraded := meta.IsStatusConditionTrue(status.Conditions, makeRemoteScyllaDBDatacenterControllerDatacenterDegradedCondition(previousDCSpec.Name))
			if isScyllaDBDatacenterControllerProgressing || isScyllaDBDatacenterControllerDegraded {
				klog.V(4).InfoS(
					"Waiting for ScyllaDBDatacenter controller for previous datacenter to finish progressing or recover from degraded state",
					"ScyllaDBCluster", klog.KObj(sc),
					"Datacenter", dc.Name,
					"PreviousDatacenter", previousDCSpec.Name,
					"Progressing", isScyllaDBDatacenterControllerProgressing,
					"Degraded", isScyllaDBDatacenterControllerDegraded,
				)
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name),
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForScyllaDBDatacenterController",
					Message:            fmt.Sprintf("Waiting for ScyllaDBDatacenter controller for %q datacenter to finish progressing or recover from degraded state", previousDCSpec.Name),
					ObservedGeneration: sc.Generation,
				})
			}

		}

		// Scylla cannot start without connecting to seeds. Before we create new DC, make sure seed service and endpoints behind it are already reconciled.
		isEndpointSliceControllerProgressing := meta.IsStatusConditionTrue(status.Conditions, makeRemoteEndpointSliceControllerDatacenterProgressingCondition(dc.Name))
		isServiceControllerProgressing := meta.IsStatusConditionTrue(status.Conditions, makeRemoteServiceControllerDatacenterProgressingCondition(dc.Name))
		isEndpointSliceControllerDegraded := meta.IsStatusConditionTrue(status.Conditions, makeRemoteEndpointSliceControllerDatacenterDegradedCondition(dc.Name))
		isServiceControllerDegraded := meta.IsStatusConditionTrue(status.Conditions, makeRemoteServiceControllerDatacenterDegradedCondition(dc.Name))
		if isEndpointSliceControllerProgressing || isServiceControllerProgressing || isEndpointSliceControllerDegraded || isServiceControllerDegraded {
			klog.V(4).InfoS(
				"Waiting until EndpointSlice and Service controllers are no longer progressing or degraded",
				"ScyllaDBCluster", klog.KObj(sc),
				"ScyllaDBDatacenter", klog.KObj(requiredScyllaDBDatacenter),
				"Datacenter", dc.Name,
				"EndpointSliceProgressing", isEndpointSliceControllerProgressing,
				"ServiceProgressing", isServiceControllerProgressing,
				"EndpointSliceDegraded", isEndpointSliceControllerDegraded,
				"ServiceDegraded", isServiceControllerDegraded,
			)
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name),
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForEndpointSliceServiceController",
				Message:            fmt.Sprintf("Waiting for EndpointSlice and Service controller for %q datacenter to finish progressing or recover from degraded state", dc.Name),
				ObservedGeneration: sc.Generation,
			})
		}

		// Wait for ScyllaDBDatacenterNodesStatusReport controller to finish progressing before proceeding.
		// This ensures that the mirrored ScyllaDBDatacenterNodeStatusReports are up to date before a new DC is added,
		// which lowers the chance of a new node being bootstrapped while the cluster is unhealthy.
		isScyllaDBDatacenterNodesStatusReportControllerProgressing := meta.IsStatusConditionTrue(status.Conditions, makeRemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterProgressingCondition(dc.Name))
		isScyllaDBDatacenterNodesStatusReportControllerDegraded := meta.IsStatusConditionTrue(status.Conditions, makeRemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterDegradedCondition(dc.Name))
		if isScyllaDBDatacenterNodesStatusReportControllerProgressing || isScyllaDBDatacenterNodesStatusReportControllerDegraded {
			klog.V(4).InfoS(
				"Waiting for ScyllaDBDatacenterNodesStatusReport controller to finish progressing or recover from degraded state",
				"ScyllaDBCluster", klog.KObj(sc),
				"ScyllaDBDatacenter", klog.KObj(requiredScyllaDBDatacenter),
				"Datacenter", dc.Name,
				"Progressing", isScyllaDBDatacenterNodesStatusReportControllerProgressing,
				"Degraded", isScyllaDBDatacenterNodesStatusReportControllerDegraded,
			)
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name),
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForScyllaDBDatacenterNodesStatusReportController",
				Message:            fmt.Sprintf("Waiting for ScyllaDBDatacenterNodesStatusReport controller for %q datacenter to finish progressing or recover from degraded state", dc.Name),
				ObservedGeneration: sc.Generation,
			})
		}
	}

	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	sdc, changed, err := resourceapply.ApplyScyllaDBDatacenter(ctx, clusterClient.ScyllaV1alpha1(), scc.remoteScyllaDBDatacenterLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, requiredScyllaDBDatacenter, resourceapply.ApplyOptions{})
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply scylladbdatacenter: %w", err)
	}

	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name), sdc, "apply", sc.Generation)
	}

	// Use existingSDC coming from cache to validate the rollout state instead of a freshly fetched SDC,
	// because the state of the required object depends on the state of other DCs (e.g., seed calculation).
	// Using a fresh SDC could result in evaluating different state than an outdated state due to cache staleness,
	// leading us to incorrectly mark the ScyllaCluster as fully rolled out while pending changes still exist.
	if !sdcExists {
		existingSDC = sdc
	}
	rolledOut, err := controllerhelpers.IsScyllaDBDatacenterRolledOut(existingSDC)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't check if scylladbdatacenter is rolled out: %w", err)
	}

	if !rolledOut {
		klog.V(4).InfoS("Waiting for ScyllaDBDatacenter to roll out", "ScyllaDBCluster", klog.KObj(sc), "ScyllaDBDatacenter", klog.KObj(sdc))
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name),
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForScyllaDBDatacenterRollout",
			Message:            fmt.Sprintf("Waiting for ScyllaDBDatacenter %q to roll out.", naming.ObjRef(sdc)),
			ObservedGeneration: sc.Generation,
		})
	}

	return progressingConditions, nil
}
