package helpers

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
)

// IsAPIGroupVersionAvailable checks whether the given API group version
// (e.g. "monitoring.coreos.com/v1") is available in the cluster by querying
// the API server's discovery endpoint.
// It returns (false, nil) when the group version is not found, and a non-nil
// error for transient or unexpected failures (e.g. network issues, server errors).
func IsAPIGroupVersionAvailable(discoveryClient discovery.DiscoveryInterface, groupVersion string) (bool, error) {
	_, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err == nil {
		return true, nil
	}

	if apierrors.IsNotFound(err) {
		return false, nil
	}

	return false, fmt.Errorf("can't discover API group version %q: %w", groupVersion, err)
}
