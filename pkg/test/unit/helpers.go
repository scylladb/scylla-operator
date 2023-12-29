package unit

import (
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSingleRackCluster returns ScyllaCluster having single racks.
func NewSingleRackCluster(members int32) *scyllav1.ScyllaCluster {
	return NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "test-dc", "test-rack", members)
}

// NewMultiRackCluster returns ScyllaCluster having multiple racks.
func NewMultiRackCluster(members ...int32) *scyllav1.ScyllaCluster {
	return NewDetailedMultiRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "test-dc", members...)
}

// NewDetailedSingleRackCluster returns ScyllaCluster having single rack with supplied information.
func NewDetailedSingleRackCluster(name, namespace, repo, version, dc, rack string, members int32) *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Repository: repo,
			Version:    version,
			Datacenter: scyllav1.DatacenterSpec{
				Name: dc,
				Racks: []scyllav1.RackSpec{
					{
						Name:    rack,
						Members: members,
						Storage: scyllav1.StorageSpec{
							Capacity: "5Gi",
						},
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			ObservedGeneration: pointer.Ptr(int64(1)),
			Racks: map[string]scyllav1.RackStatus{
				rack: {
					Version:      version,
					Members:      members,
					ReadyMembers: members,
					Stale:        pointer.Ptr(false),
				},
			},
		},
	}
}

// NewDetailedMultiRackCluster creates multi rack cluster with supplied information.
func NewDetailedMultiRackCluster(name, namespace, repo, version, dc string, members ...int32) *scyllav1.ScyllaCluster {
	c := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Repository: repo,
			Version:    version,
			Datacenter: scyllav1.DatacenterSpec{
				Name:  dc,
				Racks: []scyllav1.RackSpec{},
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			ObservedGeneration: pointer.Ptr(int64(1)),
			Racks:              map[string]scyllav1.RackStatus{},
		},
	}

	for i, m := range members {
		rack := fmt.Sprintf("rack-%d", i)
		c.Spec.Datacenter.Racks = append(c.Spec.Datacenter.Racks, scyllav1.RackSpec{
			Name:    rack,
			Members: m,
			Storage: scyllav1.StorageSpec{
				Capacity: "5Gi",
			},
		})
		c.Status.Racks[rack] = scyllav1.RackStatus{
			Version:      version,
			Members:      m,
			ReadyMembers: m,
			Stale:        pointer.Ptr(false),
		}
	}

	return c
}
