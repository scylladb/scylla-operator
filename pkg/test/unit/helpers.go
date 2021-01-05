package unit

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSingleRackCluster(members int32) *scyllav1.ScyllaCluster {
	return NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "test-dc", "test-rack", members)
}

func NewDetailedSingleRackCluster(name, namespace, repo, version, dc, rack string, members int32) *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1.ClusterSpec{
			Repository: util.RefFromString(repo),
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
					},
				},
			},
		},
		Status: scyllav1.ClusterStatus{
			Racks: map[string]scyllav1.RackStatus{
				rack: {
					Version:      version,
					Members:      members,
					ReadyMembers: members,
				},
			},
		},
	}
}
