package scylladbdatacenter

import (
	"context"
	"errors"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func getOptionalAgentCustomConfigSecret(sdc *scyllav1alpha1.ScyllaDBDatacenter, secretLister corev1listers.SecretLister) ([]metav1.Condition, *corev1.Secret, error) {
	var progressingConditions []metav1.Condition

	if len(sdc.Spec.Racks) == 0 {
		return progressingConditions, nil, nil
	}

	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent == nil {
		return progressingConditions, nil, nil
	}

	var configSecret *string
	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		configSecret = sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef
	}
	if sdc.Spec.Racks[0].ScyllaDBManagerAgent != nil && sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		configSecret = sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef
	}
	if configSecret == nil {
		return progressingConditions, nil, nil
	}

	secretName := *configSecret
	secret, err := secretLister.Secrets(sdc.Namespace).Get(secretName)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't get secret %s/%s: %w", sdc.Namespace, secretName, err)
	}

	return progressingConditions, secret, nil
}

func (sdcc *Controller) syncAgentToken(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	secrets map[string]*corev1.Secret,
) ([]metav1.Condition, error) {
	getOptionalCustomAgentConfigSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		return getOptionalAgentCustomConfigSecret(sdc, sdcc.secretLister)
	}
	getOptionalExistingAuthTokenSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		return getOptionalExistingAgentAuthTokenSecret(sdc, secrets, sdcc.secretLister)
	}

	// We allow the controller to continue on a custom agent config error,
	// so that a misconfigured or missing custom agent config does not block the controller from creating the auth token secret.
	// In case the returned error is a custom agent config error, we continue and return the error at the end of the function.
	var customAgentConfigError error
	progressingConditions, agentAuthTokenConfig, err := controllerhelpers.GetScyllaDBManagerAgentAuthTokenConfig(
		getOptionalCustomAgentConfigSecret,
		getOptionalExistingAuthTokenSecret,
		true,
	)
	if err != nil {
		if !errors.As(err, &controllerhelpers.ScyllaDBManagerAgentCustomConfigError{}) {
			return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
		}

		// Return the custom agent config error at the end.
		sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeWarning, "InvalidManagerAgentConfig", "Can't gent agent token: %s", err.Error())
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	secret, err := makeAgentAuthTokenSecret(sdc, agentAuthTokenConfig)
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

	return progressingConditions, customAgentConfigError
}

func getOptionalExistingAgentAuthTokenSecret(sdc *scyllav1alpha1.ScyllaDBDatacenter, secrets map[string]*corev1.Secret, secretLister corev1listers.SecretLister) ([]metav1.Condition, *corev1.Secret, error) {
	// The overriding Secret must take precedence over the existing token.
	progressingConditions, overrideSecret, err := getOptionalAgentAuthTokenOverrideSecret(sdc, secretLister)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't get agent auth token override secret for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}
	if len(progressingConditions) > 0 || overrideSecret != nil {
		return progressingConditions, overrideSecret, nil
	}

	existingAgentAuthTokenSecret, ok := secrets[naming.AgentAuthTokenSecretName(sdc)]
	if !ok {
		return nil, nil, nil
	}

	return nil, existingAgentAuthTokenSecret, nil
}

func getOptionalAgentAuthTokenOverrideSecret(sdc *scyllav1alpha1.ScyllaDBDatacenter, secretLister corev1listers.SecretLister) ([]metav1.Condition, *corev1.Secret, error) {
	var progressingConditions []metav1.Condition

	agentAuthTokenOverrideSecretRefAnnotationValue, hasAgentAuthTokenOverrideSecretRefAnnotation := sdc.Annotations[naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation]
	if !hasAgentAuthTokenOverrideSecretRefAnnotation {
		return nil, nil, nil
	}

	secret, err := secretLister.Secrets(sdc.Namespace).Get(agentAuthTokenOverrideSecretRefAnnotationValue)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return progressingConditions, nil, fmt.Errorf("can't get secret %q: %w", naming.ManualRef(sdc.Namespace, agentAuthTokenOverrideSecretRefAnnotationValue), err)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               agentTokenControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForSecret",
			Message:            fmt.Sprintf("Waiting for Secret %q to exist.", naming.ManualRef(sdc.Namespace, agentAuthTokenOverrideSecretRefAnnotationValue)),
			ObservedGeneration: sdc.Generation,
		})

		return progressingConditions, nil, nil
	}

	return progressingConditions, secret, nil
}
