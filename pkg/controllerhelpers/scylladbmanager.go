// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/apimachinery/pkg/labels"
)

func GetScyllaDBManagerClient(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*managerclient.Client, error) {
	if !IsManagedByGlobalScyllaDBManagerInstance(smcr) {
		return nil, controllertools.NewNonRetriable(fmt.Sprintf("ScyllaDBManagerClusterRegistration %q is not managed by global ScyllaDB Manager instance", naming.ObjRef(smcr)))
	}

	url := fmt.Sprintf("http://%s.%s.svc/api/v1", naming.ScyllaManagerServiceName, naming.ScyllaManagerNamespace)
	managerClient, err := managerclient.NewClient(url, func(httpClient *http.Client) {
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
