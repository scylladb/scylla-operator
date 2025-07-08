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

func newScyllaDBManagerAuthToken() string {
	return apimachineryutilrand.String(authTokenSize)
}

// GetScyllaDBManagerAgentAuthTokenConfigOptions defines options for selecting ScyllaDB Manager agent auth token config.
type GetScyllaDBManagerAgentAuthTokenConfigOptions struct {
	GetOptionalAgentAuthTokenFromCustomConfig func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error)
	GetOptionalAgentAuthTokenFromExisting     func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error)
}

func GetScyllaDBManagerAgentAuthTokenConfig(
	options GetScyllaDBManagerAgentAuthTokenConfigOptions,
) ([]metav1.Condition, []byte, error) {
	return getScyllaDBManagerAgentAuthTokenConfig(
		newScyllaDBManagerAuthToken,
		options,
	)
}

func getScyllaDBManagerAgentAuthTokenConfig(
	newAuthToken func() string,
	options GetScyllaDBManagerAgentAuthTokenConfigOptions,
) ([]metav1.Condition, []byte, error) {
	var progressingConditions []metav1.Condition

	authTokenProgressingConditions, authToken, err := getScyllaDBManagerAgentAuthToken(
		newAuthToken,
		options,
	)
	progressingConditions = append(progressingConditions, authTokenProgressingConditions...)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", err)
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil, nil
	}

	authTokenConfig, err := helpers.GetAgentAuthTokenConfig(authToken)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't get ScyllaDB Manager agent auth token config: %w", err)
	}
	return progressingConditions, authTokenConfig, nil
}

func getScyllaDBManagerAgentAuthToken(
	generateAuthToken func() string,
	options GetScyllaDBManagerAgentAuthTokenConfigOptions,
) ([]metav1.Condition, string, error) {
	var progressingConditions []metav1.Condition

	// User-defined config should take precedence over the operator-generated tokens.
	customConfigProgressingConditions, authToken, err := options.GetOptionalAgentAuthTokenFromCustomConfig(helpers.GetAgentAuthTokenFromAgentConfigSecret)
	progressingConditions = append(progressingConditions, customConfigProgressingConditions...)
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", err)
	}
	if len(progressingConditions) > 0 || len(authToken) > 0 {
		return progressingConditions, authToken, nil
	}

	// Try to retain the existing auth token if it exists.
	existingProgressingConditions, authToken, err := options.GetOptionalAgentAuthTokenFromExisting(helpers.GetAgentAuthTokenFromSecret)
	progressingConditions = append(progressingConditions, existingProgressingConditions...)
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token from existing secret: %w", err)
	}
	if len(progressingConditions) > 0 || len(authToken) > 0 {
		return progressingConditions, authToken, nil
	}

	// Generate a new auth token.
	return progressingConditions, generateAuthToken(), nil
}

func GetScyllaDBManagerAgentAuthTokenFromSecret(
	getOptionalAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error),
	extractAuthTokenFromSecret func(secret *corev1.Secret) (string, error),
) ([]metav1.Condition, string, error) {
	var progressingConditions []metav1.Condition

	secretProgressingConditions, optionalAuthTokenSecret, err := getOptionalAuthTokenSecret()
	progressingConditions = append(progressingConditions, secretProgressingConditions...)
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token secret: %w", err)
	}
	if len(progressingConditions) > 0 || optionalAuthTokenSecret == nil {
		return progressingConditions, "", nil
	}

	authToken, err := extractAuthTokenFromSecret(optionalAuthTokenSecret)
	if err != nil {
		return progressingConditions, "", fmt.Errorf("can't extract ScyllaDB Manager agent auth token from Secret %q: %w", naming.ObjRef(optionalAuthTokenSecret), err)
	}

	return progressingConditions, authToken, nil
}
