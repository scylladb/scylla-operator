package scylladbdatacenter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
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
