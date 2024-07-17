// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ConvertV1Alpha1ScyllaDBDatacenterToV1ScyllaCluster(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                  string
		scyllaDBDatacenter    *scyllav1alpha1.ScyllaDBDatacenter
		expectedScyllaCluster *scyllav1.ScyllaCluster
		expectedErr           error
	}{
		{
			name:                  "valid conversion with all fields",
			scyllaDBDatacenter:    newBasicScyllaDBDatacenter(),
			expectedScyllaCluster: newBasicScyllaCluster(),
			expectedErr:           nil,
		},
		{
			name: "host networking in annotations",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenter()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/host-networking": "true",
				}
				return sd
			}(),
			expectedScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/host-networking": "true",
				}
				sc.Spec.Network.HostNetworking = true
				return sc
			}(),
			expectedErr: nil,
		},
		{
			name: "sysctls in annotations",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenter()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/sysctls": `["zoo=foo", "foo=bar"]`,
				}
				return sd
			}(),
			expectedScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/sysctls": `["zoo=foo", "foo=bar"]`,
				}
				sc.Spec.Sysctls = []string{
					"zoo=foo",
					"foo=bar",
				}
				return sc
			}(),
			expectedErr: nil,
		},
		{
			name: "alternator port in annotations",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenter()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-port": "9000",
				}
				return sd
			}(),
			expectedScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-port": "9000",
				}
				sc.Spec.Alternator.Port = 9000
				return sc
			}(),
			expectedErr: nil,
		},
		{
			name: "alternator insecure http in annotations",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenter()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-enable-http": "true",
				}
				return sd
			}(),
			expectedScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-enable-http": "true",
				}
				sc.Spec.Alternator.InsecureEnableHTTP = pointer.Ptr(true)
				return sc
			}(),
			expectedErr: nil,
		},
		{
			name: "alternator insecure disable authorization in annotations",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenter()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "true",
				}
				return sd
			}(),
			expectedScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "true",
				}
				sc.Spec.Alternator.InsecureDisableAuthorization = pointer.Ptr(true)
				return sc
			}(),
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ConvertV1Alpha1ScyllaDBDatacenterToV1ScyllaCluster(tc.scyllaDBDatacenter)

			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(got, tc.expectedScyllaCluster) {
				t.Errorf("expected and got scyllacluster differ, diff %v", cmp.Diff(tc.expectedScyllaCluster, got))
			}
		})
	}
}

func Test_ConvertV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	newBasicScyllaDBDatacenterWithNoStatus := func() *scyllav1alpha1.ScyllaDBDatacenter {
		sd := newBasicScyllaDBDatacenter()
		sd.Status = scyllav1alpha1.ScyllaDBDatacenterStatus{}
		return sd
	}

	tt := []struct {
		name                       string
		scyllaCluster              *scyllav1.ScyllaCluster
		expectedScyllaDBDatacenter *scyllav1alpha1.ScyllaDBDatacenter
		expectedErr                error
	}{
		{
			name:                       "valid conversion with all fields except Status",
			scyllaCluster:              newBasicScyllaCluster(),
			expectedScyllaDBDatacenter: newBasicScyllaDBDatacenterWithNoStatus(),
		},
		{
			name: "hostNetworking propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Network.HostNetworking = true
				return sc
			}(),
			expectedScyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenterWithNoStatus()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/host-networking": "true",
				}
				return sd
			}(),
		},
		{
			name: "sysctls propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Sysctls = []string{"foo=bar", "zoo=foo"}
				return sc
			}(),
			expectedScyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenterWithNoStatus()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/sysctls": "[\"foo=bar\",\"zoo=foo\"]\n",
				}
				return sd
			}(),
		},
		{
			name: "alternator port propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Alternator.Port = 9000
				return sc
			}(),
			expectedScyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenterWithNoStatus()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-port": "9000",
				}
				return sd
			}(),
		},
		{
			name: "alternator insecure enable http propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Alternator.InsecureEnableHTTP = pointer.Ptr(true)
				return sc
			}(),
			expectedScyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenterWithNoStatus()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-enable-http": "true",
				}
				return sd
			}(),
		},
		{
			name: "alternator insecure disable authorization propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Alternator.InsecureDisableAuthorization = pointer.Ptr(true)
				return sc
			}(),
			expectedScyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sd := newBasicScyllaDBDatacenterWithNoStatus()
				sd.Annotations = map[string]string{
					"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "true",
				}
				return sd
			}(),
		},
		// TODO: ugprade
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ConvertV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(tc.scyllaCluster)
			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(got, tc.expectedScyllaDBDatacenter) {
				t.Errorf("expected and got scylladbdatacenter differ, diff %v", cmp.Diff(tc.expectedScyllaDBDatacenter, got))
			}
		})
	}
}

func newBasicScyllaCluster() *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			PodMetadata: &scyllav1.ObjectTemplateMetadata{
				Labels: map[string]string{
					"label": "value",
				},
				Annotations: map[string]string{
					"annotation": "value",
				},
			},
			Version:    "latest",
			Repository: "docker.io/scylladb/scylla-enterprise",
			Alternator: &scyllav1.AlternatorSpec{
				WriteIsolation: "write_isolation",
				ServingCertificate: &scyllav1.TLSCertificate{
					Type: "UserManaged",
					UserManagedOptions: &scyllav1.UserManagedTLSCertificateOptions{
						SecretName: "foo",
					},
					OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
						AdditionalDNSNames: []string{
							"dns-1",
							"dns-2",
						},
						AdditionalIPAddresses: []string{
							"ip-1",
							"ip-2",
						},
					},
				},
			},
			AgentVersion:                 "latest",
			AgentRepository:              "docker.io/scylladb/scylla-manager-agent",
			DeveloperMode:                true,
			CpuSet:                       false,
			AutomaticOrphanedNodeCleanup: false,
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "a",
						Members: 3,
						Storage: scyllav1.Storage{
							Metadata: &scyllav1.ObjectTemplateMetadata{
								Labels: map[string]string{
									"storage-label": "value",
								},
								Annotations: map[string]string{
									"storage-annotation": "value",
								},
							},
							Capacity:         "123Gi",
							StorageClassName: pointer.Ptr("custom-storage-class"),
						},
						Placement: &scyllav1.PlacementSpec{
							NodeAffinity:    newNodeAffinity(),
							PodAffinity:     newPodAffinity(),
							PodAntiAffinity: newPodAntiAffinity(),
							Tolerations:     newTolerations(),
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						AgentResources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("3"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("5"),
								corev1.ResourceMemory: resource.MustParse("6Gi"),
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "custom-scylla-config-map-volume-name",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "custom-scylla-config-map-name",
										},
										Optional: pointer.Ptr(true),
									},
								},
							},
							{
								Name: "custom-agent-secret-volume-name",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "custom-agent-secret-name",
										Optional:   pointer.Ptr(true),
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "custom-scylla-config-map-volume-name",
								ReadOnly:  true,
								MountPath: "/var/foo/bar",
							},
						},
						AgentVolumeMounts: []corev1.VolumeMount{
							{
								Name:      "custom-agent-secret-volume-name",
								ReadOnly:  true,
								MountPath: "/var/foo/bar",
							},
						},
						ScyllaConfig:      "custom-scylla-config-map",
						ScyllaAgentConfig: "custom-agent-secret",
					},
				},
			},
			Sysctls:    nil,
			ScyllaArgs: "arg-1 arg-2",
			Network: scyllav1.Network{
				HostNetworking: false,
				DNSPolicy:      corev1.DNSClusterFirst,
			},
			Repairs:                 nil,
			Backups:                 nil,
			ForceRedeploymentReason: "force-redeployment-reason",
			ImagePullSecrets: []corev1.LocalObjectReference{
				{
					Name: "image-pull-secret",
				},
			},
			DNSDomains: []string{"dns-domain-1", "dns-domain-2"},
			ExposeOptions: &scyllav1.ExposeOptions{
				CQL: &scyllav1.CQLExposeOptions{
					Ingress: &scyllav1.IngressOptions{
						Disabled: pointer.Ptr(false),
						ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"ingress-label": "value",
							},
							Annotations: map[string]string{
								"ingress-annotation": "value",
							},
						},
						IngressClassName: "ingress-class-name",
					},
				},
				NodeService: &scyllav1.NodeServiceTemplate{
					ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
						Labels: map[string]string{
							"node-service-label": "value",
						},
						Annotations: map[string]string{
							"node-service-annotation": "value",
						},
					},
					Type:                          scyllav1.NodeServiceTypeHeadless,
					ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyCluster),
					InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
					AllocateLoadBalancerNodePorts: pointer.Ptr(true),
					LoadBalancerClass:             pointer.Ptr("load-balancer-class"),
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
						PodIP: &scyllav1.PodIPAddressOptions{
							Source: scyllav1.StatusPodIPSource,
						},
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
						PodIP: &scyllav1.PodIPAddressOptions{
							Source: scyllav1.StatusPodIPSource,
						},
					},
				},
			},
			ExternalSeeds:                    []string{"seed1", "seed2"},
			MinTerminationGracePeriodSeconds: pointer.Ptr[int32](123),
			MinReadySeconds:                  pointer.Ptr[int32](321),
			ReadinessGates: []corev1.PodReadinessGate{
				{
					ConditionType: "condition-type",
				},
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			ObservedGeneration: pointer.Ptr[int64](123),
			Racks: map[string]scyllav1.RackStatus{
				"a": {
					Version:          "rack-current-version",
					Members:          6,
					ReadyMembers:     7,
					AvailableMembers: pointer.Ptr[int32](8),
					UpdatedMembers:   pointer.Ptr[int32](9),
					Stale:            pointer.Ptr(true),
				},
			},
			Members:          pointer.Ptr[int32](1),
			ReadyMembers:     pointer.Ptr[int32](4),
			AvailableMembers: pointer.Ptr[int32](5),
			RackCount:        pointer.Ptr[int32](1),
			ManagerID:        nil,
			Repairs:          nil,
			Backups:          nil,
			Upgrade:          nil,
			Conditions: []metav1.Condition{
				{
					Type:               "condition-type",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 123,
					LastTransitionTime: metav1.Time{},
					Reason:             "condition-reason",
					Message:            "condition-message",
				},
			},
		},
	}
}

func newBasicScyllaDBDatacenter() *scyllav1alpha1.ScyllaDBDatacenter {
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-cluster",
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			Metadata: scyllav1alpha1.ObjectTemplateMetadata{
				Labels: map[string]string{
					"label": "value",
				},
				Annotations: map[string]string{
					"annotation": "value",
				},
			},
			ClusterName:    "simple-cluster",
			DatacenterName: "dc1",
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image:         "docker.io/scylladb/scylla-enterprise:latest",
				ExternalSeeds: []string{"seed1", "seed2"},
				AlternatorOptions: &scyllav1alpha1.AlternatorOptions{
					WriteIsolation: "write_isolation",
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeUserManaged,
						UserManagedOptions: &scyllav1alpha1.UserManagedTLSCertificateOptions{
							SecretName: "foo",
						},
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames:    []string{"dns-1", "dns-2"},
							AdditionalIPAddresses: []string{"ip-1", "ip-2"},
						},
					},
				},
				AdditionalScyllaDBArguments: []string{"arg-1", "arg-2"},
				EnableDeveloperMode:         pointer.Ptr(true),
			},
			ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
				Image: pointer.Ptr("docker.io/scylladb/scylla-manager-agent:latest"),
			},
			ImagePullSecrets:        []corev1.LocalObjectReference{{Name: "image-pull-secret"}},
			DNSPolicy:               pointer.Ptr(corev1.DNSClusterFirst),
			DNSDomains:              []string{"dns-domain-1", "dns-domain-2"},
			ForceRedeploymentReason: "force-redeployment-reason",
			ExposeOptions: &scyllav1alpha1.ExposeOptions{
				CQL: &scyllav1alpha1.CQLExposeOptions{
					Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"ingress-label": "value",
							},
							Annotations: map[string]string{
								"ingress-annotation": "value",
							},
						},
						IngressClassName: "ingress-class-name",
					},
				},
				NodeService: &scyllav1alpha1.NodeServiceTemplate{
					ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
						Labels: map[string]string{
							"node-service-label": "value",
						},
						Annotations: map[string]string{
							"node-service-annotation": "value",
						},
					},
					Type:                          scyllav1alpha1.NodeServiceTypeHeadless,
					ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyCluster),
					InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
					AllocateLoadBalancerNodePorts: pointer.Ptr(true),
					LoadBalancerClass:             pointer.Ptr("load-balancer-class"),
				},
				BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
					Nodes: scyllav1alpha1.BroadcastOptions{
						Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						PodIP: &scyllav1alpha1.PodIPAddressOptions{
							Source: scyllav1alpha1.StatusPodIPSource,
						},
					},
					Clients: scyllav1alpha1.BroadcastOptions{
						Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						PodIP: &scyllav1alpha1.PodIPAddressOptions{
							Source: scyllav1alpha1.StatusPodIPSource,
						},
					},
				},
			},
			DisableAutomaticOrphanedNodeReplacement: true,
			MinTerminationGracePeriodSeconds:        pointer.Ptr[int32](123),
			MinReadySeconds:                         pointer.Ptr[int32](321),
			ReadinessGates: []corev1.PodReadinessGate{
				{
					ConditionType: "condition-type",
				},
			},
			Racks: []scyllav1alpha1.RackSpec{
				{
					RackTemplate: newBasicRackTemplate(),
					Name:         "a",
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
			ObservedGeneration: pointer.Ptr[int64](123),
			Conditions: []metav1.Condition{
				{
					Type:               "condition-type",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 123,
					LastTransitionTime: metav1.Time{},
					Reason:             "condition-reason",
					Message:            "condition-message",
				},
			},
			CurrentVersion: "current-version",
			UpdatedVersion: "updated-version",
			Nodes:          pointer.Ptr[int32](1),
			CurrentNodes:   pointer.Ptr[int32](2),
			UpdatedNodes:   pointer.Ptr[int32](3),
			ReadyNodes:     pointer.Ptr[int32](4),
			AvailableNodes: pointer.Ptr[int32](5),
			Racks: []scyllav1alpha1.RackStatus{
				{
					Name:           "a",
					CurrentVersion: "rack-current-version",
					Nodes:          pointer.Ptr[int32](6),
					ReadyNodes:     pointer.Ptr[int32](7),
					AvailableNodes: pointer.Ptr[int32](8),
					UpdatedNodes:   pointer.Ptr[int32](9),
					Stale:          pointer.Ptr(true),
				},
			},
		},
	}
}

func newBasicRackTemplate() scyllav1alpha1.RackTemplate {
	return scyllav1alpha1.RackTemplate{
		Nodes: pointer.Ptr[int32](3),
		Placement: &scyllav1alpha1.Placement{
			NodeAffinity:    newNodeAffinity(),
			PodAffinity:     newPodAffinity(),
			PodAntiAffinity: newPodAntiAffinity(),
			Tolerations:     newTolerations(),
		},
		TopologyLabelSelector: nil,
		ScyllaDB:              pointer.Ptr(newBasicScyllaDBTemplate()),
		ScyllaDBManagerAgent:  pointer.Ptr(newBasicScyllaDBManagerAgentTemplate()),
	}
}

func newTolerations() []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      "tolerations-key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}

func newPodAntiAffinity() *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pod-anti-affinity-key": "value",
					},
				},
			},
		},
	}
}

func newPodAffinity() *corev1.PodAffinity {
	return &corev1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pod-affinity-key": "value",
					},
				},
			},
		},
	}
}

func newNodeAffinity() *corev1.NodeAffinity {
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "node-affinity-key",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"value"},
						},
					},
				},
			},
		},
	}
}

func newBasicScyllaDBManagerAgentTemplate() scyllav1alpha1.ScyllaDBManagerAgentTemplate {
	return scyllav1alpha1.ScyllaDBManagerAgentTemplate{
		Resources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5"),
				corev1.ResourceMemory: resource.MustParse("6Gi"),
			},
		},
		CustomConfigSecretRef: pointer.Ptr("custom-agent-secret"),
		Volumes: []corev1.Volume{
			{
				Name: "custom-agent-secret-volume-name",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "custom-agent-secret-name",
						Optional:   pointer.Ptr(true),
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "custom-agent-secret-volume-name",
				ReadOnly:  true,
				MountPath: "/var/foo/bar",
			},
		},
	}
}

func newBasicScyllaDBTemplate() scyllav1alpha1.ScyllaDBTemplate {
	return scyllav1alpha1.ScyllaDBTemplate{
		Resources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
		Storage: &scyllav1alpha1.StorageOptions{
			Metadata: scyllav1alpha1.ObjectTemplateMetadata{
				Labels: map[string]string{
					"storage-label": "value",
				},
				Annotations: map[string]string{
					"storage-annotation": "value",
				},
			},
			Capacity:         "123Gi",
			StorageClassName: pointer.Ptr("custom-storage-class"),
		},
		CustomConfigMapRef: pointer.Ptr("custom-scylla-config-map"),
		Volumes: []corev1.Volume{
			{
				Name: "custom-scylla-config-map-volume-name",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "custom-scylla-config-map-name",
						},
						Optional: pointer.Ptr(true),
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "custom-scylla-config-map-volume-name",
				ReadOnly:  true,
				MountPath: "/var/foo/bar",
			},
		},
	}
}
