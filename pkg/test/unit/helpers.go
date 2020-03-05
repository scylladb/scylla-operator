package unit

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSingleRackCluster(members int32) *scyllav1alpha1.Cluster {
	return NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "test-dc", "test-rack", members)
}

func NewDetailedSingleRackCluster(name, namespace, repo, version, dc, rack string, members int32) *scyllav1alpha1.Cluster {
	return &scyllav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ClusterSpec{
			Repository: util.RefFromString(repo),
			Version:    version,
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name: dc,
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name:    rack,
						Members: members,
						Storage: scyllav1alpha1.StorageSpec{
							Capacity: "5Gi",
						},
					},
				},
			},
		},
		Status: scyllav1alpha1.ClusterStatus{
			Racks: map[string]*scyllav1alpha1.RackStatus{
				rack: {
					Version:      version,
					Members:      members,
					ReadyMembers: members,
				},
			},
		},
	}
}
