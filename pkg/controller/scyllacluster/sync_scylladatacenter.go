// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (scc *Controller) syncScyllaDatacenters(
	ctx context.Context,
	sc *scyllav2alpha1.ScyllaCluster,
	remoteScyllaDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDatacenter,
	status *scyllav2alpha1.ScyllaClusterStatus,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredScyllaDatacenters := MakeScyllaDatacenters(sc)

	// Delete any excessive ScyllaDatacenters.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for remoteName, scyllaDatacenters := range remoteScyllaDatacenters {
		for _, sd := range scyllaDatacenters {
			if sd.DeletionTimestamp != nil {
				continue
			}

			req, ok := requiredScyllaDatacenters[remoteName]
			if ok && sd.Name == req.Name {
				continue
			}

			propagationPolicy := metav1.DeletePropagationBackground
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDatacenterControllerProgressingCondition, sd, "delete", sc.Generation)
			var err error
			// TODO: This should first scale all racks to 0, only then remove.
			//  	 Without graceful removal, state of other DC might be skewed, but since we don't support it yet
			//       it can be postponed.
			if remoteName != "" {
				dcClient, err := scc.remoteDynamicClient.Region(remoteName)
				if err != nil {
					return progressingConditions, fmt.Errorf("can't get client to %q remote cluster: %w", remoteName, err)
				}
				err = dcClient.Resource(scyllaDatacenterGVR).Namespace(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &sd.UID,
					},
					PropagationPolicy: &propagationPolicy,
				})
			} else {
				err = scc.scyllaClient.ScyllaV1alpha1().ScyllaDatacenters(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &sd.UID,
					},
					PropagationPolicy: &propagationPolicy,
				})
			}

			deletionErrors = append(deletionErrors, err)
		}
	}
	var err error
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return nil, fmt.Errorf("can't delete scylla datacenter(s): %w", err)
	}

	for remoteName, sd := range requiredScyllaDatacenters {
		var changed bool
		if remoteName == "" {
			sd, changed, err = resourceapply.ApplyScyllaDatacenter(ctx, scc.scyllaClient.ScyllaV1alpha1(), scc.scyllaDatacenterLister, scc.eventRecorder, sd)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply scylla datacenter: %w", err)
			}
		} else {
			remoteClient, err := scc.remoteDynamicClient.Region(remoteName)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't get remote client: %w", err)
			}
			sdRemoteClient := remoteClient.Resource(scyllaDatacenterGVR)
			sdRemoteLister := scc.remoteScyllaDatacenterLister.Region(remoteName)

			sd, changed, err = resourceapply.ApplyGenericObject(ctx, sdRemoteClient, sdRemoteLister, scc.eventRecorder, sd)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply scylla datacenter: %w", err)
			}
		}
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDatacenterControllerProgressingCondition, sd, "apply", sc.Generation)
		}

		for i, dc := range sc.Spec.Datacenters {
			if dc.Name != sd.Spec.DatacenterName {
				continue
			}
			status.Datacenters[i] = *scc.calculateDatacenterStatus(sc, dc, sd)
			break
		}

		if !controllerhelpers.IsScyllaDatacenterRolledOut(sd) {
			klog.V(4).InfoS("Waiting for ScyllaDatacenter rollout", "ScyllaCluster", klog.KObj(sc), "ScyllaDatacenter", klog.KObj(sd))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               scyllaDatacenterControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForScyllaDatacenterRollout",
				Message:            fmt.Sprintf("Waiting for ScyllaDatacenter %q to rollout.", naming.ObjRef(sd)),
				ObservedGeneration: sd.Generation,
			})
		}
	}

	return progressingConditions, nil
}
