// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncNamespaces(
	ctx context.Context,
	sc *scyllav2alpha1.ScyllaCluster,
	remoteNamespaces map[string]map[string]*corev1.Namespace,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredNamespaces := MakeNamespaces(sc)

	// Delete any excessive Namespaces.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		// Do not manage namespaces in local cluster.
		if dc.RemoteKubeClusterConfigRef == nil {
			continue
		}
		remoteName := dc.RemoteKubeClusterConfigRef.Name
		for _, ns := range remoteNamespaces[remoteName] {
			if ns.DeletionTimestamp != nil {
				continue
			}

			req, ok := requiredNamespaces[remoteName]
			if ok && ns.Name == req.Name {
				continue
			}

			propagationPolicy := metav1.DeletePropagationBackground
			regionClient, err := scc.remoteDynamicClient.Region(remoteName)
			if err != nil {
				return nil, fmt.Errorf("can't get client to %q region: %w", remoteName, err)
			}
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, namespaceControllerProgressingCondition, ns, "delete", sc.Generation)
			err = regionClient.Resource(namespaceGVR).Delete(ctx, ns.Name, metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: &ns.UID,
				},
				PropagationPolicy: &propagationPolicy,
			})
			deletionErrors = append(deletionErrors, err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't delete namespace(s): %w", err)
	}

	for _, dc := range sc.Spec.Datacenters {
		// Do not manage namespaces in local cluster.
		if dc.RemoteKubeClusterConfigRef == nil {
			continue
		}
		remoteName := dc.RemoteKubeClusterConfigRef.Name
		ns := requiredNamespaces[remoteName]

		dcClient, err := scc.remoteDynamicClient.Region(remoteName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q region: %w", dc.Name, err)
		}
		lister := scc.remoteNamespaceLister.Region(remoteName)
		_, changed, err := resourceapply.ApplyGenericObjectNonNamespaced(ctx, dcClient.Resource(namespaceGVR), lister, scc.eventRecorder, ns)
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, namespaceControllerProgressingCondition, ns, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply namespace: %w", err)
		}
	}

	return progressingConditions, nil
}
