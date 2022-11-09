// Copyright (c) 2022 ScyllaDB.

package conversion_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/conversion"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestScyllaClusterV1ConversionSpecAndStatusIdentity(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		cluster *scyllav1.ScyllaCluster
	}{
		{
			name:    "basic cluster",
			cluster: basicV1ScyllaCluster(),
		},
		{
			name: "multi rack cluster with different rack sizes",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.Datacenter.Racks = append(cluster.Spec.Datacenter.Racks, scyllav1.RackSpec{
					Name:    "rack-2",
					Members: 123,
					Storage: scyllav1.StorageSpec{
						Capacity:         "666Gi",
						StorageClassName: pointer.String("rack-2-storage-class-name"),
					},
					Placement: nil,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("42"),
							corev1.ResourceMemory: resource.MustParse("666Gi"),
						},
					},
					AgentResources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("24"),
							corev1.ResourceMemory: resource.MustParse("111Gi"),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "configmap",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-config-map"},
								},
							},
						},
						{
							Name: "secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "secret",
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "configmap",
							MountPath: "/mnt/configmap",
						},
					},
					AgentVolumeMounts: []corev1.VolumeMount{
						{
							Name:      "secret",
							MountPath: "/mnt/secret",
						},
					},
					ScyllaConfig:      "my-scylla-config",
					ScyllaAgentConfig: "my-agent-config",
				})

				return cluster
			}(),
		},
		{
			name: "cluster with host networking",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.Network.HostNetworking = true

				return cluster
			}(),
		},
		{
			name: "cluster with custom sysctls",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.Sysctls = []string{"fs.max-aio-nr=2137"}

				return cluster
			}(),
		},
		{
			name: "cluster with automatedOrphanedNodeCleanup",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.AutomaticOrphanedNodeCleanup = true

				return cluster
			}(),
		},
		{
			name: "cluster with repair task",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.Repairs = []scyllav1.RepairTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:       "repair",
							StartDate:  "now",
							Interval:   "1d",
							NumRetries: pointer.Int64(3),
						},
						DC:                  []string{"dc-1"},
						FailFast:            true,
						Intensity:           "1",
						Parallel:            1,
						Keyspace:            []string{"keyspace"},
						SmallTableThreshold: "1GiB",
						Host:                pointer.String("1.1.1.1"),
					},
				}

				return cluster
			}(),
		},
		{
			name: "cluster with backup task",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Spec.Backups = []scyllav1.BackupTaskSpec{
					{
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Name:       "repair",
							StartDate:  "now",
							Interval:   "1d",
							NumRetries: pointer.Int64(3),
						},
						DC:               []string{"dc-1"},
						Keyspace:         []string{"keyspace"},
						Location:         []string{"s3:my-bucket"},
						RateLimit:        []string{"100MiB"},
						Retention:        3,
						SnapshotParallel: []string{"3"},
						UploadParallel:   []string{"3"},
					},
				}

				return cluster
			}(),
		},
		{
			name: "cluster with manager ID",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Status.ManagerID = pointer.String("manager-id")

				return cluster
			}(),
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var identity *scyllav1.ScyllaCluster
			pre := tc.cluster

			for i := 0; i < 10; i++ {
				post, err := conversion.ScyllaClusterFromV1ToV2Alpha1(pre)
				if err != nil {
					t.Fatal(err)
				}

				identity, err = conversion.ScyllaClusterFromV2Alpha1oV1(post)
				if err != nil {
					t.Fatal(err)
				}

				pre = identity
			}

			// Annotations keeps the state between conversion, ignore them in identity comparison.
			identity.Annotations = nil

			if !apiequality.Semantic.DeepEqual(tc.cluster, identity) {
				t.Errorf("expected and gotten ScyllaCluster differ: \n%s", cmp.Diff(tc.cluster, identity))
			}
		})
	}
}

func TestScyllaClusterV2Alpha1ConversionSpecAndStatusIdentity(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		cluster *scyllav2alpha1.ScyllaCluster
	}{
		{
			name:    "basic cluster",
			cluster: basicV2Alpha1ScyllaCluster(),
		},
		{
			name: "multi rack cluster",
			cluster: func() *scyllav2alpha1.ScyllaCluster {
				cluster := basicV2Alpha1ScyllaCluster()
				cluster.Spec.Datacenters[0].Racks = append(cluster.Spec.Datacenters[0].Racks, scyllav2alpha1.RackSpec{
					Name: "b",
					Placement: &scyllav2alpha1.Placement{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "foo",
												Operator: "In",
												Values:   []string{"bar"},
											},
										},
									},
								},
							},
						},
					},
				})
				cluster.Status.Datacenters[0].Racks = append(cluster.Status.Datacenters[0].Racks, scyllav2alpha1.RackStatus{
					Name:           "b",
					CurrentVersion: "docker.io/scylladb/scylla:5.0.0",
					Nodes:          pointer.Int32(3),
					UpdatedNodes:   pointer.Int32(3),
					ReadyNodes:     pointer.Int32(3),
					Stale:          pointer.Bool(false),
				})

				cluster.Status.Datacenters[0].Nodes = pointer.Int32(6)
				cluster.Status.Datacenters[0].UpdatedNodes = pointer.Int32(6)
				cluster.Status.Datacenters[0].ReadyNodes = pointer.Int32(6)

				cluster.Status.Nodes = pointer.Int32(6)
				cluster.Status.UpdatedNodes = pointer.Int32(6)
				cluster.Status.ReadyNodes = pointer.Int32(6)

				return cluster
			}(),
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var identity *scyllav2alpha1.ScyllaCluster
			pre := tc.cluster

			for i := 0; i < 10; i++ {
				post, err := conversion.ScyllaClusterFromV2Alpha1oV1(pre)
				if err != nil {
					t.Fatal(err)
				}

				identity, err = conversion.ScyllaClusterFromV1ToV2Alpha1(post)
				if err != nil {
					t.Fatal(err)
				}

				pre = identity
			}

			// Annotations keep state required for conversion, ignore them in identity test.
			identity.Annotations = nil

			if !apiequality.Semantic.DeepEqual(tc.cluster, identity) {
				t.Errorf("expected and gotten ScyllaCluster differ: \n%s", cmp.Diff(tc.cluster, identity))
			}
		})
	}
}

func TestScyllaClusterFromV1ToV2Alpha1(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		cluster  *scyllav1.ScyllaCluster
		expected *scyllav2alpha1.ScyllaCluster
	}{
		{
			name:    "basic cluster",
			cluster: basicV1ScyllaCluster(),
			expected: func() *scyllav2alpha1.ScyllaCluster {
				cluster := basicV2Alpha1ScyllaCluster()
				cluster.Annotations = map[string]string{
					naming.ScyllaClusterV1Annotation: mustEncode(basicV1ScyllaCluster()),
				}
				return cluster
			}(),
		},
		{
			name: "cluster with kubeconfig secret ref annotation",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Annotations = map[string]string{
					naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation: "kubeconfig-secret-ref",
				}
				return cluster
			}(),
			expected: func() *scyllav2alpha1.ScyllaCluster {
				cluster := basicV2Alpha1ScyllaCluster()
				cluster.Annotations = map[string]string{
					naming.ScyllaClusterV1Annotation: mustEncode(basicV1ScyllaCluster()),
				}
				cluster.Spec.Datacenters[0].RemoteKubeClusterConfigRef = &scyllav2alpha1.RemoteKubeClusterConfigRef{Name: "kubeconfig-secret-ref"}
				return cluster
			}(),
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := conversion.ScyllaClusterFromV1ToV2Alpha1(tc.cluster)
			if err != nil {
				t.Fatal(err)
			}

			if !apiequality.Semantic.DeepEqual(tc.expected, got) {
				t.Errorf("expected and gotten ScyllaCluster differ: \n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestScyllaClusterFromV2Alpha1oV1(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		cluster  *scyllav2alpha1.ScyllaCluster
		expected *scyllav1.ScyllaCluster
	}{
		{
			name:     "basic cluster",
			cluster:  basicV2Alpha1ScyllaCluster(),
			expected: basicV1ScyllaCluster(),
		},
		{
			name: "basic cluster with kubeconfig ref",
			cluster: func() *scyllav2alpha1.ScyllaCluster {
				cluster := basicV2Alpha1ScyllaCluster()
				cluster.Spec.Datacenters[0].RemoteKubeClusterConfigRef = &scyllav2alpha1.RemoteKubeClusterConfigRef{Name: "kubeconfig-secret-ref"}
				return cluster
			}(),
			expected: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				cluster.Annotations = map[string]string{
					naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation: "kubeconfig-secret-ref",
				}
				return cluster
			}(),
		},
		{
			name: "basic cluster with stale datacenter",
			cluster: func() *scyllav2alpha1.ScyllaCluster {
				cluster := basicV2Alpha1ScyllaCluster()
				cluster.Status.Datacenters[0].Stale = pointer.Bool(true)
				return cluster
			}(),
			expected: func() *scyllav1.ScyllaCluster {
				cluster := basicV1ScyllaCluster()
				rackStatus := cluster.Status.Racks[cluster.Spec.Datacenter.Racks[0].Name]
				rackStatusCopy := rackStatus.DeepCopy()
				rackStatusCopy.Stale = pointer.Bool(true)
				cluster.Status.Racks[cluster.Spec.Datacenter.Racks[0].Name] = *rackStatusCopy

				cluster.Annotations = map[string]string{
					naming.ScyllaClusterV2alpha1ScyllaDatacenterStaleAnnotation: "true",
				}
				return cluster
			}(),
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := conversion.ScyllaClusterFromV2Alpha1oV1(tc.cluster)
			if err != nil {
				t.Fatal(err)
			}

			if !apiequality.Semantic.DeepEqual(tc.expected, got) {
				t.Errorf("expected and gotten ScyllaCluster differ: \n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func basicV2Alpha1ScyllaCluster() *scyllav2alpha1.ScyllaCluster {
	return &scyllav2alpha1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-cluster",
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
					Name:         "us-east-1",
					NodesPerRack: pointer.Int32(3),
					Scylla: &scyllav2alpha1.ScyllaOverrides{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
						Storage: &scyllav2alpha1.Storage{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("100Mi"),
								},
							},
							StorageClassName: pointer.String("my-storage-class"),
						},
					},
					ScyllaManagerAgent: &scyllav2alpha1.ScyllaManagerAgentOverrides{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
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
		Status: scyllav2alpha1.ScyllaClusterStatus{
			Nodes:        pointer.Int32(3),
			UpdatedNodes: pointer.Int32(3),
			ReadyNodes:   pointer.Int32(3),
			Datacenters: []scyllav2alpha1.DatacenterStatus{
				{
					Name:           "us-east-1",
					CurrentVersion: "docker.io/scylladb/scylla:5.0.0",
					Nodes:          pointer.Int32(3),
					UpdatedNodes:   pointer.Int32(3),
					ReadyNodes:     pointer.Int32(3),
					Racks: []scyllav2alpha1.RackStatus{
						{
							Name:           "a",
							CurrentVersion: "docker.io/scylladb/scylla:5.0.0",
							Nodes:          pointer.Int32(3),
							UpdatedNodes:   pointer.Int32(3),
							ReadyNodes:     pointer.Int32(3),
							Stale:          pointer.Bool(false),
						},
					},
				},
			},
		},
	}
}
func basicV1ScyllaCluster() *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-cluster",
			Namespace: "scylla",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Version:         "5.0.0",
			Repository:      "docker.io/scylladb/scylla",
			AgentVersion:    "3.0.0",
			AgentRepository: "docker.io/scylladb/scylla-manager-agent",
			Datacenter: scyllav1.DatacenterSpec{
				Name: "us-east-1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "a",
						Members: 3,
						Storage: scyllav1.StorageSpec{
							Capacity:         "100Mi",
							StorageClassName: pointer.String("my-storage-class"),
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
						AgentResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
					},
				},
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			Racks: map[string]scyllav1.RackStatus{
				"a": {
					Version:        "5.0.0",
					Members:        3,
					ReadyMembers:   3,
					UpdatedMembers: pointer.Int32(3),
					Stale:          pointer.Bool(false),
				},
			},
		},
	}
}

func mustEncode(v any) string {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		panic(err)
	}
	return buf.String()
}
