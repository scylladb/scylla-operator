package controllerhelpers

import (
	"fmt"
	"reflect"
	"testing"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

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

	sc := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "us-east1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "us-east1-b",
						Members: 1,
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
			ClusterIP: "10.0.0.1",
			Type:      corev1.ServiceTypeClusterIP,
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
		scyllaCluster *scyllav1.ScyllaCluster
		svc           *corev1.Service
		pod           *corev1.Pod
		expected      string
		expectedError error
	}{
		{
			name: "service ClusterIP for nil expose options",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := sc.DeepCopy()
				sc.Spec.ExposeOptions = nil
				return sc
			}(),
			svc:      svc,
			pod:      pod,
			expected: "10.0.0.1",
		},
		{
			name: "service ClusterIP for nil broadcast options",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := sc.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{}
				return sc
			}(),
			svc:      svc,
			pod:      pod,
			expected: "10.0.0.1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual, err := GetScyllaHost(tc.scyllaCluster, tc.svc, tc.pod)

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
			ClusterIP: "10.0.0.1",
			Type:      corev1.ServiceTypeClusterIP,
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
		nodeBroadcastAddressType scyllav1.BroadcastAddressType
		svc                      *corev1.Service
		pod                      *corev1.Pod
		expected                 string
		expectedError            error
	}{

		{
			name:                     "service ClusterIP for ClusterIP broadcast address type",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			pod:                      pod,
			svc:                      svc,
			expected:                 "10.0.0.1",
		},
		{
			name:                     "error for ClusterIP broadcast address type and none ClusterIP",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.ClusterIP = corev1.ClusterIPNone

				return svc
			}(),
			expected:      "",
			expectedError: fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have a ClusterIP address`),
		},
		{
			name:                     "PodIP broadcast address type",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypePodIP,
			pod:                      pod,
			svc:                      svc,
			expected:                 "10.1.0.1",
			expectedError:            nil,
		},
		{
			name:                     "error for PodIP broadcast address type and empty PodIP",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypePodIP,
			pod: func() *corev1.Pod {
				pod := pod.DeepCopy()

				pod.Status.PodIP = ""

				return pod
			}(),
			svc:           svc,
			expected:      "",
			expectedError: fmt.Errorf(`pod "simple-cluster-us-east1-us-east1-b-0" does not have a PodIP address`),
		},
		{
			name:                     "error for broadcast address type service load balancer ingress and no service ingress status",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
			pod:                      pod,
			svc: func() *corev1.Service {
				svc := svc.DeepCopy()

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{}

				return svc
			}(),
			expected:      "",
			expectedError: fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have an ingress status`),
		},
		{
			name:                     "ip for broadcast address type service load balancer ingress and non-empty ip in service load balancer status",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
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
			expected:      "10.2.0.1",
			expectedError: nil,
		},
		{
			name:                     "hostname for broadcast address type service load balancer ingress, empty ip and non-empty hostname in service load balancer status",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
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
			expected:      "test.scylla.com",
			expectedError: nil,
		},
		{
			name:                     "error for broadcast address type service load balancer ingress and no external address in service load balancer status",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
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
			expected:      "",
			expectedError: fmt.Errorf(`service "simple-cluster-us-east1-us-east1-b-0" does not have an external address`),
		},
		{
			name:                     "error for unsupported broadcast address type",
			nodeBroadcastAddressType: scyllav1.BroadcastAddressType("Unsupported"),
			pod:                      pod,
			svc:                      svc,
			expected:                 "",
			expectedError:            fmt.Errorf(`unsupported broadcast address type: "Unsupported"`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual, err := GetScyllaBroadcastAddress(tc.nodeBroadcastAddressType, tc.svc, tc.pod)

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

	sc := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster",
			Namespace: "test",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "us-east1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "us-east1-b",
						Members: 2,
					},
				},
			},
		},
		Status: scyllav1.ScyllaClusterStatus{},
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
			ClusterIP: "10.0.0.1",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}

	secondService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-us-east1-us-east1-b-1",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.2",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}

	tt := []struct {
		name          string
		sc            *scyllav1.ScyllaCluster
		services      map[string]*corev1.Service
		existingPods  []*corev1.Pod
		expected      []string
		expectedError error
	}{
		{
			name: "missing service",
			sc:   sc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: nil,
			expectedError: utilerrors.NewAggregate([]error{
				fmt.Errorf(`service "test/simple-cluster-us-east1-us-east1-b-1" does not exist`),
			}),
		},
		{
			name: "missing pod",
			sc:   sc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": secondService,
			},
			existingPods: []*corev1.Pod{
				firstPod,
			},
			expected: nil,
			expectedError: utilerrors.NewAggregate([]error{
				fmt.Errorf(`can't get pod "test/simple-cluster-us-east1-us-east1-b-1": %w`, apierrors.NewNotFound(corev1.Resource("pod"), "simple-cluster-us-east1-us-east1-b-1")),
			}),
		},
		{
			name: "ClusterIP aggregate",
			sc:   sc,
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
			sc: func() *scyllav1.ScyllaCluster {
				sc := sc.DeepCopy()

				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypePodIP,
						},
					},
				}

				return sc
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
			sc:   sc,
			services: map[string]*corev1.Service{
				"simple-cluster-us-east1-us-east1-b-0": firstService,
				"simple-cluster-us-east1-us-east1-b-1": func() *corev1.Service {
					svc := secondService.DeepCopy()

					svc.Spec.ClusterIP = corev1.ClusterIPNone

					return svc
				}(),
			},
			existingPods: []*corev1.Pod{
				firstPod,
				secondPod,
			},
			expected: nil,
			expectedError: utilerrors.NewAggregate([]error{
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

			actual, err := GetRequiredScyllaHosts(tc.sc, tc.services, podLister)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#+v, got %#+v", tc.expectedError, err)
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
