package controllerhelpers

import (
	"fmt"
	"reflect"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestNewScyllaClientConfigForLocalhost(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name         string
		ipFamily     corev1.IPFamily
		expectedHost string
	}{
		{
			name:         "IPv4 creates config for 127.0.0.1",
			ipFamily:     corev1.IPv4Protocol,
			expectedHost: "127.0.0.1",
		},
		{
			name:         "IPv6 creates config for ::1",
			ipFamily:     corev1.IPv6Protocol,
			expectedHost: "::1",
		},
		{
			name:         "empty string defaults to IPv4",
			ipFamily:     "",
			expectedHost: "127.0.0.1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := NewScyllaClientConfigForLocalhost(tc.ipFamily)

			if len(cfg.Hosts) != 1 {
				t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
			}
			if cfg.Hosts[0] != tc.expectedHost {
				t.Errorf("expected host %q, got %q", tc.expectedHost, cfg.Hosts[0])
			}
			if cfg.Scheme != "http" {
				t.Errorf("expected scheme %q, got %q", "http", cfg.Scheme)
			}
		})
	}
}

func TestNewScyllaClientForLocalhost(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		ipFamily corev1.IPFamily
	}{
		{
			name:     "IPv4 creates client",
			ipFamily: corev1.IPv4Protocol,
		},
		{
			name:     "IPv6 creates client",
			ipFamily: corev1.IPv6Protocol,
		},
		{
			name:     "empty string defaults to IPv4",
			ipFamily: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewScyllaClientForLocalhost(tc.ipFamily)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer client.Close()

			if client == nil {
				t.Fatal("expected non-nil client")
			}
		})
	}
}

func TestIsNodeConfigSelectingNode(t *testing.T) {
	tt := []struct {
		name        string
		placement   *scyllav1alpha1.NodeConfigPlacement
		nodeLabels  map[string]string
		nodeTaints  []corev1.Taint
		expected    bool
		expectedErr error
	}{
		{
			name:      "empty placement selects non-tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: map[string]string{
				"foo": "bar",
			},
			nodeTaints:  nil,
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "node selector won't match a node without the label",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				NodeSelector: map[string]string{
					"alpha": "beta",
				},
			},
			nodeLabels: map[string]string{
				"foo": "bar",
			},
			nodeTaints:  nil,
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "node selector will match a node with the label",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				NodeSelector: map[string]string{
					"alpha": "beta",
				},
			},
			nodeLabels: map[string]string{
				"alpha": "beta",
			},
			nodeTaints:  nil,
			expected:    true,
			expectedErr: nil,
		},
		{
			name:       "placement without any toleration won't select tained node",
			placement:  &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name:       "placement without any toleration will select tained node with effects other then NoSchedule and NoExecute",
			placement:  &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "placement without matching toleration won't select tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				Tolerations: []corev1.Toleration{
					{
						Key:      "alpha",
						Value:    "beta",
						Effect:   corev1.TaintEffectNoSchedule,
						Operator: corev1.TolerationOpEqual,
					},
				},
			},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "placement with matching toleration will select tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				Tolerations: []corev1.Toleration{
					{
						Key:      "foo",
						Value:    "bar",
						Operator: corev1.TolerationOpEqual,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    true,
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			nc := &scyllav1alpha1.NodeConfig{
				Spec: scyllav1alpha1.NodeConfigSpec{
					Placement: *tc.placement,
				},
			}

			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.nodeLabels,
				},
				Spec: corev1.NodeSpec{
					Taints: tc.nodeTaints,
				},
			}

			got, err := IsNodeConfigSelectingNode(nc, node)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestGetScyllaHost(t *testing.T) {
	t.Parallel()

	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster",
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			DatacenterName: pointer.Ptr("us-east1"),
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "us-east1-b",
					RackTemplate: scyllav1alpha1.RackTemplate{
						Nodes: pointer.Ptr[int32](1),
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster-us-east1-us-east1-b-0",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:  "10.0.0.1",
			ClusterIPs: []string{"10.0.0.1"},
			Type:       corev1.ServiceTypeClusterIP,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster-us-east1-us-east1-b-0",
		},
		Status: corev1.PodStatus{
			PodIP: "10.1.0.1",
		},
	}

	tt := []struct {
		name          string
		sdc           *scyllav1alpha1.ScyllaDBDatacenter
		svc           *corev1.Service
		pod           *corev1.Pod
		expected      string
		expectedError error
	}{
		{
			name: "service ClusterIP for nil expose options",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := sdc.DeepCopy()
				sdc.Spec.ExposeOptions = nil
				return sdc
			}(),
			svc:      svc,
			pod:      pod,
			expected: "10.0.0.1",
		},
		{
			name: "service ClusterIP for nil broadcast options",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := sdc.DeepCopy()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{}
				return sdc
			}(),
			svc:      svc,
			pod:      pod,
			expected: "10.0.0.1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual, err := GetScyllaHost(tc.sdc, tc.svc, tc.pod)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#+v, got %#+v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected host %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestGetScyllaNodeBroadcastAddress(t *testing.T) {
	t.Parallel()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster-us-east1-us-east1-b-0",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:  "10.0.0.1",
			ClusterIPs: []string{"10.0.0.1"},
			Type:       corev1.ServiceTypeClusterIP,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster-us-east1-us-east1-b-0",
		},
		Status: corev1.PodStatus{
			PodIP: "10.1.0.1",
		},
	}

	tt := []struct {
		name                     string
		nodeBroadcastAddressType scyllav1alpha1.BroadcastAddressType
		svc                      *corev1.Service
		pod                      *corev1.Pod
		preferredFamily          *corev1.IPFamily
		expected                 string
		expectedError            error
	}{

		{
			name:                     "service ClusterIP for ClusterIP broadcast address type",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			pod:                      pod,
			svc:                      svc,
			preferredFamily:          nil, // Auto-detect
			expected:                 "10.0.0.1",
		},
		{
			name:                     "error for ClusterIP broadcast address type and none ClusterIP",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.ClusterIP = corev1.ClusterIPNone
				svc.Spec.ClusterIPs = []string{corev1.ClusterIPNone}

				return svc
			}(),
			preferredFamily: nil, // Auto-detect
			expected:        "",
			expectedError:   fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have a ClusterIP address`),
		},
		{
			name:                     "PodIP broadcast address type",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypePodIP,
			pod:                      pod,
			svc:                      svc,
			preferredFamily:          nil, // Auto-detect
			expected:                 "10.1.0.1",
			expectedError:            nil,
		},
		{
			name:                     "error for PodIP broadcast address type and empty PodIP",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypePodIP,
			pod: func() *corev1.Pod {
				pod := pod.DeepCopy()

				pod.Status.PodIP = ""

				return pod
			}(),
			svc:             svc,
			preferredFamily: nil, // Auto-detect
			expected:        "",
			expectedError:   fmt.Errorf(`pod "simple-cluster-us-east1-us-east1-b-0" does not have a PodIP address`),
		},
		{
			name:                     "error for broadcast address type service load balancer ingress and no service ingress status",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{}

				return svc
			}(),
			preferredFamily: nil, // Auto-detect
			expected:        "",
			expectedError:   fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have an ingress status`),
		},
		{
			name:                     "ip for broadcast address type service load balancer ingress and non-empty ip in service load balancer status",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP:       "10.2.0.1",
						Hostname: "test.scylla.com",
					},
				}

				return svc
			}(),
			preferredFamily: nil, // Auto-detect
			expected:        "10.2.0.1",
			expectedError:   nil,
		},
		{
			name:                     "hostname for broadcast address type service load balancer ingress, empty ip and non-empty hostname in service load balancer status",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP:       "",
						Hostname: "test.scylla.com",
					},
				}

				return svc
			}(),
			preferredFamily: nil, // Auto-detect
			expected:        "test.scylla.com",
			expectedError:   nil,
		},
		{
			name:                     "error for broadcast address type service load balancer ingress and no external address in service load balancer status",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP:       "",
						Hostname: "",
					},
				}

				return svc
			}(),
			preferredFamily: nil, // Auto-detect
			expected:        "",
			expectedError:   fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have an external address`),
		},
		{
			name:                     "error for unsupported broadcast address type",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressType("Unsupported"),
			pod:                      pod,
			svc:                      svc,
			preferredFamily:          nil, // Auto-detect
			expected:                 "",
			expectedError:            fmt.Errorf(`unsupported broadcast address type: "Unsupported"`),
		},
		{
			name:                     "LoadBalancerIngress with IPv6 address",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP: "2001:db8::1",
					},
				}
				return svc
			}(),
			preferredFamily: nil,
			expected:        "2001:db8::1",
			expectedError:   nil,
		},
		{
			name:                     "LoadBalancerIngress with dual-stack and IPv4 preference",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.2.0.1"},
					{IP: "2001:db8::2"},
				}
				return svc
			}(),
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "10.2.0.1",
			expectedError:   nil,
		},
		{
			name:                     "LoadBalancerIngress with dual-stack and IPv6 preference",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.2.0.1"},
					{IP: "2001:db8::2"},
				}
				return svc
			}(),
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "2001:db8::2",
			expectedError:   nil,
		},
		{
			name:                     "PodIP with explicit IPv4 preference",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypePodIP,
			pod: func() *corev1.Pod {
				pod := pod.DeepCopy()
				pod.Status.PodIPs = []corev1.PodIP{
					{IP: "10.1.0.1"},
					{IP: "2001:db8::1"},
				}
				return pod
			}(),
			svc:             svc,
			preferredFamily: pointer.Ptr(corev1.IPv4Protocol),
			expected:        "10.1.0.1",
			expectedError:   nil,
		},
		{
			name:                     "PodIP with explicit IPv6 preference",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypePodIP,
			pod: func() *corev1.Pod {
				pod := pod.DeepCopy()
				pod.Status.PodIPs = []corev1.PodIP{
					{IP: "10.1.0.1"},
					{IP: "2001:db8::1"},
				}
				return pod
			}(),
			svc:             svc,
			preferredFamily: pointer.Ptr(corev1.IPv6Protocol),
			expected:        "2001:db8::1",
			expectedError:   nil,
		},
		{
			name:                     "ServiceClusterIP with dual-stack auto-detection (IPv4 first)",
			nodeBroadcastAddressType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()
				svc.Spec.ClusterIPs = []string{"10.0.0.1", "2001:db8::svc"}
				svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
				return svc
			}(),
			preferredFamily: nil, // Auto-detect - should pick first from service
			expected:        "10.0.0.1",
			expectedError:   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual, err := GetScyllaBroadcastAddress(tc.nodeBroadcastAddressType, tc.svc, tc.pod, tc.preferredFamily)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#+v, got %#+v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected host %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestGetRequiredScyllaHosts(t *testing.T) {
	t.Parallel()

	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster",
			Namespace: "test",
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName:    "simple-cluster",
			DatacenterName: pointer.Ptr("us-east1"),
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "us-east1-b",
					RackTemplate: scyllav1alpha1.RackTemplate{
						Nodes: pointer.Ptr[int32](2),
					},
				},
			},
		},
	}

	firstPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-us-east1-us-east1-b-0",
			Namespace: "test",
		},
		Status: corev1.PodStatus{
			PodIP: "10.1.0.1",
		},
	}

	secondPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-us-east1-us-east1-b-1",
			Namespace: "test",
		},
		Status: corev1.PodStatus{
			PodIP: "10.1.0.2",
		},
	}

	firstService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-us-east1-us-east1-b-0",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:  "10.0.0.1",
			ClusterIPs: []string{"10.0.0.1"},
			Type:       corev1.ServiceTypeClusterIP,
		},
	}

	secondService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-us-east1-us-east1-b-1",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:  "10.0.0.2",
			ClusterIPs: []string{"10.0.0.2"},
			Type:       corev1.ServiceTypeClusterIP,
		},
	}

	tt := []struct {
		name          string
		sdc           *scyllav1alpha1.ScyllaDBDatacenter
		services      map[string]*corev1.Service
		existingPods  []*corev1.Pod
		expected      []string
		expectedError error
	}{
		{
			name: "missing service",
			sdc:  sdc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: nil,
			expectedError: apimachineryutilerrors.NewAggregate([]error{
				fmt.Errorf(`service "test/simple-cluster-us-east1-us-east1-b-1" does not exist`),
			}),
		},
		{
			name: "missing pod",
			sdc:  sdc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": secondService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
			},
			expected: nil,
			expectedError: apimachineryutilerrors.NewAggregate([]error{
				fmt.Errorf(`can't get pod "test/simple-cluster-us-east1-us-east1-b-1": %w`, apierrors.NewNotFound(corev1.Resource("pod"), "simple-cluster-us-east1-us-east1-b-1")),
			}),
		},
		{
			name: "ClusterIP aggregate",
			sdc:  sdc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": secondService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: []string{
				"10.0.0.1",
				"10.0.0.2",
			},
			expectedError: nil,
		},
		{
			name: "PodIP aggregate",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := sdc.DeepCopy()

				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": secondService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: []string{
				"10.1.0.1",
				"10.1.0.2",
			},
			expectedError: nil,
		},
		{
			name: "service missing ClusterIP",
			sdc:  sdc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": func() *corev1.Service {
					svc := secondService.DeepCopy()

					svc.Spec.ClusterIP = corev1.ClusterIPNone
					svc.Spec.ClusterIPs = []string{corev1.ClusterIPNone}

					return svc
				}(),
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: nil,
			expectedError: apimachineryutilerrors.NewAggregate([]error{
				fmt.Errorf(`can't get scylla host for service "test/simple-cluster-us-east1-us-east1-b-1": %w`, fmt.Errorf(`service "test/simple-cluster-us-east1-us-east1-b-1" does not have a ClusterIP address`)),
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			podCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, obj := range tc.existingPods {
				err := podCache.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}
			podLister := corev1listers.NewPodLister(podCache)

			actual, err := GetRequiredScyllaHosts(tc.sdc, tc.services, podLister)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected host %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestIsNodeTunedForContainer(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		nodeConfig    *scyllav1alpha1.NodeConfig
		nodeName      string
		containerID   string
		expectedTuned bool
	}{
		{
			name:          "empty nodeConfig status isn't considered to be tuned",
			nodeConfig:    &scyllav1alpha1.NodeConfig{},
			nodeName:      "node1",
			containerID:   "container-id",
			expectedTuned: false,
		},
		{
			name: "nodeConfig of different node",
			nodeConfig: &scyllav1alpha1.NodeConfig{
				Status: scyllav1alpha1.NodeConfigStatus{
					NodeStatuses: []scyllav1alpha1.NodeConfigNodeStatus{
						{
							Name:            "different-node",
							TunedNode:       true,
							TunedContainers: []string{"container-id"},
						},
					},
				},
			},
			nodeName:      "node1",
			containerID:   "container-id",
			expectedTuned: false,
		},
		{
			name: "not tuned node having no tuned containers isn't considered to be tuned",
			nodeConfig: &scyllav1alpha1.NodeConfig{
				Status: scyllav1alpha1.NodeConfigStatus{
					NodeStatuses: []scyllav1alpha1.NodeConfigNodeStatus{
						{
							Name:            "node1",
							TunedNode:       false,
							TunedContainers: []string{},
						},
					},
				},
			},
			nodeName:      "node1",
			containerID:   "container-id",
			expectedTuned: false,
		},
		{
			name: "tuned node but with different tuned container isn't considered to be tuned",
			nodeConfig: &scyllav1alpha1.NodeConfig{
				Status: scyllav1alpha1.NodeConfigStatus{
					NodeStatuses: []scyllav1alpha1.NodeConfigNodeStatus{
						{
							Name:            "node1",
							TunedNode:       true,
							TunedContainers: []string{"different-container-id"},
						},
					},
				},
			},
			nodeName:      "node1",
			containerID:   "container-id",
			expectedTuned: false,
		},
		{
			name: "tuned node having matching tuned container is considered to be tuned",
			nodeConfig: &scyllav1alpha1.NodeConfig{
				Status: scyllav1alpha1.NodeConfigStatus{
					NodeStatuses: []scyllav1alpha1.NodeConfigNodeStatus{
						{
							Name:            "node1",
							TunedNode:       true,
							TunedContainers: []string{"different-container-id", "container-id"},
						},
					},
				},
			},
			nodeName:      "node1",
			containerID:   "container-id",
			expectedTuned: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tuned := IsNodeTunedForContainer(tc.nodeConfig, tc.nodeName, tc.containerID)
			if tuned != tc.expectedTuned {
				t.Errorf("expected %v, got %v", tc.expectedTuned, tuned)
			}
		})
	}
}

// Test names follow the following naming format:
// DCT() - refers to DatacenterTemplate
// RT() - refers to RackTemplate
// R - refers to RackSpec
// Objects may be embedded within each other via parentheses.
// Number close to or embedded in means the number of nodes is specified there.
func TestGetNodeCount(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		cluster           *scyllav1alpha1.ScyllaDBCluster
		expectedNodeCount int32
	}{
		{
			name: "DCT(RT(3),R,R,R)-DC()",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						RackTemplate: &scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](3),
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								Name: "a",
							},
							{
								Name: "b",
							},
							{
								Name: "c",
							},
						},
					},
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name: "dc1",
						},
					},
				},
			},
			expectedNodeCount: 9,
		},
		{
			name: "DCT(RT(3),R,R,R)-DC(),DC(),DC()",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						RackTemplate: &scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](3),
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								Name: "a",
							},
							{
								Name: "b",
							},
							{
								Name: "c",
							},
						},
					},
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name: "dc1",
						},
						{
							Name: "dc2",
						},
						{
							Name: "dc3",
						},
					},
				},
			},
			expectedNodeCount: 27,
		},
		{
			name: "DCT(RT(3),R,R,R)-DC(),DC(RT(1)),DC()",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						RackTemplate: &scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](3),
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								Name: "a",
							},
							{
								Name: "b",
							},
							{
								Name: "c",
							},
						},
					},
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name: "dc1",
						},
						{
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								RackTemplate: &scyllav1alpha1.RackTemplate{
									Nodes: pointer.Ptr[int32](1),
								},
							},
							Name: "dc2",
						},
						{
							Name: "dc3",
						},
					},
				},
			},
			expectedNodeCount: 21,
		},
		{
			name: "DCT(RT(3),R,R,R)-DC(),DC(RT(1),R,R4),DC()",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						RackTemplate: &scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](3),
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								Name: "a",
							},
							{
								Name: "b",
							},
							{
								Name: "c",
							},
						},
					},
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name: "dc1",
						},
						{
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								RackTemplate: &scyllav1alpha1.RackTemplate{
									Nodes: pointer.Ptr[int32](1),
								},
								Racks: []scyllav1alpha1.RackSpec{
									{
										Name: "a",
									},
									{
										Name: "b",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](4),
										},
									},
								},
							},
							Name: "dc2",
						},
						{
							Name: "dc3",
						},
					},
				},
			},
			expectedNodeCount: 23,
		},
		{
			name: "DC(R1),DC(R1,R2),DC(R3,R2,R1)",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name: "dc1",
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								Racks: []scyllav1alpha1.RackSpec{
									{
										Name: "a",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](1),
										},
									},
								},
							},
						},
						{
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								Racks: []scyllav1alpha1.RackSpec{
									{
										Name: "a",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](1),
										},
									},
									{
										Name: "b",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](2),
										},
									},
								},
							},
							Name: "dc2",
						},
						{
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								Racks: []scyllav1alpha1.RackSpec{
									{
										Name: "a",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](3),
										},
									},
									{
										Name: "b",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](2),
										},
									},
									{
										Name: "c",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](1),
										},
									},
								},
							},
							Name: "dc3",
						},
					},
				},
			},
			expectedNodeCount: 10,
		},
		{
			name: "DC(RT(3),R,R2,R)",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
								RackTemplate: &scyllav1alpha1.RackTemplate{
									Nodes: pointer.Ptr[int32](3),
								},
								Racks: []scyllav1alpha1.RackSpec{
									{
										Name: "a",
									},
									{
										Name: "b",
										RackTemplate: scyllav1alpha1.RackTemplate{
											Nodes: pointer.Ptr[int32](2),
										},
									},
									{
										Name: "c",
									},
								},
							},
							Name: "dc1",
						},
					},
				},
			},
			expectedNodeCount: 8,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotNodeCount := GetScyllaDBClusterNodeCount(tc.cluster)
			if gotNodeCount != tc.expectedNodeCount {
				t.Errorf("expected node count to be %d, got %d", tc.expectedNodeCount, gotNodeCount)
			}
		})
	}
}

func TestIsScyllaPod(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with required pod-type label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.PodTypeLabel: string(naming.PodTypeScyllaDBNode),
					},
				},
			},
			expected: true,
		},
		{
			name: "pod without required pod-type label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some-other-label": "some-value",
					},
				},
			},
			expected: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IsScyllaPod(tc.pod)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
