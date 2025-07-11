// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
)

func GetScyllaDBManagerClient(_ context.Context, _ *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*managerclient.Client, error) {
	url := fmt.Sprintf("http://%s.%s.svc/api/v1", naming.ScyllaManagerServiceName, naming.ScyllaManagerNamespace)
	managerClient, err := managerclient.NewClient(url, func(httpClient *http.Client) {
		// FIXME: https://github.com/scylladb/scylla-operator/issues/2693
		httpClient.Transport = http.DefaultTransport
		// Limit manager calls by default to a higher bound.
		// Individual calls can still be further limited using context.
		// Manager is prone to extremely long calls because it (unfortunately) retries errors internally.
		httpClient.Timeout = 15 * time.Second
	})
	if err != nil {
		return nil, fmt.Errorf("can't build manager client: %w", err)
	}

	return &managerClient, nil
}

func IsManagedByGlobalScyllaDBManagerInstance(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) bool {
	return naming.GlobalScyllaDBManagerClusterRegistrationSelector().Matches(labels.Set(smcr.GetLabels()))
}

const (
	authTokenSize = 128
)

type ScyllaDBManagerAgentCustomConfigError struct {
	error
}

func (e ScyllaDBManagerAgentCustomConfigError) Unwrap() error {
	return e.error
}

func NewScyllaDBManagerAgentCustomConfigError(err error) error {
	return ScyllaDBManagerAgentCustomConfigError{err}
}

var _ error = (*ScyllaDBManagerAgentCustomConfigError)(nil)

func GetScyllaDBManagerAgentAuthTokenConfig(
	getOptionalCustomAgentConfigSecret func() ([]metav1.Condition, *corev1.Secret, error),
	getOptionalExistingAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
	continueOnCustomAgentConfigError bool,
) ([]metav1.Condition, []byte, error) {
	return getScyllaDBManagerAgentAuthTokenConfig(
		getOptionalCustomAgentConfigSecret,
		getOptionalExistingAuthTokenSecret,
		continueOnCustomAgentConfigError,
		func() string {
			return apimachineryutilrand.String(authTokenSize)
		},
	)
}

func getScyllaDBManagerAgentAuthTokenConfig(
	getOptionalCustomAgentConfigSecret func() ([]metav1.Condition, *corev1.Secret, error),
	getOptionalExistingAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
	continueOnCustomAgentConfigError bool,
	generateAuthToken func() string,
) ([]metav1.Condition, []byte, error) {
	progressingConditions, authToken, err := getScyllaDBManagerAgentAuthToken(
		getOptionalCustomAgentConfigSecret,
		getOptionalExistingAuthTokenSecret,
		continueOnCustomAgentConfigError,
		generateAuthToken,
	)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil, nil
	}

	authTokenConfig, err := helpers.GetAgentAuthTokenConfig(authToken)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}
	return nil, authTokenConfig, nil
}

func getScyllaDBManagerAgentAuthToken(
	getOptionalCustomAgentConfigSecret func() ([]metav1.Condition, *corev1.Secret, error),
	getOptionalExistingAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
	continueOnCustomAgentConfigError bool,
	generateAuthToken func() string,
) ([]metav1.Condition, string, error) {
	var customConfigError error

	// User-defined config should take precedence over the operator-generated tokens.
	progressingConditions, authToken, err := getScyllaDBManagerAgentAuthTokenFromAgentConfigSecret(getOptionalCustomAgentConfigSecret)
	if err != nil {
		customConfigError = NewScyllaDBManagerAgentCustomConfigError(fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", err))

		// For backward compatibility, provide an option to continue on a custom agent config error.
		if !continueOnCustomAgentConfigError {
			return progressingConditions, "", customConfigError
		}
		// TODO: defer and join errors instead?
	}
	if len(progressingConditions) > 0 || len(authToken) > 0 {
		return progressingConditions, authToken, customConfigError
	}

	// Try to retain the existing auth token if it exists.
	progressingConditions, authToken, err = getScyllaDBManagerAgentAuthTokenFromExistingSecret(getOptionalExistingAuthTokenSecret)
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token from existing secret: %w", err)
	}
	if len(progressingConditions) > 0 || len(authToken) > 0 {
		return progressingConditions, authToken, customConfigError
	}

	// Generate a new auth token.
	return nil, generateAuthToken(), customConfigError
}

func getScyllaDBManagerAgentAuthTokenFromAgentConfigSecret(
	getOptionalCustomAgentConfigSecret func() ([]metav1.Condition, *corev1.Secret, error),
) ([]metav1.Condition, string, error) {
	return getScyllaDBManagerAgentAuthTokenFromSecret(getOptionalCustomAgentConfigSecret, helpers.GetAgentAuthTokenFromAgentConfigSecret)
}

func getScyllaDBManagerAgentAuthTokenFromExistingSecret(
	getOptionalExistingAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
) ([]metav1.Condition, string, error) {
	return getScyllaDBManagerAgentAuthTokenFromSecret(getOptionalExistingAuthTokenSecret, helpers.GetAgentAuthTokenFromSecret)
}

func getScyllaDBManagerAgentAuthTokenFromSecret(
	getOptionalAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
	extractAuthTokenFromSecret func(secret *corev1.Secret) (string, error),
) ([]metav1.Condition, string, error) {
	progressingConditions, optionalAuthTokenSecret, err := getOptionalAuthTokenSecret()
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token secret: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, "", nil
	}
	if optionalAuthTokenSecret == nil {
		return nil, "", nil
	}

	authToken, err := extractAuthTokenFromSecret(optionalAuthTokenSecret)
	if err != nil {
		return nil, "", fmt.Errorf("can't extract ScyllaDB Manager agent auth token from Secret %q: %w", naming.ObjRef(optionalAuthTokenSecret), err)
	}

	return nil, authToken, nil
}
