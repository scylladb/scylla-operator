package scylladbdatacenter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newDecommissionTestScyllaDBDatacenter(nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic",
			Namespace: testNamespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			DatacenterName: new("dc"),
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "a",
					RackTemplate: scyllav1alpha1.RackTemplate{
						Nodes: new(nodes),
					},
				},
			},
		},
	}
}

func newDecommissionTestStatefulSet(replicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-dc-a",
			Namespace: testNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(replicas),
		},
	}
}

func newDecommissionTestMemberService(name string, decommissionedLabelValue *string) *corev1.Service {
	labels := map[string]string{
		naming.RackNameLabel: "a",
	}
	if decommissionedLabelValue != nil {
		labels[naming.DecommissionedLabel] = *decommissionedLabelValue
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    labels,
		},
	}
}

func Test_getEffectiveRackNodeCount(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                 string
		nodes                int32
		decommissioningNodes []string
		expectedNodeCount    int32
		expectedErrorString  string
	}{
		{
			name:                 "node count from the spec when nothing is leaving",
			nodes:                3,
			decommissioningNodes: nil,
			expectedNodeCount:    3,
		},
		{
			name:                 "node count pinned to the lowest recorded ordinal",
			nodes:                3,
			decommissioningNodes: []string{"basic-dc-a-1", "basic-dc-a-2"},
			expectedNodeCount:    1,
		},
		{
			name:                 "node count pinned below a node count raised in the meantime",
			nodes:                5,
			decommissioningNodes: []string{"basic-dc-a-2"},
			expectedNodeCount:    2,
		},
		{
			name:                 "node count pinned above a node count lowered in the meantime",
			nodes:                0,
			decommissioningNodes: []string{"basic-dc-a-2"},
			expectedNodeCount:    2,
		},
		{
			name:                 "error for an unparsable node name",
			nodes:                3,
			decommissioningNodes: []string{"basic-dc-a"},
			expectedErrorString:  `can't get ordinal of decommissioning node "basic-dc-a": can't parse ordinal from member service name "basic-dc-a"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodeCount, err := getEffectiveRackNodeCount(newDecommissionTestScyllaDBDatacenter(tc.nodes), "a", tc.decommissioningNodes)

			gotErrorString := ""
			if err != nil {
				gotErrorString = err.Error()
			}
			if gotErrorString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, gotErrorString)
			}
			if err != nil {
				return
			}

			if *nodeCount != tc.expectedNodeCount {
				t.Errorf("expected node count %d, got %d", tc.expectedNodeCount, *nodeCount)
			}
		})
	}
}

func Test_makeRackDecommissioningNodes(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                 string
		nodes                int32
		replicas             int32
		decommissioningNodes []string
		services             []*corev1.Service
		expectedNodes        []string
		expectedErrorString  string
	}{
		{
			name:          "nothing is leaving a rack that isn't being scaled down",
			nodes:         2,
			replicas:      2,
			expectedNodes: nil,
		},
		{
			name:          "a scale down commits all the nodes it removes",
			nodes:         1,
			replicas:      3,
			expectedNodes: []string{"basic-dc-a-1", "basic-dc-a-2"},
		},
		{
			name:          "a scale down to zero commits all the nodes of the rack",
			nodes:         0,
			replicas:      2,
			expectedNodes: []string{"basic-dc-a-0", "basic-dc-a-1"},
		},
		{
			name:                 "a scale down requested while nodes are still leaving isn't committed",
			nodes:                0,
			replicas:             3,
			decommissioningNodes: []string{"basic-dc-a-2"},
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-2", new(naming.LabelValueFalse)),
			},
			expectedNodes: []string{"basic-dc-a-2"},
		},
		{
			name:                 "a node count raised back doesn't release a node that is still leaving",
			nodes:                3,
			replicas:             3,
			decommissioningNodes: []string{"basic-dc-a-2"},
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-2", new(naming.LabelValueFalse)),
			},
			expectedNodes: []string{"basic-dc-a-2"},
		},
		{
			name:                 "a decommissioned node is kept until its service is pruned",
			nodes:                1,
			replicas:             1,
			decommissioningNodes: []string{"basic-dc-a-1"},
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-1", new(naming.LabelValueTrue)),
			},
			expectedNodes: []string{"basic-dc-a-1"},
		},
		{
			name:                 "a removed node is dropped from the record",
			nodes:                1,
			replicas:             1,
			decommissioningNodes: []string{"basic-dc-a-1"},
			expectedNodes:        nil,
		},
		{
			name:                 "a node whose service was recreated afresh is dropped from the record",
			nodes:                2,
			replicas:             1,
			decommissioningNodes: []string{"basic-dc-a-1"},
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-1", nil),
			},
			expectedNodes: nil,
		},
		{
			name:                 "a node still accounted for by the StatefulSet is kept in the record",
			nodes:                2,
			replicas:             2,
			decommissioningNodes: []string{"basic-dc-a-1"},
			expectedNodes:        []string{"basic-dc-a-1"},
		},
		{
			name:     "a labeled node absent from the record is taken into it",
			nodes:    2,
			replicas: 2,
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-0", new(naming.LabelValueTrue)),
				newDecommissionTestMemberService("basic-dc-a-1", new(naming.LabelValueFalse)),
			},
			expectedNodes: []string{"basic-dc-a-0", "basic-dc-a-1"},
		},
		{
			name:                 "the record is ordered by ordinal",
			nodes:                0,
			replicas:             11,
			decommissioningNodes: []string{"basic-dc-a-10", "basic-dc-a-2"},
			services: []*corev1.Service{
				newDecommissionTestMemberService("basic-dc-a-10", new(naming.LabelValueFalse)),
				newDecommissionTestMemberService("basic-dc-a-2", new(naming.LabelValueFalse)),
				newDecommissionTestMemberService("basic-dc-a-9", new(naming.LabelValueFalse)),
			},
			expectedNodes: []string{"basic-dc-a-2", "basic-dc-a-9", "basic-dc-a-10"},
		},
		{
			name:                 "error for an unparsable recorded node name",
			nodes:                1,
			replicas:             1,
			decommissioningNodes: []string{"basic-dc-a"},
			expectedErrorString:  `can't get ordinal of decommissioning node "basic-dc-a": can't parse ordinal from member service name "basic-dc-a"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdc := newDecommissionTestScyllaDBDatacenter(tc.nodes)
			rackServices := map[string]*corev1.Service{}
			for _, svc := range tc.services {
				rackServices[svc.Name] = svc
			}

			got, err := makeRackDecommissioningNodes(sdc, sdc.Spec.Racks[0], tc.decommissioningNodes, newDecommissionTestStatefulSet(tc.replicas), rackServices)

			gotErrorString := ""
			if err != nil {
				gotErrorString = err.Error()
			}
			if gotErrorString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, gotErrorString)
			}
			if err != nil {
				return
			}

			if !cmp.Equal(got, tc.expectedNodes) {
				t.Errorf("expected and actual decommissioning nodes differ: %s", cmp.Diff(tc.expectedNodes, got))
			}
		})
	}
}
