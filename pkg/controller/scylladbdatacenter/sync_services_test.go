package scylladbdatacenter

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getEffectiveRackNodeCount(t *testing.T) {
	t.Parallel()

	newSDC := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic",
				Namespace: testNamespace,
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "a",
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: new(int32(3)),
						},
					},
				},
			},
		}
	}

	newMemberService := func(name string, labels map[string]string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
				Labels:    labels,
			},
		}
	}

	tt := []struct {
		name              string
		services          map[string]*corev1.Service
		expectedNodeCount *int32
	}{
		{
			name:              "spec node count with no services",
			services:          nil,
			expectedNodeCount: new(int32(3)),
		},
		{
			name: "spec node count with no decommissioned members",
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", map[string]string{
					naming.RackNameLabel: "a",
				}),
			},
			expectedNodeCount: new(int32(3)),
		},
		{
			name: "clamped to the lowest ordinal of a member being decommissioned",
			services: map[string]*corev1.Service{
				"basic-dc-a-2": newMemberService("basic-dc-a-2", map[string]string{
					naming.RackNameLabel:       "a",
					naming.DecommissionedLabel: naming.LabelValueFalse,
				}),
			},
			expectedNodeCount: new(int32(2)),
		},
		{
			name: "clamped to the lowest ordinal of a decommissioned member",
			services: map[string]*corev1.Service{
				"basic-dc-a-1": newMemberService("basic-dc-a-1", map[string]string{
					naming.RackNameLabel:       "a",
					naming.DecommissionedLabel: naming.LabelValueTrue,
				}),
				"basic-dc-a-2": newMemberService("basic-dc-a-2", map[string]string{
					naming.RackNameLabel:       "a",
					naming.DecommissionedLabel: naming.LabelValueTrue,
				}),
			},
			expectedNodeCount: new(int32(1)),
		},
		{
			name: "not raised by a decommissioned member above the spec node count",
			services: map[string]*corev1.Service{
				"basic-dc-a-4": newMemberService("basic-dc-a-4", map[string]string{
					naming.RackNameLabel:       "a",
					naming.DecommissionedLabel: naming.LabelValueTrue,
				}),
			},
			expectedNodeCount: new(int32(3)),
		},
		{
			name: "decommissioned members of other racks are ignored",
			services: map[string]*corev1.Service{
				"basic-dc-b-0": newMemberService("basic-dc-b-0", map[string]string{
					naming.RackNameLabel:       "b",
					naming.DecommissionedLabel: naming.LabelValueFalse,
				}),
			},
			expectedNodeCount: new(int32(3)),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodeCount, err := getEffectiveRackNodeCount(newSDC(), "a", tc.services)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if *nodeCount != *tc.expectedNodeCount {
				t.Errorf("expected node count %d, got %d", *tc.expectedNodeCount, *nodeCount)
			}
		})
	}
}
