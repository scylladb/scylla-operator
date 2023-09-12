package scyllacluster

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
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
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
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
			"scylla-operator.scylladb.com/scylla-service-type": "member",
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
			Name: "agent-api",
			Port: 10001,
		},
		{
			Name: "prometheus",
			Port: 9180,
		},
		{
			Name: "agent-prometheus",
			Port: 5090,
		},
		{
			Name: "node-exporter",
			Port: 9100,
		},
		{
			Name: "cql",
			Port: 9042,
		},
		{
			Name: "cql-ssl",
			Port: 9142,
		},
		{
			Name: "cql-shard-aware",
			Port: 19042,
		},
		{
			Name: "cql-ssl-shard-aware",
			Port: 19142,
		},
		{
			Name: "thrift",
			Port: 9160,
		},
	}

	tt := []struct {
		name            string
		scyllaCluster   *scyllav1.ScyllaCluster
		rackName        string
		svcName         string
		oldService      *corev1.Service
		jobs            map[string]*batchv1.Job
		expectedService *corev1.Service
	}{
		{
			name:          "new service",
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService:    nil,
			jobs:          nil,
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
			scyllaCluster: func() *scyllav1.ScyllaCluster {
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
			jobs:       nil,
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
			scyllaCluster: func() *scyllav1.ScyllaCluster {
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
			jobs: nil,
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
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "10.0.0.1",
					},
				},
			},
			jobs: nil,
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
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "",
					},
				},
			},
			jobs: nil,
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
			scyllaCluster: func() *scyllav1.ScyllaCluster {
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
			jobs: nil,
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
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			jobs: nil,
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
			name:          "existing service with maintenance mode label, it is not carried over into required object - #1252",
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.NodeMaintenanceLabel: "42",
					},
				},
			},
			jobs: nil,
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
			name:          "last cleaned up annotation is rewritten from current one when it's missing in existing service",
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash": "abc",
					},
				},
			},
			jobs: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   basicSVCName,
					Labels: basicSVCLabels(),
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "abc",
					},
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
			name:          "last cleaned up annotation is added when cleanup job is completed",
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService:    nil,
			jobs: map[string]*batchv1.Job{
				"cleanup-member": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
					},
					Status: batchv1.JobStatus{
						CompletionTime: pointer.Ptr(metav1.Now()),
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   basicSVCName,
					Labels: basicSVCLabels(),
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "abc",
					},
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
			name:          "existing service with decommission label carries it over into required object",
			scyllaCluster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.DecommissionedLabel: naming.LabelValueFalse,
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.DecommissionedLabel] = naming.LabelValueFalse
						return labels
					}(),
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := MemberService(tc.scyllaCluster, tc.rackName, tc.svcName, tc.oldService, tc.jobs)

			if !apiequality.Semantic.DeepEqual(got, tc.expectedService) {
				t.Errorf("expected and actual services differ: %s", cmp.Diff(tc.expectedService, got))
			}
		})
	}
}

func TestStatefulSetForRack(t *testing.T) {
	t.Logf("Running TestStatefulSetForRack with TLS feature enabled: %t", utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates))

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
						Controller:         pointer.Ptr(true),
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Ptr(int32(0)),
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
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  pointer.Ptr(int64(0)),
							RunAsGroup: pointer.Ptr(int64(0)),
						},
						Volumes: func() []corev1.Volume {
							volumes := []corev1.Volume{
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
											Optional: pointer.Ptr(true),
										},
									},
								},
								{
									Name: "scylla-agent-config-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "scylla-agent-config-secret",
											Optional:   pointer.Ptr(true),
										},
									},
								},
								{
									Name: "scylla-client-config-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "scylla-client-config-secret",
											Optional:   pointer.Ptr(true),
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
							}

							if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
								volumes = append(volumes, []corev1.Volume{
									{
										Name: "scylladb-serving-certs",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-local-serving-certs",
											},
										},
									},
									{
										Name: "scylladb-client-ca",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-local-client-ca",
											},
										},
									},
									{
										Name: "scylladb-user-admin",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-local-user-admin",
											},
										},
									},
								}...)
							}

							return volumes
						}(),
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
										Name:          "node-exporter",
										ContainerPort: 9100,
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
								Command: func() []string {
									featureGatesFlagString := "--feature-gates=AllAlpha=false,AllBeta=false"
									if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
										featureGatesFlagString += ",AutomaticTLSCertificates=true"
									} else {
										featureGatesFlagString += ",AutomaticTLSCertificates=false"
									}

									return []string{
										"/mnt/shared/scylla-operator",
										"sidecar",
										featureGatesFlagString,
										"--service-name=$(SERVICE_NAME)",
										"--cpu-count=$(CPU_COUNT)",
										"--loglevel=2",
									}
								}(),
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
								VolumeMounts: func() []corev1.VolumeMount {
									mounts := []corev1.VolumeMount{
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
									}

									if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
										mounts = append(mounts, []corev1.VolumeMount{
											{
												Name:      "scylladb-serving-certs",
												MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs",
												ReadOnly:  true,
											},
											{
												Name:      "scylladb-client-ca",
												MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca",
												ReadOnly:  true,
											},
											{
												Name:      "scylladb-user-admin",
												MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/user-admin",
												ReadOnly:  true,
											},
										}...)
									}

									return mounts
								}(),
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:  pointer.Ptr(int64(0)),
									RunAsGroup: pointer.Ptr(int64(0)),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"SYS_NICE"},
									},
								},
								StartupProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(40),
									PeriodSeconds:    int32(10),
									ProbeHandler: corev1.ProbeHandler{
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
									ProbeHandler: corev1.ProbeHandler{
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
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: intstr.FromInt(8080),
											Path: "/readyz",
										},
									},
								},
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{
											Command: []string{
												"/bin/sh", "-c", "PID=$(pgrep -x scylla);supervisorctl stop scylla; while kill -0 $PID; do sleep 1; done;",
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
						TerminationGracePeriodSeconds: pointer.Ptr(int64(900)),
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
						Partition: pointer.Ptr(int32(0)),
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

	const scyllaContainerIndex = 0

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
		{
			name: "new StatefulSet with non-empty externalSeeds in scylla container",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.ExternalSeeds = []string{"10.0.1.1", "10.0.1.2", "10.0.1.3"}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command = append(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command, "--external-seeds=10.0.1.1,10.0.1.2,10.0.1.3")

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

func TestStatefulSetForRackWithReversedTLSFeature(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(
		t,
		utilfeature.DefaultMutableFeatureGate,
		features.AutomaticTLSCertificates,
		!utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates),
	)()

	t.Run("", TestStatefulSetForRack)
}

func TestMakeIngresses(t *testing.T) {
	basicScyllaCluster := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
				Racks: []scyllav1.RackSpec{
					{
						Name: "rack",
						Storage: scyllav1.StorageSpec{
							Capacity: "1Gi",
						},
					},
				},
			},
		},
	}

	newIdentityService := func(name string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
				},
			},
		}
	}

	newMemberService := func(name, hostID string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"internal.scylla-operator.scylladb.com/host-id": hostID,
				},
				Labels: map[string]string{
					"scylla-operator.scylladb.com/scylla-service-type": "member",
				},
			},
		}
	}

	pathTypePrefix := networkingv1.PathTypePrefix

	tt := []struct {
		name              string
		cluster           *scyllav1.ScyllaCluster
		services          map[string]*corev1.Service
		expectedIngresses []*networkingv1.Ingress
	}{
		{
			name:              "no ingresses when cluster isn't exposed",
			cluster:           basicScyllaCluster,
			services:          map[string]*corev1.Service{},
			expectedIngresses: nil,
		},
		{
			name: "no ingresses when ingresses are explicitly disabled",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicScyllaCluster.DeepCopy()
				cluster.Spec.DNSDomains = []string{"public.scylladb.com", "private.scylladb.com"}
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					CQL: &scyllav1.CQLExposeOptions{
						Ingress: &scyllav1.IngressOptions{
							Disabled:         pointer.Ptr(true),
							IngressClassName: "cql-ingress-class",
							Annotations: map[string]string{
								"my-cql-custom-annotation": "my-cql-custom-annotation-value",
							},
						},
					},
				}

				return cluster
			}(),
			services:          map[string]*corev1.Service{},
			expectedIngresses: nil,
		},
		{
			name: "ingress objects are generated for every domain",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
				"node-2": newMemberService("node-2", "host-id-2"),
				"node-3": newMemberService("node-3", "host-id-3"),
			},
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicScyllaCluster.DeepCopy()
				cluster.Spec.DNSDomains = []string{"public.scylladb.com", "private.scylladb.com"}
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					CQL: &scyllav1.CQLExposeOptions{
						Ingress: &scyllav1.IngressOptions{
							IngressClassName: "cql-ingress-class",
							Annotations: map[string]string{
								"my-cql-custom-annotation": "my-cql-custom-annotation-value",
							},
						},
					},
				}

				return cluster
			}(),
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "any-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "any",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "any",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-1.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-1",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-1.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-1",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-2.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-2",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-2.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-2",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-3-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-3.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-3",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-3.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-3",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MakeIngresses(tc.cluster, tc.services)
			if !apiequality.Semantic.DeepEqual(got, tc.expectedIngresses) {
				t.Errorf("expected and actual Ingresses differ: %s", cmp.Diff(tc.expectedIngresses, got))
			}
		})
	}
}

func TestMakeJobs(t *testing.T) {
	basicScyllaCluster := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic",
			Namespace: "default",
			UID:       "the-uid",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
				Racks: []scyllav1.RackSpec{
					{
						Name: "rack",
						Storage: scyllav1.StorageSpec{
							Capacity: "1Gi",
						},
						Members: 1,
					},
				},
			},
		},
	}

	newMemberService := func(name string, annotations map[string]string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: annotations,
			},
		}
	}

	tt := []struct {
		name               string
		cluster            *scyllav1.ScyllaCluster
		services           map[string]*corev1.Service
		expectedJobs       []*batchv1.Job
		expectedConditions []metav1.Condition
	}{
		{
			name:         "progressing condition rack member service is not present",
			cluster:      basicScyllaCluster,
			services:     map[string]*corev1.Service{},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForService",
					Message: `Waiting for Service "default/basic-dc-rack-0"`,
				},
			},
		},
		{
			name:    "progressing condition when member service doesn't have current token ring hash annotation",
			cluster: basicScyllaCluster,
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{}),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForServiceState",
					Message: `Service "basic-dc-rack-0" is missing current token ring hash annotation`,
				},
			},
		},
		{
			name:    "progressing condition when member service current token ring hash annotation is empty",
			cluster: basicScyllaCluster,
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash": "",
				}),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "UnexpectedServiceState",
					Message: `Service "basic-dc-rack-0" has unexpected empty current token ring hash annotation, can't create cleanup Job`,
				},
			},
		},
		{
			name:    "progressing condition when member service doesn't have latest token ring hash annotation",
			cluster: basicScyllaCluster,
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash": "abc",
				}),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForServiceState",
					Message: `Service "basic-dc-rack-0" is missing last cleaned up token ring hash annotation`,
				},
			},
		},
		{
			name:    "progressing condition when member service last cleaned up token ring hash annotation is empty",
			cluster: basicScyllaCluster,
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "",
				}),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "UnexpectedServiceState",
					Message: `Service "basic-dc-rack-0" has unexpected empty last cleaned up token ring hash annotation, can't create cleanup Job`,
				},
			},
		},
		{
			name:    "no cleanup jobs when member service token ring hash annotations are equal",
			cluster: basicScyllaCluster,
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "abc",
				}),
			},
			expectedJobs:       nil,
			expectedConditions: nil,
		},
		{
			name:    "cleanup job when member service token ring hash annotations differ",
			cluster: basicScyllaCluster,
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=basic-dc-rack-0.default.svc",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotJobs, gotConditions := MakeJobs(tc.cluster, tc.services, "scylladb/scylla-operator:latest")
			if !apiequality.Semantic.DeepEqual(gotJobs, tc.expectedJobs) {
				t.Errorf("expected and actual Job differ: %s", cmp.Diff(tc.expectedJobs, gotJobs))
			}
			if !reflect.DeepEqual(gotConditions, tc.expectedConditions) {
				t.Fatalf("expected and actual conditions differ: %s", cmp.Diff(tc.expectedConditions, gotConditions))
			}
		})
	}
}
