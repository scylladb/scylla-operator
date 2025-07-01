// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMigrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                        string
		scyllaDBDatacenter          *scyllav1alpha1.ScyllaDBDatacenter
		configMaps                  []*corev1.ConfigMap
		services                    []*corev1.Service
		expectedScyllaClusterStatus scyllav1.ScyllaClusterStatus
	}{
		{
			name:                        "valid migration with all fields",
			scyllaDBDatacenter:          newBasicScyllaDBDatacenter(),
			configMaps:                  nil,
			services:                    nil,
			expectedScyllaClusterStatus: newBasicScyllaCluster().Status,
		},
		{
			name:       "decommissioning and leaving rack condition when one of the member services has special label",
			configMaps: nil,
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"scylla/rack":           "a",
							"scylla/decommissioned": "false",
						},
					},
				},
			},
			scyllaDBDatacenter: newBasicScyllaDBDatacenter(),
			expectedScyllaClusterStatus: func() scyllav1.ScyllaClusterStatus {
				status := newBasicScyllaCluster().Status
				rackStatus := status.Racks["a"]
				rackStatus.Conditions = []scyllav1.RackCondition{
					{
						Type:   "MemberDecommissioning",
						Status: "True",
					},
					{
						Type:   "MemberLeaving",
						Status: "True",
					},
				}
				status.Racks["a"] = rackStatus
				return status
			}(),
		},
		{
			name:       "replacing rack condition when one of the member services has special label",
			configMaps: nil,
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"scylla/rack":    "a",
							"scylla/replace": "foo",
						},
					},
				},
			},
			scyllaDBDatacenter: newBasicScyllaDBDatacenter(),
			expectedScyllaClusterStatus: func() scyllav1.ScyllaClusterStatus {
				status := newBasicScyllaCluster().Status
				rackStatus := status.Racks["a"]
				rackStatus.Conditions = []scyllav1.RackCondition{
					{
						Type:   "MemberReplacing",
						Status: "True",
					},
				}
				status.Racks["a"] = rackStatus
				return status
			}(),
		},
		{
			name:       "upgrading rack condition current version is different than updated one",
			configMaps: nil,
			services:   nil,
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				sdc.Status.Racks[0].UpdatedVersion = "bar"
				return sdc
			}(),
			expectedScyllaClusterStatus: func() scyllav1.ScyllaClusterStatus {
				status := newBasicScyllaCluster().Status
				rackStatus := status.Racks["a"]
				rackStatus.Conditions = []scyllav1.RackCondition{
					{
						Type:   "RackUpgrading",
						Status: "True",
					},
				}
				status.Racks["a"] = rackStatus
				return status
			}(),
		},
		{
			name:               "upgrade status is taken from upgrade context ConfigMap",
			scyllaDBDatacenter: newBasicScyllaDBDatacenter(),
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-upgrade-context", newBasicScyllaDBDatacenter().Name),
					},
					Data: map[string]string{
						"upgrade-context.json": func() string {
							uc := &internalapi.DatacenterUpgradeContext{
								State:             "a",
								FromVersion:       "b",
								ToVersion:         "c",
								SystemSnapshotTag: "d",
								DataSnapshotTag:   "e",
							}
							buf := &bytes.Buffer{}

							err := json.NewEncoder(buf).Encode(uc)
							if err != nil {
								panic(err)
							}

							return buf.String()
						}(),
					},
				},
			},
			services: nil,
			expectedScyllaClusterStatus: func() scyllav1.ScyllaClusterStatus {
				status := newBasicScyllaCluster().Status
				status.Upgrade = &scyllav1.UpgradeStatus{
					State:             "a",
					FromVersion:       "b",
					ToVersion:         "c",
					SystemSnapshotTag: "d",
					DataSnapshotTag:   "e",
				}
				return status
			}(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := migrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(tc.scyllaDBDatacenter, tc.configMaps, tc.services)
			if !equality.Semantic.DeepEqual(got, tc.expectedScyllaClusterStatus) {
				t.Errorf("expected and got status differ, diff %v", cmp.Diff(tc.expectedScyllaClusterStatus, got))
			}
		})
	}
}

func TestMigrateV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	newBasicScyllaDBDatacenterWithNoStatus := func() *scyllav1alpha1.ScyllaDBDatacenter {
		sd := newBasicScyllaDBDatacenter()
		sd.Status = scyllav1alpha1.ScyllaDBDatacenterStatus{}
		return sd
	}

	tt := []struct {
		name                       string
		scyllaCluster              *scyllav1.ScyllaCluster
		expectedUpgradeContext     *internalapi.DatacenterUpgradeContext
		expectedScyllaDBDatacenter *scyllav1alpha1.ScyllaDBDatacenter
		expectedErr                error
	}{
		{
			name:                       "valid migration with all fields except Status",
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
				metav1.SetMetaDataAnnotation(&sd.ObjectMeta, naming.TransformScyllaClusterToScyllaDBDatacenterHostNetworkingAnnotation, "true")
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
				metav1.SetMetaDataAnnotation(&sd.ObjectMeta, naming.TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation, "[\"foo=bar\",\"zoo=foo\"]\n")
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
				metav1.SetMetaDataAnnotation(&sd.ObjectMeta, naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation, "9000")
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
				metav1.SetMetaDataAnnotation(&sd.ObjectMeta, naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation, "true")
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
				metav1.SetMetaDataAnnotation(&sd.ObjectMeta, naming.TransformScyllaClusterToScyllaDBDatacenterInsecureDisableAuthorizationAnnotation, "true")
				return sd
			}(),
		},
		{
			name: "alternator insecure disable authorization propagates into annotation",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Status.Upgrade = &scyllav1.UpgradeStatus{
					State:             "PreHooks",
					FromVersion:       "from-version",
					ToVersion:         "to-version",
					SystemSnapshotTag: "system-snapshot-tag",
					DataSnapshotTag:   "data-snapshot-tag",
				}
				return sc
			}(),
			expectedScyllaDBDatacenter: newBasicScyllaDBDatacenterWithNoStatus(),
			expectedUpgradeContext: &internalapi.DatacenterUpgradeContext{
				State:             internalapi.PreHooksUpgradePhase,
				FromVersion:       "from-version",
				ToVersion:         "to-version",
				SystemSnapshotTag: "system-snapshot-tag",
				DataSnapshotTag:   "data-snapshot-tag",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSDc, gotUpgradeContext, err := MigrateV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(tc.scyllaCluster)
			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(gotSDc, tc.expectedScyllaDBDatacenter) {
				t.Errorf("expected and got scylladbdatacenter differ, diff %v", cmp.Diff(tc.expectedScyllaDBDatacenter, gotSDc))
			}
			if !equality.Semantic.DeepEqual(gotUpgradeContext, tc.expectedUpgradeContext) {
				t.Errorf("expected and got upgrade context differ, diff %v", cmp.Diff(tc.expectedUpgradeContext, gotUpgradeContext))
			}
		})
	}
}

func newBasicScyllaCluster() *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster",
			Namespace: "scylla",
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
					Version:          "scylladb-version",
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
			Name:      "simple-cluster",
			Namespace: "scylla",
			Labels: map[string]string{
				naming.GlobalScyllaDBManagerRegistrationLabel: "true",
			},
			Annotations: map[string]string{
				naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation: "scylla/simple-cluster",
			},
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
				Labels: map[string]string{
					"label": "value",
				},
				Annotations: map[string]string{
					"annotation": "value",
				},
			},
			ClusterName:    "simple-cluster",
			DatacenterName: pointer.Ptr("dc1"),
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
			ForceRedeploymentReason: pointer.Ptr("force-redeployment-reason"),
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
			DisableAutomaticOrphanedNodeReplacement: pointer.Ptr(true),
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
			CurrentVersion: "scylladb-version",
			UpdatedVersion: "scylladb-version",
			Nodes:          pointer.Ptr[int32](1),
			CurrentNodes:   pointer.Ptr[int32](2),
			UpdatedNodes:   pointer.Ptr[int32](3),
			ReadyNodes:     pointer.Ptr[int32](4),
			AvailableNodes: pointer.Ptr[int32](5),
			Racks: []scyllav1alpha1.RackStatus{
				{
					Name:           "a",
					CurrentVersion: "scylladb-version",
					UpdatedVersion: "scylladb-version",
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
			Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
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

func TestMigrateV1ScyllaClusterToV1Alpha1ScyllaDBManagerTasks(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                         string
		scyllaCluster                *scyllav1.ScyllaCluster
		expectedScyllaDBManagerTasks []*scyllav1alpha1.ScyllaDBManagerTask
		expectedErr                  error
	}{
		{
			name:                         "cluster with no tasks",
			scyllaCluster:                newBasicScyllaCluster(),
			expectedScyllaDBManagerTasks: nil,
			expectedErr:                  nil,
		},
		{
			name: "cluster with backup and repair tasks",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()

				sc.Spec.Backups = []scyllav1.BackupTaskSpec{
					newScyllaV1BackupTaskSpec(),
				}
				sc.Spec.Repairs = []scyllav1.RepairTaskSpec{
					newScyllaV1RepairTaskSpec(),
				}

				return sc
			}(),
			expectedScyllaDBManagerTasks: []*scyllav1alpha1.ScyllaDBManagerTask{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "simple-cluster-backup-task1-1a8wu",
						Namespace: "scylla",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "simple-cluster",
						},
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override":                        "task1",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-missing-owner-uid-force-adopt":        "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override":           "5m",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-start-date-override":         "now+3d2h10m",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override":           "CET",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-dc-no-validate":                "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-keyspace-no-validate":          "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-location-no-validate":          "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-rate-limit-no-validate":        "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-retention-no-validate":         "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-snapshot-parallel-no-validate": "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-upload-parallel-no-validate":   "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "simple-cluster",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
						Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
						ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
							Name: "simple-cluster",
							Kind: "ScyllaDBDatacenter",
						},
						Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
							ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
								Cron:       pointer.Ptr("0 23 * * SAT"),
								NumRetries: pointer.Ptr[int64](3),
							},
							DC:               []string{"dc1", "!otherdc*"},
							Keyspace:         []string{"keyspace", "!keyspace.table_prefix_*"},
							Location:         []string{"s3:bucket.domain", "dc:s3:otherbucket.otherdomain"},
							RateLimit:        []string{"dc1:100", "dc2:200"},
							Retention:        pointer.Ptr[int64](3),
							SnapshotParallel: []string{"dc1:1", "dc2:2"},
							UploadParallel:   []string{"dc3:3", "dc4:4"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "simple-cluster-repair-task1-1ml5z",
						Namespace: "scylla",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "simple-cluster",
						},
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override":                         "task1",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-missing-owner-uid-force-adopt":         "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override":            "7d",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-start-date-override":          "now+4d3h20m",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override":            "Europe/Warsaw",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-dc-no-validate":                 "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override":             "0.5",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-keyspace-no-validate":           "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-parallel-no-validate":           "",
							"internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-small-table-threshold-override": "1GiB",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "simple-cluster",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
						Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
						ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
							Name: "simple-cluster",
							Kind: "ScyllaDBDatacenter",
						},
						Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
							ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
								Cron:       pointer.Ptr("0 5 * * *"),
								NumRetries: pointer.Ptr[int64](1),
							},
							DC:                  []string{"dc1", "!otherdc*"},
							Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
							FailFast:            pointer.Ptr(false),
							Host:                pointer.Ptr("10.0.0.1"),
							Intensity:           nil,
							Parallel:            pointer.Ptr[int64](2),
							SmallTableThreshold: nil,
							IgnoreDownHosts:     pointer.Ptr(false),
						},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := MigrateV1ScyllaClusterToV1Alpha1ScyllaDBManagerTasks(tc.scyllaCluster)
			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(got, tc.expectedScyllaDBManagerTasks) {
				t.Errorf("expected and got ScyllaDBManagerTasks differ: %v", cmp.Diff(tc.expectedScyllaDBManagerTasks, got))
			}
		})
	}
}

func TestMigrateV1Alpha1ScyllaDBManagerTaskStatusToV1BackupTaskStatus(t *testing.T) {
	t.Parallel()

	const taskName = "task1"
	const taskID = "1856aa51-5712-4b39-a7a5-41cee77ac50f"
	statusAnnotation := strings.TrimSpace(`{
"startDate":"2025-06-12T21:44:34.694Z",
"interval":"5m",
"numRetries":3,
"cron":"{\"spec\":\"0 23 * * SAT\",\"start_date\":\"0001-01-01T00:00:00Z\"}",
"timezone":"CET",
"name":"task1",
"id":"1856aa51-5712-4b39-a7a5-41cee77ac50f",
"labels":{
	"scylla-operator.scylladb.com/managed-hash":"IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
	"scylla-operator.scylladb.com/owner-uid":"74ab78ad-686e-47f0-983f-d2aae618473c"
},
"dc": ["dc1", "!otherdc*"],
"keyspace": ["keyspace", "!keyspace.table_prefix_*"],
"location": ["s3:bucket.domain", "dc:s3:otherbucket.otherdomain"],
"rateLimit": ["dc1:100", "dc2:200"],
"retention": 3,
"snapshotParallel": ["dc1:1", "dc2:2"],
"uploadParallel": ["dc3:3", "dc4:4"]
}`)

	tests := []struct {
		name                string
		scyllaDBManagerTask *scyllav1alpha1.ScyllaDBManagerTask
		taskName            string
		expected            scyllav1.BackupTaskStatus
		expectedOK          bool
		expectedErr         error
	}{
		{
			name: "no status annotation, no degraded condition",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.BackupTaskStatus{},
			expectedOK:  false,
			expectedErr: nil,
		},
		{
			name: "no status annotation, degraded condition present",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.Status.Conditions = []metav1.Condition{
					{
						Type:               scyllav1alpha1.DegradedCondition,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: 123,
						Reason:             "TaskDegradedReason",
						Message:            "task degraded message",
					},
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.BackupTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					Name:  "task1",
					ID:    pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Error: pointer.Ptr("task degraded message"),
				},
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "status annotation present, no degraded condition",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: statusAnnotation,
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.BackupTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr("2025-06-12T21:44:34.694Z"),
						Interval:   pointer.Ptr("5m"),
						NumRetries: pointer.Ptr[int64](3),
						Cron:       pointer.Ptr(`{"spec":"0 23 * * SAT","start_date":"0001-01-01T00:00:00Z"}`),
						Timezone:   pointer.Ptr("CET"),
					},
					Name: "task1",
					ID:   pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Labels: map[string]string{
						"scylla-operator.scylladb.com/managed-hash": "IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
						"scylla-operator.scylladb.com/owner-uid":    "74ab78ad-686e-47f0-983f-d2aae618473c",
					},
					Error: nil,
				},
				DC:               []string{"dc1", "!otherdc*"},
				Keyspace:         []string{"keyspace", "!keyspace.table_prefix_*"},
				Location:         []string{"s3:bucket.domain", "dc:s3:otherbucket.otherdomain"},
				RateLimit:        []string{"dc1:100", "dc2:200"},
				Retention:        pointer.Ptr[int64](3),
				SnapshotParallel: []string{"dc1:1", "dc2:2"},
				UploadParallel:   []string{"dc3:3", "dc4:4"},
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "status annotation present, degraded condition present",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: statusAnnotation,
				}
				smt.Status.Conditions = []metav1.Condition{
					{
						Type:               scyllav1alpha1.DegradedCondition,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: 123,
						Reason:             "TaskDegradedReason",
						Message:            "task degraded message",
					},
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.BackupTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr("2025-06-12T21:44:34.694Z"),
						Interval:   pointer.Ptr("5m"),
						NumRetries: pointer.Ptr[int64](3),
						Cron:       pointer.Ptr(`{"spec":"0 23 * * SAT","start_date":"0001-01-01T00:00:00Z"}`),
						Timezone:   pointer.Ptr("CET"),
					},
					Name: "task1",
					ID:   pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Labels: map[string]string{
						"scylla-operator.scylladb.com/managed-hash": "IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
						"scylla-operator.scylladb.com/owner-uid":    "74ab78ad-686e-47f0-983f-d2aae618473c",
					},
					Error: pointer.Ptr("task degraded message"),
				},
				DC:               []string{"dc1", "!otherdc*"},
				Keyspace:         []string{"keyspace", "!keyspace.table_prefix_*"},
				Location:         []string{"s3:bucket.domain", "dc:s3:otherbucket.otherdomain"},
				RateLimit:        []string{"dc1:100", "dc2:200"},
				Retention:        pointer.Ptr[int64](3),
				SnapshotParallel: []string{"dc1:1", "dc2:2"},
				UploadParallel:   []string{"dc3:3", "dc4:4"},
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "empty status annotation",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: ``,
				}

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.BackupTaskStatus{},
			expectedOK:  false,
			expectedErr: fmt.Errorf("can't decode ScyllaDBManagerTask status annotation: %w", io.EOF),
		},
		{
			name: "invalid status annotation",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: `{"invalid":`,
				}

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.BackupTaskStatus{},
			expectedOK:  false,
			expectedErr: fmt.Errorf("can't decode ScyllaDBManagerTask status annotation: %w", io.ErrUnexpectedEOF),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotOK, err := migrateV1Alpha1ScyllaDBManagerTaskStatusToV1BackupTaskStatus(tt.scyllaDBManagerTask, tt.taskName)

			if !reflect.DeepEqual(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if gotOK != tt.expectedOK {
				t.Fatalf("expected ok %v, got %v", tt.expectedOK, gotOK)
			}

			if !equality.Semantic.DeepEqual(got, tt.expected) {
				t.Errorf("expected and got statuses differ: %v", cmp.Diff(tt.expected, got))
			}
		})
	}
}

func TestMigrateV1Alpha1ScyllaDBManagerTaskStatusToV1RepairTaskStatus(t *testing.T) {
	t.Parallel()

	const taskName = "task1"
	const taskID = "1856aa51-5712-4b39-a7a5-41cee77ac50f"
	statusAnnotation := strings.TrimSpace(`{
"startDate":"2025-06-12T21:44:34.694Z",
"interval":"5m",
"numRetries":3,
"cron":"{\"spec\":\"0 23 * * SAT\",\"start_date\":\"0001-01-01T00:00:00Z\"}",
"timezone":"CET",
"name":"task1",
"id":"1856aa51-5712-4b39-a7a5-41cee77ac50f",
"labels":{
	"scylla-operator.scylladb.com/managed-hash":"IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
	"scylla-operator.scylladb.com/owner-uid":"74ab78ad-686e-47f0-983f-d2aae618473c"
},
"dc": ["dc1", "!otherdc*"],
"keyspace": ["keyspace", "!keyspace.table_prefix_*"],
"host": "10.0.0.1",
"failFast": false,
"intensity": "0.5",
"parallel": 2,
"smallTableThreshold": "1GiB",
"ignoreDownHosts": false
}`)

	tests := []struct {
		name                string
		scyllaDBManagerTask *scyllav1alpha1.ScyllaDBManagerTask
		taskName            string
		expected            scyllav1.RepairTaskStatus
		expectedOK          bool
		expectedErr         error
	}{
		{
			name: "no status annotation, no degraded condition",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.RepairTaskStatus{},
			expectedOK:  false,
			expectedErr: nil,
		},
		{
			name: "no status annotation, degraded condition present",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.Status.Conditions = []metav1.Condition{
					{
						Type:               scyllav1alpha1.DegradedCondition,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: 123,
						Reason:             "TaskDegradedReason",
						Message:            "task degraded message",
					},
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.RepairTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					Name:  "task1",
					ID:    pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Error: pointer.Ptr("task degraded message"),
				},
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "status annotation present, no degraded condition",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: statusAnnotation,
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.RepairTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr("2025-06-12T21:44:34.694Z"),
						Interval:   pointer.Ptr("5m"),
						NumRetries: pointer.Ptr[int64](3),
						Cron:       pointer.Ptr(`{"spec":"0 23 * * SAT","start_date":"0001-01-01T00:00:00Z"}`),
						Timezone:   pointer.Ptr("CET"),
					},
					Name: "task1",
					ID:   pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Labels: map[string]string{
						"scylla-operator.scylladb.com/managed-hash": "IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
						"scylla-operator.scylladb.com/owner-uid":    "74ab78ad-686e-47f0-983f-d2aae618473c",
					},
					Error: nil,
				},
				DC:                  []string{"dc1", "!otherdc*"},
				FailFast:            pointer.Ptr(false),
				Intensity:           pointer.Ptr("0.5"),
				Parallel:            pointer.Ptr[int64](2),
				Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
				SmallTableThreshold: pointer.Ptr("1GiB"),
				Host:                pointer.Ptr("10.0.0.1"),
				IgnoreDownHosts:     pointer.Ptr(false),
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "status annotation present, degraded condition present",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: statusAnnotation,
				}
				smt.Status.Conditions = []metav1.Condition{
					{
						Type:               scyllav1alpha1.DegradedCondition,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: 123,
						Reason:             "TaskDegradedReason",
						Message:            "task degraded message",
					},
				}

				return smt
			}(),
			taskName: taskName,
			expected: scyllav1.RepairTaskStatus{
				TaskStatus: scyllav1.TaskStatus{
					SchedulerTaskStatus: scyllav1.SchedulerTaskStatus{
						StartDate:  pointer.Ptr("2025-06-12T21:44:34.694Z"),
						Interval:   pointer.Ptr("5m"),
						NumRetries: pointer.Ptr[int64](3),
						Cron:       pointer.Ptr(`{"spec":"0 23 * * SAT","start_date":"0001-01-01T00:00:00Z"}`),
						Timezone:   pointer.Ptr("CET"),
					},
					Name: "task1",
					ID:   pointer.Ptr("1856aa51-5712-4b39-a7a5-41cee77ac50f"),
					Labels: map[string]string{
						"scylla-operator.scylladb.com/managed-hash": "IaWoli79Qzq+PWK0ZbW6KQseXZpmkvH4ix9QWWrVGITcSCCntWych/2YGr+7VD69H6INyr7S+Ajjp079YqJypw==",
						"scylla-operator.scylladb.com/owner-uid":    "74ab78ad-686e-47f0-983f-d2aae618473c",
					},
					Error: pointer.Ptr("task degraded message"),
				},
				DC:                  []string{"dc1", "!otherdc*"},
				FailFast:            pointer.Ptr(false),
				Intensity:           pointer.Ptr("0.5"),
				Parallel:            pointer.Ptr[int64](2),
				Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
				SmallTableThreshold: pointer.Ptr("1GiB"),
				Host:                pointer.Ptr("10.0.0.1"),
				IgnoreDownHosts:     pointer.Ptr(false),
			},
			expectedOK:  true,
			expectedErr: nil,
		},
		{
			name: "empty status annotation",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: ``,
				}

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.RepairTaskStatus{},
			expectedOK:  false,
			expectedErr: fmt.Errorf("can't decode ScyllaDBManagerTask status annotation: %w", io.EOF),
		},
		{
			name: "invalid status annotation",
			scyllaDBManagerTask: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTask()

				smt.Status.TaskID = pointer.Ptr(taskID)
				smt.ObjectMeta.Annotations = map[string]string{
					naming.ScyllaDBManagerTaskStatusAnnotation: `{"invalid":`,
				}

				return smt
			}(),
			taskName:    taskName,
			expected:    scyllav1.RepairTaskStatus{},
			expectedOK:  false,
			expectedErr: fmt.Errorf("can't decode ScyllaDBManagerTask status annotation: %w", io.ErrUnexpectedEOF),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotOK, err := migrateV1Alpha1ScyllaDBManagerTaskStatusToV1RepairTaskStatus(tt.scyllaDBManagerTask, tt.taskName)

			if !reflect.DeepEqual(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if gotOK != tt.expectedOK {
				t.Fatalf("expected ok %v, got %v", tt.expectedOK, gotOK)
			}

			if !equality.Semantic.DeepEqual(got, tt.expected) {
				t.Errorf("expected and got statuses differ: %v", cmp.Diff(tt.expected, got))
			}
		})
	}
}

func newScyllaV1BackupTaskSpec() scyllav1.BackupTaskSpec {
	return scyllav1.BackupTaskSpec{
		TaskSpec: scyllav1.TaskSpec{
			Name: "task1",
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				StartDate:  pointer.Ptr("now+3d2h10m"),
				Interval:   pointer.Ptr("5m"),
				NumRetries: pointer.Ptr[int64](3),
				Cron:       pointer.Ptr("0 23 * * SAT"),
				Timezone:   pointer.Ptr("CET"),
			},
		},
		DC:               []string{"dc1", "!otherdc*"},
		Keyspace:         []string{"keyspace", "!keyspace.table_prefix_*"},
		Location:         []string{"s3:bucket.domain", "dc:s3:otherbucket.otherdomain"},
		RateLimit:        []string{"dc1:100", "dc2:200"},
		Retention:        3,
		SnapshotParallel: []string{"dc1:1", "dc2:2"},
		UploadParallel:   []string{"dc3:3", "dc4:4"},
	}
}

func newScyllaV1RepairTaskSpec() scyllav1.RepairTaskSpec {
	return scyllav1.RepairTaskSpec{
		TaskSpec: scyllav1.TaskSpec{
			Name: "task1",
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				StartDate:  pointer.Ptr("now+4d3h20m"),
				Interval:   pointer.Ptr("7d"),
				NumRetries: pointer.Ptr[int64](1),
				Cron:       pointer.Ptr("0 5 * * *"),
				Timezone:   pointer.Ptr("Europe/Warsaw"),
			},
		},
		DC:                  []string{"dc1", "!otherdc*"},
		FailFast:            false,
		Intensity:           "0.5",
		Parallel:            2,
		Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
		SmallTableThreshold: "1GiB",
		Host:                pointer.Ptr("10.0.0.1"),
		IgnoreDownHosts:     pointer.Ptr(false),
	}
}

func newBackupScyllaDBManagerTask() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-backup-task1-1a8wu",
			Namespace: "scylla",
			Labels: map[string]string{
				"app":                          "scylla",
				"app.kubernetes.io/managed-by": "scylla-operator",
				"app.kubernetes.io/name":       "scylla",
				"scylla/cluster":               "simple-cluster",
			},
			Annotations: map[string]string{
				naming.ScyllaDBManagerTaskNameOverrideAnnotation:                     "task1",
				naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation:         "5m",
				naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation:        "now+3d2h10m",
				naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation:         "CET",
				naming.ScyllaDBManagerTaskBackupDCNoValidateAnnotation:               "",
				naming.ScyllaDBManagerTaskBackupKeyspaceNoValidateAnnotation:         "",
				naming.ScyllaDBManagerTaskBackupLocationNoValidateAnnotation:         "",
				naming.ScyllaDBManagerTaskBackupRateLimitNoValidateAnnotation:        "",
				naming.ScyllaDBManagerTaskBackupRetentionNoValidateAnnotation:        "",
				naming.ScyllaDBManagerTaskBackupSnapshotParallelNoValidateAnnotation: "",
				naming.ScyllaDBManagerTaskBackupUploadParallelNoValidateAnnotation:   "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "scylla.scylladb.com/v1",
					Kind:               "ScyllaCluster",
					Name:               "simple-cluster",
					Controller:         pointer.Ptr(true),
					BlockOwnerDeletion: pointer.Ptr(true),
				},
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Name: "simple-cluster",
				Kind: "ScyllaDBDatacenter",
			},
			Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 23 * * SAT"),
					NumRetries: pointer.Ptr[int64](3),
				},
				DC:               []string{"dc1", "!otherdc*"},
				Keyspace:         []string{"keyspace", "!keyspace.table_prefix_*"},
				Location:         []string{"s3:bucket.domain", "dc:s3:otherbucket.otherdomain"},
				RateLimit:        []string{"dc1:100", "dc2:200"},
				Retention:        pointer.Ptr[int64](3),
				SnapshotParallel: []string{"dc1:1", "dc2:2"},
				UploadParallel:   []string{"dc3:3", "dc4:4"},
			},
		},
	}
}

func newRepairScyllaDBManagerTask() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-cluster-repair-task1-1ml5z",
			Namespace: "scylla",
			Labels: map[string]string{
				"app":                          "scylla",
				"app.kubernetes.io/managed-by": "scylla-operator",
				"app.kubernetes.io/name":       "scylla",
				"scylla/cluster":               "simple-cluster",
			},
			Annotations: map[string]string{
				naming.ScyllaDBManagerTaskNameOverrideAnnotation:                      "task1",
				naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation:          "7d",
				naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation:         "now+4d3h20m",
				naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation:          "Europe/Warsaw",
				naming.ScyllaDBManagerTaskRepairDCNoValidateAnnotation:                "",
				naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation:           "0.5",
				naming.ScyllaDBManagerTaskRepairKeyspaceNoValidateAnnotation:          "",
				naming.ScyllaDBManagerTaskRepairParallelNoValidateAnnotation:          "",
				naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation: "1GiB",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "scylla.scylladb.com/v1",
					Kind:               "ScyllaCluster",
					Name:               "simple-cluster",
					Controller:         pointer.Ptr(true),
					BlockOwnerDeletion: pointer.Ptr(true),
				},
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Name: "simple-cluster",
				Kind: "ScyllaDBDatacenter",
			},
			Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 5 * * *"),
					NumRetries: pointer.Ptr[int64](1),
				},
				DC:                  []string{"dc1", "!otherdc*"},
				Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
				FailFast:            pointer.Ptr(false),
				Host:                pointer.Ptr("10.0.0.1"),
				IgnoreDownHosts:     pointer.Ptr(false),
				Intensity:           nil,
				Parallel:            pointer.Ptr[int64](2),
				SmallTableThreshold: nil,
			},
		},
	}
}
