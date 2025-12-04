package helpers

import (
	"net"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "IPv4 address",
			ip:       "192.168.1.1",
			expected: false,
		},
		{
			name:     "IPv6 address",
			ip:       "2001:db8::1",
			expected: true,
		},
		{
			name:     "IPv6 loopback",
			ip:       "::1",
			expected: true,
		},
		{
			name:     "IPv4 loopback",
			ip:       "127.0.0.1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			if got := IsIPv6(ip); got != tt.expected {
				t.Errorf("IsIPv6() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "IPv4 address",
			ip:       "192.168.1.1",
			expected: true,
		},
		{
			name:     "IPv6 address",
			ip:       "2001:db8::1",
			expected: false,
		},
		{
			name:     "IPv6 loopback",
			ip:       "::1",
			expected: false,
		},
		{
			name:     "IPv4 loopback",
			ip:       "127.0.0.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			if got := IsIPv4(ip); got != tt.expected {
				t.Errorf("IsIPv4() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetIPFamily(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected corev1.IPFamily
	}{
		{
			name:     "IPv4 address",
			ip:       "192.168.1.1",
			expected: corev1.IPv4Protocol,
		},
		{
			name:     "IPv6 address",
			ip:       "2001:db8::1",
			expected: corev1.IPv6Protocol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			if got := GetIPFamily(ip); got != tt.expected {
				t.Errorf("GetIPFamily() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPreferredIP(t *testing.T) {
	tests := []struct {
		name            string
		ips             []string
		preferredFamily *corev1.IPFamily
		expected        string
		expectError     bool
	}{
		{
			name:        "empty IP list",
			ips:         []string{},
			expected:    "",
			expectError: true,
		},
		{
			name:     "no preference, single IPv4",
			ips:      []string{"192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "no preference, single IPv6",
			ips:      []string{"2001:db8::1"},
			expected: "2001:db8::1",
		},
		{
			name:            "prefer IPv4, both available",
			ips:             []string{"2001:db8::1", "192.168.1.1"},
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "192.168.1.1",
		},
		{
			name:            "prefer IPv6, both available",
			ips:             []string{"192.168.1.1", "2001:db8::1"},
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "2001:db8::1",
		},
		{
			name:            "prefer IPv6, only IPv4 available",
			ips:             []string{"192.168.1.1"},
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "192.168.1.1",
		},
		{
			name:            "prefer IPv4, only IPv6 available",
			ips:             []string{"2001:db8::1"},
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPreferredIP(tt.ips, tt.preferredFamily)
			if tt.expectError {
				if err == nil {
					t.Errorf("GetPreferredIP() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPreferredIP() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("GetPreferredIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPreferredPodIP(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		preferredFamily *corev1.IPFamily
		expected        string
		expectError     bool
	}{
		{
			name:        "nil pod",
			pod:         nil,
			expectError: true,
		},
		{
			name: "pod with no IPs",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{},
			},
			expectError: true,
		},
		{
			name: "pod with single IPv4",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					PodIP: "192.168.1.1",
				},
			},
			expected: "192.168.1.1",
		},
		{
			name: "pod with dual stack, prefer IPv4",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					PodIP: "192.168.1.1",
					PodIPs: []corev1.PodIP{
						{IP: "192.168.1.1"},
						{IP: "2001:db8::1"},
					},
				},
			},
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "192.168.1.1",
		},
		{
			name: "pod with dual stack, prefer IPv6",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					PodIP: "192.168.1.1",
					PodIPs: []corev1.PodIP{
						{IP: "192.168.1.1"},
						{IP: "2001:db8::1"},
					},
				},
			},
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPreferredPodIP(tt.pod, tt.preferredFamily)
			if tt.expectError {
				if err == nil {
					t.Errorf("GetPreferredPodIP() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPreferredPodIP() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("GetPreferredPodIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPreferredServiceIP(t *testing.T) {
	tests := []struct {
		name            string
		svc             *corev1.Service
		preferredFamily *corev1.IPFamily
		expected        string
		expectError     bool
	}{
		{
			name:        "nil service",
			svc:         nil,
			expectError: true,
		},
		{
			name: "service with None ClusterIP",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "None",
				},
			},
			expectError: true,
		},
		{
			name: "service with single IPv4 ClusterIP",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.96.0.1",
					ClusterIPs: []string{"10.96.0.1"},
				},
			},
			expected: "10.96.0.1",
		},
		{
			name: "service with dual stack, prefer IPv4",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.96.0.1",
					ClusterIPs: []string{"10.96.0.1", "fd00::1"},
				},
			},
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "10.96.0.1",
		},
		{
			name: "service with dual stack, prefer IPv6",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.96.0.1",
					ClusterIPs: []string{"10.96.0.1", "fd00::1"},
				},
			},
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "fd00::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPreferredServiceIP(tt.svc, tt.preferredFamily)
			if tt.expectError {
				if err == nil {
					t.Errorf("GetPreferredServiceIP() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPreferredServiceIP() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("GetPreferredServiceIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPreferredServiceIPFamily(t *testing.T) {
	tests := []struct {
		name     string
		service  *corev1.Service
		expected corev1.IPFamily
	}{
		{
			name:     "nil service - defaults to IPv4",
			service:  nil,
			expected: corev1.IPv4Protocol,
		},
		{
			name: "service with no IP families - defaults to IPv4",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "service with IPv4 family first - uses IPv4",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
				},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "service with IPv6 family first - uses IPv6",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol},
				},
			},
			expected: corev1.IPv6Protocol,
		},
		{
			name: "service with only IPv6 family - uses IPv6",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol},
				},
			},
			expected: corev1.IPv6Protocol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPreferredServiceIPFamily(tt.service)
			if result != tt.expected {
				t.Errorf("GetPreferredServiceIPFamily() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestAutoDetectionBehavior tests the auto-detection logic for backward compatibility
func TestAutoDetectionBehavior(t *testing.T) {
	tests := []struct {
		name           string
		service        *corev1.Service
		expectedFamily corev1.IPFamily
		description    string
	}{
		{
			name: "legacy service (no IP families) - should default to IPv4",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "legacy-svc"},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.96.0.1",
					ClusterIPs: []string{"10.96.0.1"},
					// No IPFamilies field - legacy behavior
				},
			},
			expectedFamily: corev1.IPv4Protocol,
			description:    "Existing clusters without IPv6 config should continue using IPv4",
		},
		{
			name: "dual-stack service (IPv4 first) - should use IPv4",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "dual-stack-svc"},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.96.0.1",
					ClusterIPs: []string{"10.96.0.1", "fd00::1"},
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
				},
			},
			expectedFamily: corev1.IPv4Protocol,
			description:    "Dual-stack with IPv4 first should use IPv4 for ScyllaDB",
		},
		{
			name: "IPv6-only service - should use IPv6",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "ipv6-only-svc"},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "fd00::1",
					ClusterIPs: []string{"fd00::1"},
					IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol},
				},
			},
			expectedFamily: corev1.IPv6Protocol,
			description:    "IPv6-only clusters should use IPv6 for ScyllaDB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the auto-detection logic
			detectedFamily := GetPreferredServiceIPFamily(tt.service)

			if detectedFamily != tt.expectedFamily {
				t.Errorf("Auto-detection failed: %s\nExpected %v, got %v",
					tt.description, tt.expectedFamily, detectedFamily)
			}

			t.Logf("%s: Auto-detected %v as expected", tt.description, detectedFamily)
		})
	}
}

func TestDetectEndpointsIPFamily(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []discoveryv1.Endpoint
		expected  corev1.IPFamily
	}{
		{
			name:      "empty endpoints - defaults to IPv4",
			endpoints: []discoveryv1.Endpoint{},
			expected:  corev1.IPv4Protocol,
		},
		{
			name: "endpoints with no addresses - defaults to IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{}},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "single IPv4 endpoint - detects IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"192.168.1.1"}},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "single IPv6 endpoint - detects IPv6",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"2001:db8::1"}},
			},
			expected: corev1.IPv6Protocol,
		},
		{
			name: "multiple IPv4 endpoints - detects IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"192.168.1.1"}},
				{Addresses: []string{"192.168.1.2"}},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "multiple IPv6 endpoints - detects IPv6",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"2001:db8::1"}},
				{Addresses: []string{"2001:db8::2"}},
			},
			expected: corev1.IPv6Protocol,
		},
		{
			name: "mixed endpoints (IPv4 first) - detects IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"192.168.1.1"}},
				{Addresses: []string{"2001:db8::1"}},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "mixed endpoints (IPv6 first) - detects IPv6",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"2001:db8::1"}},
				{Addresses: []string{"192.168.1.1"}},
			},
			expected: corev1.IPv6Protocol,
		},
		{
			name: "invalid IP followed by valid IPv4 - detects IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"not-an-ip"}},
				{Addresses: []string{"192.168.1.1"}},
			},
			expected: corev1.IPv4Protocol,
		},
		{
			name: "invalid IP followed by valid IPv6 - detects IPv6",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"not-an-ip"}},
				{Addresses: []string{"2001:db8::1"}},
			},
			expected: corev1.IPv6Protocol,
		},
		{
			name: "all invalid IPs - defaults to IPv4",
			endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"not-an-ip"}},
				{Addresses: []string{"also-not-an-ip"}},
			},
			expected: corev1.IPv4Protocol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectEndpointsIPFamily(tt.endpoints)
			if got != tt.expected {
				t.Errorf("DetectEndpointsIPFamily() = %v, want %v", got, tt.expected)
			}
		})
	}
}
