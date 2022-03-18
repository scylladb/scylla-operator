package scyllacluster

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func TestMemberService(t *testing.T) {
	basicSC := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			Racks: map[string]scyllav1.RackStatus{},
		},
	}
	basicSCOwnerRefs := []metav1.OwnerReference{
		{
			APIVersion:         "scylla.scylladb.com/v1",
			Kind:               "ScyllaCluster",
			Name:               "basic",
			UID:                "the-uid",
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
		},
	}
	basicRackName := "rack"
	basicSVCName := "member"
	basicSVCSelector := map[string]string{
		"statefulset.kubernetes.io/pod-name": "member",
	}
	basicSVCLabels := func() map[string]string {
		return map[string]string{
			"app":                          "scylla",
			"app.kubernetes.io/name":       "scylla",
			"app.kubernetes.io/managed-by": "scylla-operator",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
		}
	}
	basicPorts := []corev1.ServicePort{
		{
			Name: "inter-node-communication",
			Port: 7000,
		},
		{
			Name: "ssl-inter-node-communication",
			Port: 7001,
		},
		{
			Name: "jmx-monitoring",
			Port: 7199,
		},
		{
			Name: "cql",
			Port: 9042,
		},
		{
			Name: "cql-ssl",
			Port: 9142,
		}, {
			Name: "agent-api",
			Port: 10001,
		},
		{
			Name: "thrift",
			Port: 9160,
		},
	}

	tt := []struct {
		name            string
		scyllaCLuster   *scyllav1.ScyllaCluster
		rackName        string
		svcName         string
		oldService      *corev1.Service
		expectedService *corev1.Service
	}{
		{
			name:          "new service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService:    nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "new service with saved IP",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "new service with saved IP and existing replace address",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName: basicRackName,
			svcName:  basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "10.0.0.1",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "new service with unsaved IP and existing replace address",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "10.0.0.1",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "existing initial service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = ""
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "existing initial service with IP",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName: basicRackName,
			svcName:  basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = ""
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "existing service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := MemberService(tc.scyllaCLuster, tc.rackName, tc.svcName, tc.oldService)

			if !apiequality.Semantic.DeepEqual(got, tc.expectedService) {
				t.Errorf("expected and actual services differ: %s", cmp.Diff(tc.expectedService, got))
			}
		})
	}
}

func TestStatefulSetForRack(t *testing.T) {
	newBasicRack := func() scyllav1.RackSpec {
		return scyllav1.RackSpec{
			Name: "rack",
			Storage: scyllav1.StorageSpec{
				Capacity: "1Gi",
			},
		}
	}

	newBasicScyllaCluster := func() *scyllav1.ScyllaCluster {
		return &scyllav1.ScyllaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "basic",
				UID:  "the-uid",
			},
			Spec: scyllav1.ScyllaClusterSpec{
				Datacenter: scyllav1.DatacenterSpec{
					Name: "dc",
					Racks: []scyllav1.RackSpec{
						newBasicRack(),
					},
				},
			},
			Status: scyllav1.ScyllaClusterStatus{
				Racks: map[string]scyllav1.RackStatus{},
			},
		}
	}

	newBasicStatefulSetLabels := func() map[string]string {
		return map[string]string{
			"app":                          "scylla",
			"app.kubernetes.io/managed-by": "scylla-operator",
			"app.kubernetes.io/name":       "scylla",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
		}
	}

	newBasicStatefulSetLabelsWithVersion := func() map[string]string {
		m := newBasicStatefulSetLabels()
		m["scylla/scylla-version"] = ""
		return m
	}

	newBasicStatefulSet := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "basic-dc-rack",
				Labels:      newBasicStatefulSetLabelsWithVersion(),
				Annotations: nil,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						UID:                "the-uid",
						Controller:         pointer.BoolPtr(true),
						BlockOwnerDeletion: pointer.BoolPtr(true),
					},
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Int32Ptr(0),
				Selector: &metav1.LabelSelector{
					MatchLabels: newBasicStatefulSetLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: newBasicStatefulSetLabelsWithVersion(),
						Annotations: map[string]string{
							"prometheus.io/port":   "9180",
							"prometheus.io/scrape": "true",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "shared",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "scylla-config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "scylla-config",
										},
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "scylla-agent-config-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "scylla-agent-config-secret",
										Optional:   pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "scylla-client-config-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "scylla-client-config-secret",
										Optional:   pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "scylla-agent-auth-token-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "basic-auth-token",
									},
								},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name:            "sidecar-injection",
								ImagePullPolicy: "IfNotPresent",
								Image:           "scylladb/scylla-operator:latest",
								Command: []string{
									"/bin/sh",
									"-c",
									"cp -a /usr/bin/scylla-operator /mnt/shared",
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "shared",
										MountPath: "/mnt/shared",
										ReadOnly:  false,
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "scylla",
								Image:           ":",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Ports: []corev1.ContainerPort{
									{
										Name:          "intra-node",
										ContainerPort: 7000,
									},
									{
										Name:          "tls-intra-node",
										ContainerPort: 7001,
									},
									{
										Name:          "jmx",
										ContainerPort: 7199,
									},
									{
										Name:          "prometheus",
										ContainerPort: 9180,
									},
									{
										Name:          "cql",
										ContainerPort: 9042,
									},
									{
										Name:          "cql-ssl",
										ContainerPort: 9142,
									},
									{
										Name:          "thrift",
										ContainerPort: 9160,
									},
								},
								Command: []string{
									"/mnt/shared/scylla-operator",
									"sidecar",
									"--secret-name=basic-auth-token",
									"--service-name=$(SERVICE_NAME)",
									"--cpu-count=$(CPU_COUNT)",
									"--loglevel=2",
								},
								Env: []corev1.EnvVar{
									{
										Name: "SERVICE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.name",
											},
										},
									},
									{
										Name: "CPU_COUNT",
										ValueFrom: &corev1.EnvVarSource{
											ResourceFieldRef: &corev1.ResourceFieldSelector{
												ContainerName: "scylla",
												Resource:      "limits.cpu",
												Divisor:       resource.MustParse("1"),
											},
										},
									},
								},
								Resources: newBasicRack().Resources,
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "data",
										MountPath: "/var/lib/scylla",
									},
									{
										Name:      "shared",
										MountPath: "/mnt/shared",
										ReadOnly:  true,
									},
									{
										Name:      "scylla-config-volume",
										MountPath: "/mnt/scylla-config",
										ReadOnly:  true,
									},
									{
										Name:      "scylla-client-config-volume",
										MountPath: "/mnt/scylla-client-config",
										ReadOnly:  true,
									},
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"SYS_NICE"},
									},
								},
								StartupProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(40),
									PeriodSeconds:    int32(10),
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: intstr.FromInt(8080),
											Path: "/healthz",
										},
									},
								},
								LivenessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(10),
									FailureThreshold: int32(12),
									PeriodSeconds:    int32(10),
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: intstr.FromInt(8080),
											Path: "/healthz",
										},
									},
								},
								ReadinessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(1),
									PeriodSeconds:    int32(10),
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: intstr.FromInt(8080),
											Path: "/readyz",
										},
									},
								},
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.Handler{
										Exec: &corev1.ExecAction{
											Command: []string{
												"/bin/sh", "-c", "PID=$(pgrep -x scylla);supervisorctl stop scylla-server; while kill -0 $PID; do sleep 1; done;",
											},
										},
									},
								},
							},
							{
								Name:            "scylla-manager-agent",
								Image:           ":",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"-c",
									"/etc/scylla-manager-agent/scylla-manager-agent.yaml",
									"-c",
									"/mnt/scylla-agent-config/scylla-manager-agent.yaml",
									"-c",
									"/mnt/scylla-agent-config/auth-token.yaml",
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "agent-rest-api",
										ContainerPort: 10001,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "data",
										MountPath: "/var/lib/scylla",
									},
									{
										Name:      "scylla-agent-config-volume",
										MountPath: "/mnt/scylla-agent-config/scylla-manager-agent.yaml",
										SubPath:   "scylla-manager-agent.yaml",
										ReadOnly:  true,
									},
									{
										Name:      "scylla-agent-auth-token-volume",
										MountPath: "/mnt/scylla-agent-config/auth-token.yaml",
										SubPath:   "auth-token.yaml",
										ReadOnly:  true,
									},
								},
								Resources: newBasicRack().Resources,
							},
						},
						DNSPolicy:                     "ClusterFirstWithHostNet",
						ServiceAccountName:            "basic-member",
						Affinity:                      &corev1.Affinity{},
						TerminationGracePeriodSeconds: pointer.Int64Ptr(900),
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "data",
							Labels: newBasicStatefulSetLabels(),
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: newBasicRack().Storage.StorageClassName,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse(newBasicRack().Storage.Capacity),
								},
							},
						},
					},
				},
				ServiceName:         "basic-client",
				PodManagementPolicy: appsv1.OrderedReadyPodManagement,
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
						Partition: pointer.Int32Ptr(0),
					},
				},
			},
		}
	}

	newNodeAffinity := func() *corev1.NodeAffinity {
		return &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "key",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"value"},
							},
						},
					},
				},
			},
		}
	}

	newPodAffinity := func() *corev1.PodAffinity {
		return &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		}
	}

	newPodAntiAffinity := func() *corev1.PodAntiAffinity {
		return &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		}
	}

	tt := []struct {
		name                string
		rack                scyllav1.RackSpec
		scyllaCluster       *scyllav1.ScyllaCluster
		existingStatefulSet *appsv1.StatefulSet
		expectedStatefulSet *appsv1.StatefulSet
		expectedError       error
	}{
		{
			name:                "new StatefulSet",
			rack:                newBasicRack(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: newBasicStatefulSet(),
			expectedError:       nil,
		},
		{
			name: "error for invalid Rack storage",
			rack: func() scyllav1.RackSpec {
				r := newBasicRack()
				r.Storage = scyllav1.StorageSpec{
					Capacity: "",
				}
				return r
			}(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: nil,
			expectedError:       fmt.Errorf(`cannot parse "": %v`, resource.ErrFormatWrong),
		},
		{
			name: "new StatefulSet with non-nil Tolerations",
			rack: func() scyllav1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1.PlacementSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "key",
							Operator: corev1.TolerationOpEqual,
							Value:    "value",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				return r
			}(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Tolerations = []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				}
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil NodeAffinity",
			rack: func() scyllav1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1.PlacementSpec{
					NodeAffinity: newNodeAffinity(),
				}
				return r
			}(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.NodeAffinity = newNodeAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil PodAffinity",
			rack: func() scyllav1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1.PlacementSpec{
					PodAffinity: newPodAffinity(),
				}
				return r
			}(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.PodAffinity = newPodAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil PodAntiAffinity",
			rack: func() scyllav1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1.PlacementSpec{
					PodAntiAffinity: newPodAntiAffinity(),
				}
				return r
			}(),
			scyllaCluster:       newBasicScyllaCluster(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.PodAntiAffinity = newPodAntiAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil ImagePullSecrets",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{
						Name: "basic-secrets",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{
						Name: "basic-secrets",
					},
				}
				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil ForceRedeploymentReason",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.ForceRedeploymentReason = "reason"
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Annotations[naming.ForceRedeploymentReasonAnnotation] = "reason"
				return sts
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := StatefulSetForRack(tc.rack, tc.scyllaCluster, tc.existingStatefulSet, "scylladb/scylla-operator:latest")

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected and actual errors differ: %s",
					cmp.Diff(tc.expectedError, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedStatefulSet) {
				t.Errorf("expected and actual StatefulSets differ: %s",
					cmp.Diff(tc.expectedStatefulSet, got))
			}
		})
	}
}
