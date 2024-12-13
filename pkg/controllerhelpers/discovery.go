package controllerhelpers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memcacheddiscovery "k8s.io/client-go/discovery/cached/memory"
)

func IsSCCAvailable(client discovery.DiscoveryInterface) (bool, error) {
	enabled, err := discovery.IsResourceEnabled(
		client,
		schema.GroupVersionResource{
			Group:    "security.openshift.io",
			Version:  "v1",
			Resource: "securitycontextconstraints",
		},
	)
	if err != nil {
		// Kubernetes has inconsistent errors between a real discovery client and memcached client,
		// so we have to do an extra "not found" check here.
		if errors.Is(err, memcacheddiscovery.ErrCacheNotFound) {
			return false, nil
		}

		return false, err
	}

	return enabled, nil
}
