package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncRemoteSecrets(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteSecrets map[string]*corev1.Secret,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredRemoteSecrets, err := MakeRemoteSecrets(sc, dc, remoteNamespace, remoteController, scc.secretLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make remote secrets: %w", err)
	}

	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredRemoteSecrets,
		remoteSecrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.CoreV1().Secrets(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune secret(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	var errs []error
	for _, rs := range requiredRemoteSecrets {
		_, changed, err := resourceapply.ApplySecret(ctx, clusterClient.CoreV1(), scc.remoteSecretLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, rs, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteSecretControllerDatacenterProgressingCondition(dc.Name), rs, "apply", sc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply secret: %w", err))
			continue
		}
	}

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply secret(s): %w", err)
	}

	return progressingConditions, nil
}

func (scc *Controller) syncLocalSecrets(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	localSecrets map[string]*corev1.Secret,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	// We do not take custom agent config into account on ScyllaDBCluster level, as the secrets are propagated to ScyllaDBDatacenters.
	scyllaDBManagerAgentAuthTokenProgressingConditions, scyllaDBManagerAgentAuthToken, err := controllerhelpers.GetScyllaDBManagerAgentAuthToken(
		getOptionalExistingScyllaDBManagerAuthTokenFunc(sc, localSecrets),
	)
	progressingConditions = append(progressingConditions, scyllaDBManagerAgentAuthTokenProgressingConditions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	requiredSecrets, err := makeLocalSecrets(sc, scyllaDBManagerAgentAuthToken)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make local secrets: %w", err)
	}

	err = controllerhelpers.Prune(ctx,
		requiredSecrets,
		localSecrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: scc.kubeClient.CoreV1().Secrets(sc.Namespace).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune secret(s) of %q ScyllaDBCluster: %w", naming.ObjRef(sc), err)
	}

	for _, s := range requiredSecrets {
		_, changed, err := resourceapply.ApplySecret(ctx, scc.kubeClient.CoreV1(), scc.secretLister, scc.eventRecorder, s, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, secretControllerProgressingCondition, s, "apply", sc.Generation)
		}
		if err != nil {
			return nil, fmt.Errorf("can't apply secret: %w", err)
		}
	}

	return progressingConditions, nil
}

func getOptionalExistingScyllaDBManagerAuthTokenFunc(sc *scyllav1alpha1.ScyllaDBCluster, localSecrets map[string]*corev1.Secret) func() ([]metav1.Condition, string, error) {
	return func() ([]metav1.Condition, string, error) {
		agentAuthTokenSecretName, err := naming.ScyllaDBManagerAgentAuthTokenSecretNameForScyllaDBCluster(sc)
		if err != nil {
			return nil, "", fmt.Errorf("can't get agent auth token secret name for ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
		}

		secret, ok := localSecrets[agentAuthTokenSecretName]
		if !ok {
			return nil, "", nil
		}

		authToken, err := helpers.GetAgentAuthTokenFromSecret(secret)
		if err != nil {
			return nil, "", fmt.Errorf("can't get agent auth token from Secret %q: %w", naming.ObjRef(secret), err)
		}

		return nil, authToken, nil
	}
}
