package collect_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FakeServerPreferredResourcesGetter struct {
	Resources []*metav1.APIResourceList
	Error     error
}

func (f *FakeServerPreferredResourcesGetter) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.Resources, f.Error
}

// Based on a realistic response from a Kubernetes API server.
var fakeResources = []*metav1.APIResourceList{
	{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{
				Name:       "pods",
				Namespaced: true,
				Kind:       "Pod",
				Verbs:      []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				Name:       "secrets",
				Namespaced: true,
				Kind:       "Secret",
				Verbs:      []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	},
	{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{
			{
				Name:       "deployments",
				Namespaced: true,
				Kind:       "Deployment",
				Verbs:      []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				Name:       "statefulsets",
				Namespaced: true,
				Kind:       "StatefulSet",
				Verbs:      []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	},
	{
		GroupVersion: "scylla.scylladb.com/v1",
		APIResources: []metav1.APIResource{
			{
				Name:       "scyllaclusters",
				Namespaced: true,
				Kind:       "ScyllaCluster",
				Verbs:      []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	},
}

func TestResourceDiscoverer(t *testing.T) {
	testCases := []struct {
		name                                string
		resourcesToExclude                  []schema.GroupKind
		includeSensitiveResources           bool
		serverPreferredResourcesGetterError error

		expectedDiscoveredResources []*collect.ResourceInfo
		expectError                 bool
	}{
		{
			name:                      "No custom exclusions",
			resourcesToExclude:        []schema.GroupKind{},
			includeSensitiveResources: false,
			expectedDiscoveredResources: []*collect.ResourceInfo{
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				},
				// Secret is excluded by default.
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "scylla.scylladb.com", Version: "v1", Resource: "scyllaclusters"},
				},
			},
		},
		{
			name:                      "Include sensitive resources",
			resourcesToExclude:        []schema.GroupKind{},
			includeSensitiveResources: true,
			expectedDiscoveredResources: []*collect.ResourceInfo{
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				},
				// Secret is included as sensitive resources are allowed.
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "scylla.scylladb.com", Version: "v1", Resource: "scyllaclusters"},
				},
			},
		},
		{
			name: "Exclude resources",
			resourcesToExclude: []schema.GroupKind{
				{Group: "scylla.scylladb.com", Kind: "ScyllaCluster"},
				{Group: "", Kind: "Pod"},
			},
			includeSensitiveResources: false,
			expectedDiscoveredResources: []*collect.ResourceInfo{
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
				},
			},
		},
		{
			name: "Exclude resource with different case kind",
			resourcesToExclude: []schema.GroupKind{
				{Group: "", Kind: "pod"},
				{Group: "scylla.scylladb.com", Kind: "SCYLLACLUSTER"},
			},
			includeSensitiveResources: false,
			expectedDiscoveredResources: []*collect.ResourceInfo{
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				},
				{
					Scope:    meta.RESTScopeNamespace,
					Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
				},
			},
		},
		{
			name:                                "PreferredResourcesGetter returns error",
			includeSensitiveResources:           false,
			serverPreferredResourcesGetterError: errors.New("failed to get preferred resources"),
			expectedDiscoveredResources:         nil,
			expectError:                         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeServerPreferredResourcesGetter := &FakeServerPreferredResourcesGetter{
				Resources: fakeResources,
				Error:     tc.serverPreferredResourcesGetterError,
			}
			discoverer := collect.NewResourceDiscoverer(tc.includeSensitiveResources, fakeServerPreferredResourcesGetter)
			discoveredResources, err := discoverer.DiscoverResources(tc.resourcesToExclude...)
			if err == nil && tc.expectError {
				t.Fatalf("Expected error but got none")
			} else if err != nil && !tc.expectError {
				t.Fatalf("Unexpected error: %v", err)
			}

			cmpOpts := []cmp.Option{
				// Make sure the slices are compared in a consistent order.
				cmpopts.SortSlices(func(a, b *collect.ResourceInfo) bool {
					return a.Resource.String() < b.Resource.String()
				}),
				// Scope cannot be compared directly because meta.RESTScopeNamespace is unexported.
				cmp.Comparer(func(x, y *collect.ResourceInfo) bool {
					return x.Scope.Name() == y.Scope.Name() && x.Resource == y.Resource
				}),
			}
			if diff := cmp.Diff(discoveredResources, tc.expectedDiscoveredResources, cmpOpts...); diff != "" {
				t.Errorf("Discovered resources mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
