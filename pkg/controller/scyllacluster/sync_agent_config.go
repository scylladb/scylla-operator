package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster/resource"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func (scc *Controller) getAgentTokenFromAgentConfig(sc *scyllav1.ScyllaCluster) (string, error) {
	if len(sc.Spec.Datacenter.Racks) == 0 ||
		len(sc.Spec.Datacenter.Racks[0].ScyllaAgentConfig) == 0 {
		return "", nil
	}

	secretName := sc.Spec.Datacenter.Racks[0].ScyllaAgentConfig
	secret, err := scc.secretLister.Secrets(sc.Namespace).Get(secretName)
	if err != nil {
		return "", fmt.Errorf("can't get secret %s/%s: %w", sc.Namespace, secretName, err)
	}

	return helpers.GetAgentAuthTokenFromAgentConfigSecret(secret)
}

func (scc *Controller) syncAgentToken(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	status *scyllav1.ScyllaClusterStatus,
	secrets map[string]*corev1.Secret,
) (*scyllav1.ScyllaClusterStatus, error) {
	token, tokenErr := scc.getAgentTokenFromAgentConfig(sc)
	if tokenErr != nil {
		tokenErr = fmt.Errorf("can't get agent token: %w", tokenErr)
		scc.eventRecorder.Eventf(sc, corev1.EventTypeWarning, "InvalidManagerAgentConfig", "Can't gent agent token: %s", tokenErr.Error())
	}
	// If we can't read a token we still need to secure the manager agent by creating a random one.
	// We handle the error at the end.

	// First we try to retain an already generated token.
	if len(token) == 0 {
		tokenSecretName := naming.AgentAuthTokenSecretName(sc.Name)
		tokenSecret, exists := secrets[tokenSecretName]
		if exists {
			var err error
			token, err = helpers.GetAgentAuthTokenFromSecret(tokenSecret)
			if err != nil {
				return status, fmt.Errorf("can't read token from secret %q: %w", naming.ObjRef(tokenSecret), err)
			}
		}
	}

	// If we still don't have the token, we generate a random one.
	if len(token) == 0 {
		token = rand.String(128)
	}

	secret, err := resource.MakeAgentAuthTokenSecret(sc, token)
	if err != nil {
		return status, fmt.Errorf("can't make auth token secret: %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, _, err = resourceapply.ApplySecret(ctx, scc.kubeClient.CoreV1(), scc.secretLister, scc.eventRecorder, secret, true)
	if err != nil {
		return status, fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(secret), err)
	}

	return status, tokenErr
}
