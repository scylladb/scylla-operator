package scylladbdatacenter

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
	"k8s.io/apimachinery/pkg/util/rand"
)

func (sdcc *Controller) getAgentTokenFromAgentConfig(sdc *scyllav1alpha1.ScyllaDBDatacenter) (string, error) {
	if len(sdc.Spec.Racks) == 0 {
		return "", nil
	}

	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent == nil {
		return "", nil
	}

	var configSecret *string
	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		configSecret = sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef
	}
	if sdc.Spec.Racks[0].ScyllaDBManagerAgent != nil && sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		configSecret = sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef
	}
	if configSecret == nil {
		return "", nil
	}

	secretName := *configSecret
	secret, err := sdcc.secretLister.Secrets(sdc.Namespace).Get(secretName)
	if err != nil {
		return "", fmt.Errorf("can't get secret %s/%s: %w", sdc.Namespace, secretName, err)
	}

	return helpers.GetAgentAuthTokenFromAgentConfigSecret(secret)
}

func (sdcc *Controller) syncAgentToken(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	secrets map[string]*corev1.Secret,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	token, tokenErr := sdcc.getAgentTokenFromAgentConfig(sdc)
	if tokenErr != nil {
		tokenErr = fmt.Errorf("can't get agent token: %w", tokenErr)
		sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeWarning, "InvalidManagerAgentConfig", "Can't gent agent token: %s", tokenErr.Error())
	}
	// If we can't read a token we still need to secure the manager agent by creating a random one.
	// We handle the error at the end.

	// First we try to retain an already generated token.
	if len(token) == 0 {
		tokenSecretName := naming.AgentAuthTokenSecretName(sdc)
		tokenSecret, exists := secrets[tokenSecretName]
		if exists {
			var err error
			token, err = helpers.GetAgentAuthTokenFromSecret(tokenSecret)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't read token from secret %q: %w", naming.ObjRef(tokenSecret), err)
			}
		}
	}

	// If we still don't have the token, we generate a random one.
	if len(token) == 0 {
		token = rand.String(128)
	}

	secret, err := MakeAgentAuthTokenSecret(sdc, token)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make auth token secret: %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, changed, err := resourceapply.ApplySecret(ctx, sdcc.kubeClient.CoreV1(), sdcc.secretLister, sdcc.eventRecorder, secret, resourceapply.ApplyOptions{
		ForceOwnership: true,
	})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, agentTokenControllerProgressingCondition, secret, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(secret), err)
	}

	return progressingConditions, tokenErr
}
