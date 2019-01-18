package unit

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSingleRackCluster(members int32) *scyllav1alpha1.Cluster {
	return &scyllav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
		},
		Spec: scyllav1alpha1.ClusterSpec{
			Version: "2.3.1",
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name: "test-dc",
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name:    "test-rack",
						Members: members,
					},
				},
			},
		},
	}
}
