// Copyright (c) 2024 ScyllaDB.

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

	remoteCluster, err := scc.remoteCluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q region: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete any excessive Services.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredServices,
		remoteServices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ctrlclient.DeleteFunc[corev1.Service](remoteCluster.GetClient(), remoteNamespace.Name),
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune service(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	for _, svc := range requiredServices {
		_, changed, err := resourceapply.ApplyServiceWithControl(ctx, ctrlclient.ApplyControl[corev1.Service](ctx, remoteCluster.GetClient(), remoteNamespace.Name), scc.eventRecorder, svc, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteServiceControllerDatacenterProgressingCondition(dc.Name), svc, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply service: %w", err)
		}
	}

	return progressingConditions, nil
}

func (scc *Controller) syncLocalServices(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	localServices map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var err error

	requiredServices, err := makeLocalServices(sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make local services for %q ScyllaDBCluster: %w", naming.ObjRef(sc), err)
	}

	err = controllerhelpers.Prune(ctx,
		requiredServices,
		localServices,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: ctrlclient.DeleteFunc[corev1.Service](scc.client, sc.Namespace),
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune service(s) of %q ScyllaDBCluster: %w", naming.ObjRef(sc), err)
	}

	for _, svc := range requiredServices {
		_, changed, err := resourceapply.ApplyServiceWithControl(ctx, ctrlclient.ApplyControl[corev1.Service](ctx, scc.client, sc.Namespace), scc.eventRecorder, svc, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply service: %w", err)
		}
	}

	return progressingConditions, nil
}
