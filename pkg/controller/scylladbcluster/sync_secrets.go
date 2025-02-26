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
)

func (scc *Controller) syncSecrets(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	remoteNamespaces map[string]*corev1.Namespace,
	remoteControllers map[string]metav1.Object,
	remoteSecrets map[string]map[string]*corev1.Secret,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredRemoteSecrets, err := MakeRemoteSecrets(sc, remoteNamespaces, remoteControllers, scc.secretLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote secrets: %w", err)
	}

	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Delete has to be the first action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			continue
		}

		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		err = controllerhelpers.Prune(ctx,
			requiredRemoteSecrets[dc.RemoteKubernetesClusterName],
			remoteSecrets[dc.RemoteKubernetesClusterName],
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: clusterClient.CoreV1().Secrets(ns.Name).Delete,
			},
			scc.eventRecorder,
		)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't prune secret(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
		}
	}

	if err := utilerrors.NewAggregate(deletionErrors); err != nil {
		return nil, fmt.Errorf("can't prune remote secret(s): %w", err)
	}

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get client to %q cluster: %w", dc.Name, err))
			continue
		}

		for _, rs := range requiredRemoteSecrets[dc.RemoteKubernetesClusterName] {
			_, changed, err := resourceapply.ApplySecret(ctx, clusterClient.CoreV1(), scc.remoteSecretLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, rs, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, remoteSecretControllerProgressingCondition, rs, "apply", sc.Generation)
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("can't apply secret: %w", err))
				continue
			}
		}
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply secret(s): %w", err)
	}

	return progressingConditions, nil
}
