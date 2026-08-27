package scylladbdatacenter

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_calculateDecommissioningNodes(t *testing.T) {
	t.Parallel()

	newRackService := func(name, rackName string, labels map[string]string) *corev1.Service {
		if labels == nil {
			labels = map[string]string{}
		}
		labels[naming.RackNameLabel] = rackName
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      name,
				Labels:    labels,
			},
		}
	}

	tt := []struct {
		name     string
		services map[string]*corev1.Service
		expected []scyllav1alpha1.DecommissioningNodeStatus
	}{
		{
			name:     "no Services yields nil",
			services: nil,
			expected: nil,
		},
		{
			name: "no labelled Services yields nil",
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newRackService("basic-dc-a-0", "a", nil),
				"basic-dc-a-1": newRackService("basic-dc-a-1", "a", nil),
			},
			expected: nil,
		},
		{
			name: "labels with either value are listed",
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newRackService("basic-dc-a-0", "a", nil),
				"basic-dc-a-1": newRackService("basic-dc-a-1", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueTrue}),
				"basic-dc-a-2": newRackService("basic-dc-a-2", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
			},
			expected: []scyllav1alpha1.DecommissioningNodeStatus{
				{Name: "basic-dc-a-1"},
				{Name: "basic-dc-a-2"},
			},
		},
		{
			name: "only Services of this rack are listed",
			services: map[string]*corev1.Service{
				"basic-dc-a-1": newRackService("basic-dc-a-1", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
				"basic-dc-b-1": newRackService("basic-dc-b-1", "b", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
			},
			expected: []scyllav1alpha1.DecommissioningNodeStatus{
				{Name: "basic-dc-a-1"},
			},
		},
		{
			name: "entries are sorted by name",
			services: map[string]*corev1.Service{
				"basic-dc-a-2":  newRackService("basic-dc-a-2", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
				"basic-dc-a-10": newRackService("basic-dc-a-10", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
				"basic-dc-a-1":  newRackService("basic-dc-a-1", "a", map[string]string{naming.DecommissionedLabel: naming.LabelValueFalse}),
			},
			expected: []scyllav1alpha1.DecommissioningNodeStatus{
				{Name: "basic-dc-a-1"},
				{Name: "basic-dc-a-10"},
				{Name: "basic-dc-a-2"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := calculateDecommissioningNodes("a", tc.services)
			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("expected and got decommissioning nodes differ (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_getServiceOrdinal(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		serviceName     string
		expectedOrdinal int32
		expectedErr     error
	}{
		{
			name:            "parses the trailing ordinal",
			serviceName:     "basic-dc-a-12",
			expectedOrdinal: 12,
		},
		{
			name:        "rejects a name without an ordinal",
			serviceName: "basic-dc-a",
			expectedErr: fmt.Errorf(`can't parse ordinal from service name "basic-dc-a"`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := getServiceOrdinal(tc.serviceName)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}
			if got != tc.expectedOrdinal {
				t.Errorf("expected ordinal %d, got %d", tc.expectedOrdinal, got)
			}
		})
	}
}

func Test_getEffectiveRackNodeCount(t *testing.T) {
	t.Parallel()

	newScyllaDBDatacenterWithNodes := func(nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc := newScyllaDBDatacenter()
		sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
			Nodes: new(nodes),
		}
		sdc.Spec.Racks = []scyllav1alpha1.RackSpec{
			{Name: "a"},
		}
		return sdc
	}
	newRackStatefulSet := func(sdc *scyllav1alpha1.ScyllaDBDatacenter, replicas int32) map[string]*appsv1.StatefulSet {
		return map[string]*appsv1.StatefulSet{
			naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc): {
				Spec: appsv1.StatefulSetSpec{
					Replicas: new(replicas),
				},
			},
		}
	}
	newStatus := func(nodes ...string) *scyllav1alpha1.ScyllaDBDatacenterStatus {
		rackStatus := scyllav1alpha1.RackStatus{
			Name: "a",
		}
		for _, node := range nodes {
			rackStatus.DecommissioningNodes = append(rackStatus.DecommissioningNodes, scyllav1alpha1.DecommissioningNodeStatus{Name: node})
		}
		return &scyllav1alpha1.ScyllaDBDatacenterStatus{
			Racks: []scyllav1alpha1.RackStatus{rackStatus},
		}
	}

	sdc := newScyllaDBDatacenterWithNodes(3)
	stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

	tt := []struct {
		name              string
		sdc               *scyllav1alpha1.ScyllaDBDatacenter
		status            *scyllav1alpha1.ScyllaDBDatacenterStatus
		statefulSets      map[string]*appsv1.StatefulSet
		expectedNodeCount *int32
		expectedErr       error
	}{
		{
			name:              "spec node count without a rack status",
			sdc:               sdc,
			status:            &scyllav1alpha1.ScyllaDBDatacenterStatus{},
			statefulSets:      newRackStatefulSet(sdc, 3),
			expectedNodeCount: new(int32(3)),
		},
		{
			name:              "spec node count with an empty record",
			sdc:               sdc,
			status:            newStatus(),
			statefulSets:      newRackStatefulSet(sdc, 3),
			expectedNodeCount: new(int32(3)),
		},
		{
			name:              "lowest recorded ordinal while nodes are leaving, ignoring a raised spec",
			sdc:               newScyllaDBDatacenterWithNodes(5),
			status:            newStatus(stsName+"-1", stsName+"-2"),
			statefulSets:      newRackStatefulSet(sdc, 3),
			expectedNodeCount: new(int32(1)),
		},
		{
			name:              "lowest recorded ordinal while nodes are leaving, ignoring a lowered spec",
			sdc:               newScyllaDBDatacenterWithNodes(0),
			status:            newStatus(stsName + "-2"),
			statefulSets:      newRackStatefulSet(sdc, 3),
			expectedNodeCount: new(int32(2)),
		},
		{
			name:              "capped at the StatefulSet replicas when the recorded nodes are already scaled down",
			sdc:               newScyllaDBDatacenterWithNodes(5),
			status:            newStatus(stsName + "-2"),
			statefulSets:      newRackStatefulSet(sdc, 2),
			expectedNodeCount: new(int32(2)),
		},
		{
			name:              "lowest recorded ordinal without a StatefulSet",
			sdc:               sdc,
			status:            newStatus(stsName + "-1"),
			statefulSets:      nil,
			expectedNodeCount: new(int32(1)),
		},
		{
			name:         "unparsable recorded node name",
			sdc:          sdc,
			status:       newStatus("foo"),
			statefulSets: newRackStatefulSet(sdc, 3),
			expectedErr:  fmt.Errorf(`can't get ordinal of decommissioning node "foo" of rack "a" of ScyllaDBDatacenter "%s": %w`, naming.ObjRef(sdc), fmt.Errorf(`can't parse ordinal from service name "foo"`)),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := getEffectiveRackNodeCount(tc.sdc, tc.status, tc.statefulSets, tc.sdc.Spec.Racks[0])
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(got, tc.expectedNodeCount) {
				t.Errorf("expected node count %v, got %v", tc.expectedNodeCount, got)
			}
		})
	}
}
