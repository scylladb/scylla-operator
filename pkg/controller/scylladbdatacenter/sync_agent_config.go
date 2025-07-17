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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func (sdcc *Controller) syncAgentToken(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	secrets map[string]*corev1.Secret,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	agentAuthTokenProgressingConditions, agentAuthToken, err := controllerhelpers.GetScyllaDBManagerAgentAuthToken(
		getOptionalAgentAuthTokenFromCustomConfigFunc(sdc, sdcc.secretLister),
		getOptionalAgentAuthTokenOverrideFunc(sdc, sdcc.secretLister),
		getOptionalExistingAgentAuthTokenFunc(sdc, secrets),
	)
	progressingConditions = append(progressingConditions, agentAuthTokenProgressingConditions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	secret, err := makeAgentAuthTokenSecret(sdc, agentAuthToken)
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

	return progressingConditions, nil
}

func getOptionalAgentAuthTokenFromCustomConfigFunc(sdc *scyllav1alpha1.ScyllaDBDatacenter, secretLister corev1listers.SecretLister) func() ([]metav1.Condition, string, error) {
	return func() ([]metav1.Condition, string, error) {
		var progressingConditions []metav1.Condition

		var configSecret *string
		if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent != nil && sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
			configSecret = sdc.Spec.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef
		}
		if len(sdc.Spec.Racks) > 0 && sdc.Spec.Racks[0].ScyllaDBManagerAgent != nil && sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
			configSecret = sdc.Spec.Racks[0].ScyllaDBManagerAgent.CustomConfigSecretRef
		}
		if configSecret == nil {
			return progressingConditions, "", nil
		}

		secretName := *configSecret
		secret, err := secretLister.Secrets(sdc.Namespace).Get(secretName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return progressingConditions, "", fmt.Errorf("can't get secret %q: %w", naming.ManualRef(sdc.Namespace, secretName), err)
			}

			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               agentTokenControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForSecret",
				Message:            fmt.Sprintf("Waiting for Secret %q to exist.", naming.ManualRef(sdc.Namespace, secretName)),
				ObservedGeneration: sdc.Generation,
			})

			return progressingConditions, "", nil
		}

		authToken, err := helpers.GetAgentAuthTokenFromAgentConfigSecret(secret)
		if err != nil {
			return progressingConditions, "", fmt.Errorf("can't get agent auth token from agent config: %w", err)
		}

		return progressingConditions, authToken, nil
	}
}

func getOptionalAgentAuthTokenOverrideFunc(sdc *scyllav1alpha1.ScyllaDBDatacenter, secretLister corev1listers.SecretLister) func() ([]metav1.Condition, string, error) {
	return func() ([]metav1.Condition, string, error) {
		var progressingConditions []metav1.Condition

		agentAuthTokenOverrideSecretRefAnnotationValue, ok := sdc.Annotations[naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation]
		if !ok {
			return progressingConditions, "", nil
		}

		secret, err := secretLister.Secrets(sdc.Namespace).Get(agentAuthTokenOverrideSecretRefAnnotationValue)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return progressingConditions, "", fmt.Errorf("can't get Secret %q: %w", naming.ManualRef(sdc.Namespace, agentAuthTokenOverrideSecretRefAnnotationValue), err)
			}

			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               agentTokenControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForSecret",
				Message:            fmt.Sprintf("Waiting for Secret %q to exist.", naming.ManualRef(sdc.Namespace, agentAuthTokenOverrideSecretRefAnnotationValue)),
				ObservedGeneration: sdc.Generation,
			})

			return progressingConditions, "", nil
		}

		authToken, err := helpers.GetAgentAuthTokenFromSecret(secret)
		if err != nil {
			return progressingConditions, "", fmt.Errorf("can't get agent auth token from Secret %q: %w", naming.ObjRef(secret), err)
		}

		return progressingConditions, authToken, nil
	}
}

func getOptionalExistingAgentAuthTokenFunc(sdc *scyllav1alpha1.ScyllaDBDatacenter, secrets map[string]*corev1.Secret) func() ([]metav1.Condition, string, error) {
	return func() ([]metav1.Condition, string, error) {
		var progressingConditions []metav1.Condition

		secret, ok := secrets[naming.AgentAuthTokenSecretName(sdc)]
		if !ok {
			return progressingConditions, "", nil
		}

		authToken, err := helpers.GetAgentAuthTokenFromSecret(secret)
		if err != nil {
			return progressingConditions, "", fmt.Errorf("can't get agent auth token from Secret %q: %w", naming.ObjRef(secret), err)
		}

		return progressingConditions, authToken, nil
	}
}
