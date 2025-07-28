// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
)

var httpDefaultTransport = http.DefaultTransport.(*http.Transport).Clone()

func GetScyllaDBManagerClient(_ context.Context, _ *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*managerclient.Client, error) {
	url := fmt.Sprintf("http://%s.%s.svc/api/v1", naming.ScyllaManagerServiceName, naming.ScyllaManagerNamespace)
	managerClient, err := managerclient.NewClient(url, func(httpClient *http.Client) {
		httpClient.Transport = httpDefaultTransport
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

// GetScyllaDBManagerAgentAuthToken retrieves the ScyllaDB Manager agent auth token.
// It greedily gets the auth tokens from the provided functions, returning on the first non-empty result.
// If no auth token is provided by any of the sources, a new auth token is generated.
func GetScyllaDBManagerAgentAuthToken(
	getAuthTokens ...func() ([]metav1.Condition, string, error),
) ([]metav1.Condition, string, error) {
	return getScyllaDBManagerAgentAuthToken(
		newScyllaDBManagerAuthToken,
		getAuthTokens...,
	)
}

func getScyllaDBManagerAgentAuthToken(
	generateAuthToken func() string,
	getAuthTokens ...func() ([]metav1.Condition, string, error),
) ([]metav1.Condition, string, error) {
	var progressingConditions []metav1.Condition
	var authToken string
	var err error

	for _, getAuthToken := range getAuthTokens {
		progressingConditions, authToken, err = getAuthToken()
		if err != nil {
			return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", err)
		}
		if len(progressingConditions) > 0 || len(authToken) > 0 {
			// If any of the sources provide a non-empty auth token or progressing conditions, return early.
			return progressingConditions, authToken, nil
		}
	}

	// Generate a new auth token if no source provided one.
	authToken = generateAuthToken()
	return progressingConditions, authToken, nil
}
