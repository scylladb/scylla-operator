// Copyright (c) 2022 ScyllaDB.

package v2alpha1_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	validation "github.com/scylladb/scylla-operator/pkg/api/validation/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestValidateScyllaCluster(t *testing.T) {
	tests := []struct {
		name                string
		obj                 *scyllav2alpha1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			obj:                 validCluster(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "too many datacenters",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters = append(cluster.Spec.Datacenters, *cluster.Spec.Datacenters[0].DeepCopy())
				cluster.Spec.Datacenters[1].Name = "us-east-2"
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeTooMany, Field: "spec.datacenters", BadValue: 2, Detail: "must have at most 1 items"},
			},
			expectedErrorString: `spec.datacenters: Too many: 2: must have at most 1 items`,
		},
		{
			name: "duplicate rack names",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].Racks = append(cluster.Spec.Datacenters[0].Racks, *cluster.Spec.Datacenters[0].Racks[0].DeepCopy())
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenters[0].racks[1].name", BadValue: "a"},
			},
			expectedErrorString: `spec.datacenters[0].racks[1].name: Duplicate value: "a"`,
		},
		{
			name: "missing scylla properties",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].Scylla = nil
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenters[0].scylla", BadValue: "", Detail: "scylla datacenter properties are required"},
			},
			expectedErrorString: `spec.datacenters[0].scylla: Required value: scylla datacenter properties are required`,
		},
		{
			name: "missing scylla resources",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].Scylla.Resources = nil
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenters[0].scylla.resources", BadValue: "", Detail: "scylla resources are required"},
			},
			expectedErrorString: `spec.datacenters[0].scylla.resources: Required value: scylla resources are required`,
		},
		{
			name: "missing scylla manager agent properties",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].ScyllaManagerAgent = nil
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenters[0].scyllaManagerAgent", BadValue: "", Detail: "scylla manager agent datacenter properties are required when agent is enabled"},
			},
			expectedErrorString: `spec.datacenters[0].scyllaManagerAgent: Required value: scylla manager agent datacenter properties are required when agent is enabled`,
		},
		{
			name: "missing scylla manager agent resources",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].ScyllaManagerAgent.Resources = nil
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenters[0].scyllaManagerAgent.resources", BadValue: "", Detail: "scylla manager agent resources are required"},
			},
			expectedErrorString: `spec.datacenters[0].scyllaManagerAgent.resources: Required value: scylla manager agent resources are required`,
		},
		{
			name: "missing nodes per rack",
			obj: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].NodesPerRack = nil
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenters[0].nodesPerRack", BadValue: "", Detail: "number of nodes per rack is required"},
			},
			expectedErrorString: `spec.datacenters[0].nodesPerRack: Required value: number of nodes per rack is required`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaCluster(test.obj)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaClusterUpdate(t *testing.T) {
	tests := []struct {
		name                string
		old                 *scyllav2alpha1.ScyllaCluster
		new                 *scyllav2alpha1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 validCluster(),
			new:                 validCluster(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "storage changed",
			old:  validCluster(),
			new: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].Scylla.Storage.StorageClassName = pointer.String("different-class-name")
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].scylla.storage", BadValue: "", Detail: "changes in storage are currently not supported"},
			},
			expectedErrorString: "spec.datacenters[0].scylla.storage: Forbidden: changes in storage are currently not supported",
		},
		{
			name: "remove empty datacenter",
			old: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].NodesPerRack = pointer.Int32(0)
				return cluster
			}(),
			new: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters = []scyllav2alpha1.Datacenter{}
				return cluster
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "remove empty datacenter with nodes under decommission",
			old: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].NodesPerRack = pointer.Int32(0)
				cluster.Status.Datacenters = []scyllav2alpha1.DatacenterStatus{
					{
						Name:  cluster.Spec.Datacenters[0].Name,
						Nodes: pointer.Int32(3),
					},
				}
				return cluster
			}(),
			new: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters = []scyllav2alpha1.Datacenter{}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0]", BadValue: "", Detail: `datacenter "us-east-1" can't be removed because the nodes are being scaled down`},
			},
			expectedErrorString: `spec.datacenters[0]: Forbidden: datacenter "us-east-1" can't be removed because the nodes are being scaled down`,
		},
		{
			name: "remove empty datacenter with stale status",
			old: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].NodesPerRack = pointer.Int32(0)
				cluster.Status.Datacenters = []scyllav2alpha1.DatacenterStatus{
					{
						Name:  cluster.Spec.Datacenters[0].Name,
						Nodes: pointer.Int32(0),
						Stale: pointer.Bool(true),
					},
				}
				return cluster
			}(),
			new: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters = []scyllav2alpha1.Datacenter{}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenters[0]", Detail: `datacenter "us-east-1" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenters[0]: Internal error: datacenter "us-east-1" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "remove empty datacenter with not reconciled generation",
			old: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters[0].NodesPerRack = pointer.Int32(0)
				cluster.Status.ObservedGeneration = pointer.Int64(666)
				cluster.Status.Datacenters = []scyllav2alpha1.DatacenterStatus{
					{
						Name:  cluster.Spec.Datacenters[0].Name,
						Nodes: pointer.Int32(0),
					},
				}
				return cluster
			}(),
			new: func() *scyllav2alpha1.ScyllaCluster {
				cluster := validCluster()
				cluster.Spec.Datacenters = []scyllav2alpha1.Datacenter{}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenters[0]", Detail: `datacenter "us-east-1" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenters[0]: Internal error: datacenter "us-east-1" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaClusterUpdate(test.new, test.old)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func validCluster() *scyllav2alpha1.ScyllaCluster {
	return &scyllav2alpha1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-cluster",
			Namespace: "scylla",
		},
		Spec: scyllav2alpha1.ScyllaClusterSpec{
			Scylla: scyllav2alpha1.Scylla{
				Image: "docker.io/scylladb/scylla:5.0.0",
			},
			ScyllaManagerAgent: &scyllav2alpha1.ScyllaManagerAgent{
				Image: "docker.io/scylladb/scylla-manager-agent:3.0.0",
			},
			Datacenters: []scyllav2alpha1.Datacenter{
				{
					Name:                       "us-east-1",
					RemoteKubeClusterConfigRef: &scyllav2alpha1.RemoteKubeClusterConfigRef{Name: "kubeconfig"},
					NodesPerRack:               pointer.Int32(3),
					Scylla: &scyllav2alpha1.ScyllaOverrides{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
						},
						Storage: &scyllav2alpha1.Storage{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("100G"),
								},
							},
						},
					},
					ScyllaManagerAgent: &scyllav2alpha1.ScyllaManagerAgentOverrides{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("50M"),
							},
						},
					},
					Racks: []scyllav2alpha1.RackSpec{
						{
							Name: "a",
						},
					},
				},
			},
		},
	}
}
