package helpers

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestIsAPIGroupVersionAvailable(t *testing.T) {
	tests := []struct {
		name         string
		groupVersion string
		resources    []*metav1.APIResourceList
		expected     bool
	}{
		{
			name:         "returns true when the API group version exists",
			groupVersion: "monitoring.coreos.com/v1",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "monitoring.coreos.com/v1",
					APIResources: []metav1.APIResource{
						{Name: "prometheuses", Kind: "Prometheus"},
						{Name: "servicemonitors", Kind: "ServiceMonitor"},
						{Name: "prometheusrules", Kind: "PrometheusRule"},
					},
				},
			},
			expected: true,
		},
		{
			name:         "returns false when the API group version does not exist",
			groupVersion: "monitoring.coreos.com/v1",
			resources:    []*metav1.APIResourceList{},
			expected:     false,
		},
		{
			name:         "returns false when a different API group version exists",
			groupVersion: "monitoring.coreos.com/v1",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "deployments", Kind: "Deployment"},
					},
				},
			},
			expected: false,
		},
		{
			name:         "returns true for core API group version",
			groupVersion: "v1",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Kind: "Pod"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeclientset.NewClientset()
			fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
			if !ok {
				t.Fatal("failed to get fake discovery client")
			}
			fakeDiscovery.Resources = tc.resources

			got, err := IsAPIGroupVersionAvailable(fakeDiscovery, tc.groupVersion)
			if err != nil {
				t.Fatalf("IsAPIGroupVersionAvailable(%q) returned unexpected error: %v", tc.groupVersion, err)
			}
			if got != tc.expected {
				t.Errorf("IsAPIGroupVersionAvailable(%q) = %v, expected %v", tc.groupVersion, got, tc.expected)
			}
		})
	}
}

func TestIsAPIGroupVersionAvailable_DiscoveryError(t *testing.T) {
	client := fakeclientset.NewClientset()
	fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatal("failed to get fake discovery client")
	}
	fakeDiscovery.PrependReactor("*", "*", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated API server error")
	})

	got, err := IsAPIGroupVersionAvailable(fakeDiscovery, "monitoring.coreos.com/v1")
	if err == nil {
		t.Fatal("IsAPIGroupVersionAvailable should return an error for transient failures")
	}
	if got != false {
		t.Errorf("IsAPIGroupVersionAvailable with error = %v, expected false", got)
	}
}
