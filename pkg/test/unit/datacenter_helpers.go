package unit

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// NewSingleRackDatacenter returns ScyllaDatacenter having single racks.
func NewSingleRackDatacenter(nodes int32) *scyllav1alpha1.ScyllaDatacenter {
	return NewDetailedSingleRackDatacenter("test-datacenter", "test-ns", "repo:2.3.1", "test-dc", "test-rack", nodes)
}

// NewMultiRackDatacenter returns ScyllaDatacenter having multiple racks.
func NewMultiRackDatacenter(members ...int32) *scyllav1alpha1.ScyllaDatacenter {
	return NewDetailedMultiRackDatacenter("test-datacenter", "test-ns", "repo:2.3.1", "test-dc", members...)
}

// NewDetailedSingleRackDatacenter returns ScyllaDatacenter having single rack with supplied information.
func NewDetailedSingleRackDatacenter(name, namespace, image, dc, rack string, nodes int32) *scyllav1alpha1.ScyllaDatacenter {
	return &scyllav1alpha1.ScyllaDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1alpha1.ScyllaDatacenterSpec{
			Scylla: scyllav1alpha1.Scylla{
				Image: image,
			},
			DatacenterName: dc,
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name:  rack,
					Nodes: pointer.Int32(nodes),
					Scylla: &scyllav1alpha1.ScyllaOverrides{
						Resources: &corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						Storage: &scyllav1alpha1.Storage{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceStorage: resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDatacenterStatus{
			ObservedGeneration: pointer.Int64(1),
			Racks: map[string]scyllav1alpha1.RackStatus{
				rack: {
					Image:        image,
					Nodes:        pointer.Int32(nodes),
					ReadyNodes:   pointer.Int32(nodes),
					UpdatedNodes: pointer.Int32(nodes),
					Stale:        pointer.Bool(false),
				},
			},
		},
	}
}

// NewDetailedMultiRackDatacenter creates multi rack database with supplied information.
func NewDetailedMultiRackDatacenter(name, namespace, image, dc string, nodes ...int32) *scyllav1alpha1.ScyllaDatacenter {
	c := &scyllav1alpha1.ScyllaDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1alpha1.ScyllaDatacenterSpec{
			Scylla: scyllav1alpha1.Scylla{
				Image: image,
			},
			DatacenterName: dc,
			Racks:          []scyllav1alpha1.RackSpec{},
		},
		Status: scyllav1alpha1.ScyllaDatacenterStatus{
			ObservedGeneration: pointer.Int64(1),
			Racks:              map[string]scyllav1alpha1.RackStatus{},
		},
	}

	for i, n := range nodes {
		rack := fmt.Sprintf("rack-%d", i)
		c.Spec.Racks = append(c.Spec.Racks, scyllav1alpha1.RackSpec{
			Name:  rack,
			Nodes: pointer.Int32(n),
			Scylla: &scyllav1alpha1.ScyllaOverrides{
				Storage: &scyllav1alpha1.Storage{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceStorage: resource.MustParse("5Gi"),
						},
					},
				},
			},
		})
		c.Status.Racks[rack] = scyllav1alpha1.RackStatus{
			Image:      image,
			Nodes:      pointer.Int32(n),
			ReadyNodes: pointer.Int32(n),
			Stale:      pointer.Bool(false),
		}
	}

	return c
}
