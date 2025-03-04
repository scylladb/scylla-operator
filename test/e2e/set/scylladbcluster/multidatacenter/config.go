package multidatacenter

import (
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"
)

const (
	testTimeout = 1 * time.Hour
)

var (
	remoteKubernetesClusterResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    "scylla.scylladb.com",
			Version:  "v1alpha1",
			Resource: "remotekubernetesclusters",
		},
		Scope: meta.RESTScopeRoot,
	}
)
