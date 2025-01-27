package remotekubernetescluster

import (
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testSetupTimeout    = 1 * time.Minute
	testTeardownTimeout = 1 * time.Minute
	testTimeout         = 30 * time.Minute
)

var (
	remoteKubernetesClusterResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    scyllav1alpha1.GroupName,
			Version:  scyllav1alpha1.GroupVersion.Version,
			Resource: "remotekubernetesclusters",
		},
		Scope: meta.RESTScopeRoot,
	}
)
