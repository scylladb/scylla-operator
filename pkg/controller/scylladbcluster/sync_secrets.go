package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	getOptionalScyllaDBManagerCustomAgentConfigSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		return getOptionalScyllaDBManagerAgentCustomConfigSecret(sc, scc.secretLister)
	}

	getOptionalExistingScyllaDBManagerAgentAuthTokenSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		secret, err := getOptionalExistingScyllaDBManagerAuthTokenSecret(sc, localSecrets)
		return nil, secret, err
	}

	progressingConditions, scyllaDBManagerAgentAuthTokenConfig, err := controllerhelpers.GetScyllaDBManagerAgentAuthTokenConfig(
		getOptionalScyllaDBManagerCustomAgentConfigSecret,
		getOptionalExistingScyllaDBManagerAgentAuthTokenSecret,
		false,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	requiredSecrets, err := makeLocalSecrets(sc, scyllaDBManagerAgentAuthTokenConfig)
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

func getOptionalScyllaDBManagerAgentCustomConfigSecret(sc *scyllav1alpha1.ScyllaDBCluster, secretLister corev1listers.SecretLister) ([]metav1.Condition, *corev1.Secret, error) {
	var progressingConditions []metav1.Condition

	if sc.Spec.DatacenterTemplate == nil || sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent == nil || sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef == nil {
		return progressingConditions, nil, nil
	}

	agentCustomConfigSecret, err := secretLister.Secrets(sc.Namespace).Get(*sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               secretControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForSecret",
				Message:            fmt.Sprintf("Waiting for Secret %q to exist.", naming.ManualRef(sc.Namespace, *sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef)),
				ObservedGeneration: sc.Generation,
			})
			return progressingConditions, nil, nil
		}

		return progressingConditions, nil, fmt.Errorf("can't get %q Secret: %w", naming.ManualRef(sc.Namespace, *sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef), err)
	}

	return progressingConditions, agentCustomConfigSecret, nil
}

func getOptionalExistingScyllaDBManagerAuthTokenSecret(sc *scyllav1alpha1.ScyllaDBCluster, localSecrets map[string]*corev1.Secret) (*corev1.Secret, error) {
	agentAuthTokenSecretName, err := naming.ScyllaDBManagerAgentAuthTokenSecretNameForScyllaDBCluster(sc)
	if err != nil {
		return nil, fmt.Errorf("can't get agent auth token secret name for ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	agentAuthTokenSecret, ok := localSecrets[agentAuthTokenSecretName]
	if !ok {
		return nil, nil
	}

	return agentAuthTokenSecret, nil
}
