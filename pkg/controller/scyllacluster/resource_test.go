package scyllacluster

import (
	"fmt"
	"reflect"
	"strings"
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
			Labels: map[string]string{
				"default-sc-label": "foo",
			},
			Annotations: map[string]string{
				"default-sc-annotation": "bar",
			},
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
			"default-sc-label":             "foo",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
			"scylla-operator.scylladb.com/scylla-service-type": "member",
		}
	}
	basicSVCAnnotations := func() map[string]string {
		return map[string]string{
			"default-sc-annotation": "bar",
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
					Annotations:     basicSVCAnnotations(),
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
		// This behaviour is based on the fact the we merge labels on apply.
		// TODO: to be addressed with https://github.com/scylladb/scylla-operator/issues/1440.
		{
			name:          "new service with unsaved IP and existing replace label",
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
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name:          "existing initial service with IP",
			scyllaCluster: basicSC.DeepCopy(),
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
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
					Annotations:     basicSVCAnnotations(),
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
					Annotations:     basicSVCAnnotations(),
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
			}, jobs: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   basicSVCName,
					Labels: basicSVCLabels(),
					Annotations: func() map[string]string {
						res := basicSVCAnnotations()
						res["internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash"] = "abc"
						return res
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
					Annotations: func() map[string]string {
						res := basicSVCAnnotations()
						res["internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash"] = "abc"
						return res
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
			name: "Service properties are taken from ExposeOptions.NodeService",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"foo": "bar",
							},
							Labels: map[string]string{
								"user-label": "user-label-value",
							},
						},
						Type:                          scyllav1.NodeServiceTypeLoadBalancer,
						ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyLocal),
						AllocateLoadBalancerNodePorts: pointer.Ptr(true),
						LoadBalancerClass:             pointer.Ptr("my-lb-class"),
						InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
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
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"user-label":                   "user-label-value",
						"scylla/cluster":               "basic",
						"scylla/datacenter":            "dc",
						"scylla/rack":                  "rack",
						"scylla-operator.scylladb.com/scylla-service-type": "member",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                          corev1.ServiceTypeLoadBalancer,
					Selector:                      basicSVCSelector,
					PublishNotReadyAddresses:      true,
					Ports:                         basicPorts,
					ExternalTrafficPolicy:         corev1.ServiceExternalTrafficPolicyLocal,
					AllocateLoadBalancerNodePorts: pointer.Ptr(true),
					LoadBalancerClass:             pointer.Ptr("my-lb-class"),
					InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
		},
		{
			name: "headless service type in node service template",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeHeadless,
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
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
					ClusterIP:                corev1.ClusterIPNone,
				},
			},
		},
		{
			name: "ClusterIP service type in node service template",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeClusterIP,
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
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name: "LoadBalancer service type in node service template",
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeLoadBalancer,
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
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeLoadBalancer,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MemberService(tc.scyllaCluster, tc.rackName, tc.svcName, tc.oldService, tc.jobs)
			if err != nil {
				t.Fatal(err)
			}

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
			Storage: scyllav1.Storage{
				Capacity: "1Gi",
			},
		}
	}

	newBasicScyllaCluster := func() *scyllav1.ScyllaCluster {
		return &scyllav1.ScyllaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "basic",
				UID:  "the-uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
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

	newBasicStatefulSetLabels := func(ordinal int) map[string]string {
		return map[string]string{
			"app":                          "scylla",
			"app.kubernetes.io/managed-by": "scylla-operator",
			"app.kubernetes.io/name":       "scylla",
			"default-sc-label":             "foo",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
			"scylla/scylla-version":        "",
			"scylla/rack-ordinal":          fmt.Sprintf("%d", ordinal),
		}
	}

	newBasicStatefulSet := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "basic-dc-rack",
				Labels: newBasicStatefulSetLabels(0),
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
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
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Ptr(int32(0)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "basic",
						"scylla/datacenter":            "dc",
						"scylla/rack":                  "rack",
					},
				},
				MinReadySeconds: 5,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: newBasicStatefulSetLabels(0),
						Annotations: map[string]string{
							"default-sc-annotation":                    "bar",
							"scylla-operator.scylladb.com/inputs-hash": "",
							"prometheus.io/port":                       "9180",
							"prometheus.io/scrape":                     "true",
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
									Name: "scylladb-managed-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "basic-managed-config",
											},
											Optional: pointer.Ptr(false),
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
										Name:          "cql",
										ContainerPort: 9042,
									},
									{
										Name:          "cql-ssl",
										ContainerPort: 9142,
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
										"--nodes-broadcast-address-type=ServiceClusterIP",
										"--clients-broadcast-address-type=ServiceClusterIP",
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
											Name:      "scylladb-managed-config",
											MountPath: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/managed-config",
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
												"/usr/bin/bash",
												"-euExo",
												"pipefail",
												"-O",
												"inherit_errexit",
												"-c",
												"nodetool drain & sleep 15 & wait",
											},
										},
									},
								},
							},
							{
								Name:            "scylladb-api-status-probe",
								Image:           "scylladb/scylla-operator:latest",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/usr/bin/scylla-operator",
									"serve-probes",
									"scylladb-api-status",
									"--port=8080",
									"--service-name=$(SERVICE_NAME)",
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
								},
								ReadinessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(1),
									PeriodSeconds:    int32(5),
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: intstr.FromInt32(naming.ScyllaDBAPIStatusProbePort),
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
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
							Name: "data",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"app.kubernetes.io/name":       "scylla",
								"scylla/cluster":               "basic",
								"scylla/datacenter":            "dc",
								"scylla/rack":                  "rack",
								"default-sc-label":             "foo",
							},
							Annotations: map[string]string{
								"default-sc-annotation": "bar",
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: newBasicRack().Storage.StorageClassName,
							Resources: corev1.VolumeResourceRequirements{
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
				r.Storage = scyllav1.Storage{
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
		{
			name: "new StatefulSet with custom pod metadata uses the new values and doesn't inherit from the ScyllaCluster",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.PodMetadata = &scyllav1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"custom-pod-annotation": "custom-pod-annotation-value",
					},
					Labels: map[string]string{
						"custom-pod-label": "custom-pod-label-value",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.ObjectMeta.Annotations = map[string]string{
					"custom-pod-annotation":                    "custom-pod-annotation-value",
					"prometheus.io/port":                       "9180",
					"prometheus.io/scrape":                     "true",
					"scylla-operator.scylladb.com/inputs-hash": "",
				}
				sts.Spec.Template.ObjectMeta.Labels = map[string]string{
					"app":                          "scylla",
					"app.kubernetes.io/managed-by": "scylla-operator",
					"app.kubernetes.io/name":       "scylla",
					"custom-pod-label":             "custom-pod-label-value",
					"scylla/cluster":               "basic",
					"scylla/datacenter":            "dc",
					"scylla/rack":                  "rack",
					"scylla/rack-ordinal":          "0",
					"scylla/scylla-version":        "",
				}

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with custom minReadySeconds",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.MinReadySeconds = pointer.Ptr(int32(1234))
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.MinReadySeconds = 1234

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with custom readiness gates",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "my-custom-pod-readiness-gate-1",
					},
					{
						ConditionType: "my-custom-pod-readiness-gate-2",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "my-custom-pod-readiness-gate-1",
					},
					{
						ConditionType: "my-custom-pod-readiness-gate-2",
					},
				}

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with default Alternator enabled",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Alternator = &scyllav1.AlternatorSpec{}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				tmplSpec := &sts.Spec.Template.Spec
				scylladbContainer := &tmplSpec.Containers[scyllaContainerIndex]

				tmplSpec.Volumes = append(tmplSpec.Volumes, corev1.Volume{
					Name: "scylladb-alternator-serving-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "basic-alternator-local-serving-certs",
							Optional:   pointer.Ptr(false),
						},
					},
				})

				scylladbContainer.VolumeMounts = append(scylladbContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "scylladb-alternator-serving-certs",
					ReadOnly:  true,
					MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs",
				})
				scylladbContainer.Ports = append(
					scylladbContainer.Ports,
					corev1.ContainerPort{
						Name:          "alternator-tls",
						ContainerPort: 8043,
					},
				)

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with default Alternator enabled and disabled http",
			rack: newBasicRack(),
			scyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := newBasicScyllaCluster()
				sc.Spec.Alternator = &scyllav1.AlternatorSpec{
					InsecureEnableHTTP: pointer.Ptr(false),
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				tmplSpec := &sts.Spec.Template.Spec
				scylladbContainer := &tmplSpec.Containers[scyllaContainerIndex]

				tmplSpec.Volumes = append(tmplSpec.Volumes, corev1.Volume{
					Name: "scylladb-alternator-serving-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "basic-alternator-local-serving-certs",
							Optional:   pointer.Ptr(false),
						},
					},
				})

				scylladbContainer.VolumeMounts = append(scylladbContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "scylladb-alternator-serving-certs",
					ReadOnly:  true,
					MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs",
				})
				scylladbContainer.Ports = append(
					scylladbContainer.Ports,
					corev1.ContainerPort{
						Name:          "alternator-tls",
						ContainerPort: 8043,
					},
				)

				return sts
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := StatefulSetForRack(tc.rack, tc.scyllaCluster, tc.existingStatefulSet, "scylladb/scylla-operator:latest", 0, "")

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
			Annotations: map[string]string{
				"default-sc-annotation": "bar",
			},
			Labels: map[string]string{
				"default-sc-label": "foo",
			},
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
				Racks: []scyllav1.RackSpec{
					{
						Name: "rack",
						Storage: scyllav1.Storage{
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
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
				Labels: map[string]string{
					"default-sc-label":            "foo",
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
					"default-sc-annotation":                         "bar",
					"internal.scylla-operator.scylladb.com/host-id": hostID,
				},
				Labels: map[string]string{
					"default-sc-label": "foo",
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
							ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
								Annotations: map[string]string{
									"my-cql-custom-annotation": "my-cql-custom-annotation-value",
								},
								Labels: map[string]string{
									"my-cql-custom-label": "my-cql-custom-label-value",
								},
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
							ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
								Annotations: map[string]string{
									"my-cql-custom-annotation": "my-cql-custom-annotation-value",
								},
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
							"default-sc-label":             "foo",
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
							"default-sc-label":             "foo",
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
							"default-sc-label":             "foo",
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
							"default-sc-label":             "foo",
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
		{
			name: "ingress objects inherit scyllacluster labels and annotations if none are specified",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
			},
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicScyllaCluster.DeepCopy()
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					CQL: &scyllav1.CQLExposeOptions{
						Ingress: &scyllav1.IngressOptions{
							IngressClassName: "cql-ingress-class",
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
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
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
						Rules:            []networkingv1.IngressRule{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
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
						Rules:            []networkingv1.IngressRule{},
					},
				},
			},
		},
	}

	for _, tc := range tt {
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
			Labels: map[string]string{
				"default-sc-label": "foo",
			},
			Annotations: map[string]string{
				"default-sc-annotation": "bar",
			},
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
				Racks: []scyllav1.RackSpec{
					{
						Name: "rack",
						Storage: scyllav1.Storage{
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
				Name: name,
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
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
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
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
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
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
		{
			name: "cleanup job has the same placement requirements as ScyllaCluster",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := basicScyllaCluster.DeepCopy()
				cluster.Spec.Datacenter.Racks[0].Placement = &scyllav1.PlacementSpec{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "topology.kubernetes.io/zone",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zone-1", "zone-2"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/name": "scylla",
										"scylla/rack":            "rack",
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "role",
							Operator: corev1.TolerationOpEqual,
							Value:    "scylla-clusters",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				cluster.Spec.Datacenter.Racks = append(cluster.Spec.Datacenter.Racks,
					scyllav1.RackSpec{
						Name: "rack-2",
						Storage: scyllav1.Storage{
							Capacity: "1Gi",
						},
						Members: 1,
						Placement: &scyllav1.PlacementSpec{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "topology.kubernetes.io/zone",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"zone-3", "zone-4"},
												},
											},
										},
									},
								},
							},
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app.kubernetes.io/name": "scylla",
												"scylla/rack":            "rack-2",
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"scylla-operator.scylladb.com/node-job-type": "Cleanup",
												"scylla/rack-ordinal":                        "0",
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
							Tolerations: []corev1.Toleration{
								{
									Key:      "role",
									Operator: corev1.TolerationOpEqual,
									Value:    "scylla-clusters-rack-2",
									Effect:   corev1.TaintEffectNoSchedule,
								},
							},
						},
					},
				)

				return cluster
			}(),
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}),
					"basic-dc-rack-2-0": newMemberService("basic-dc-rack-2-0", map[string]string{
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
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
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
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Tolerations: []corev1.Toleration{
									{
										Key:      "role",
										Operator: corev1.TolerationOpEqual,
										Value:    "scylla-clusters",
										Effect:   corev1.TaintEffectNoSchedule,
									},
								},
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{
														{
															Key:      "topology.kubernetes.io/zone",
															Operator: corev1.NodeSelectorOpIn,
															Values:   []string{"zone-1", "zone-2"},
														},
													},
												},
											},
										},
									},
									PodAffinity: &corev1.PodAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"app.kubernetes.io/name": "scylla",
														"scylla/rack":            "rack",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"scylla-operator.scylladb.com/node-job-type": "Cleanup",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-2-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-2-0",
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
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-2-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Tolerations: []corev1.Toleration{
									{
										Key:      "role",
										Operator: corev1.TolerationOpEqual,
										Value:    "scylla-clusters-rack-2",
										Effect:   corev1.TaintEffectNoSchedule,
									},
								},
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{
														{
															Key:      "topology.kubernetes.io/zone",
															Operator: corev1.NodeSelectorOpIn,
															Values:   []string{"zone-3", "zone-4"},
														},
													},
												},
											},
										},
									},
									PodAffinity: &corev1.PodAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"app.kubernetes.io/name": "scylla",
														"scylla/rack":            "rack-2",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"scylla-operator.scylladb.com/node-job-type": "Cleanup",
														"scylla/rack-ordinal":                        "0",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=basic-dc-rack-2-0.default.svc",
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

	for _, tc := range tt {
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

func Test_MakeManagedScyllaDBConfig(t *testing.T) {
	tt := []struct {
		name                 string
		sc                   *scyllav1.ScyllaCluster
		enableTLSFeatureGate bool
		expectedCM           *corev1.ConfigMap
		expectedErr          error
	}{
		{
			name: "no TLS config when the feature is disabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
			},
			enableTLSFeatureGate: false,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "TLS config present when the feature is enabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup on TLS-only with authorization by default",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						InsecureEnableHTTP: nil,
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup on TLS-only when insecure port is disabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						InsecureEnableHTTP: pointer.Ptr(false),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup both on TLS and insecure when insecure port is enabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						InsecureEnableHTTP: pointer.Ptr(true),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_port: 8000
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when manual port is specified for backwards compatibility",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						Port: 42,
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup with authorization when manual port is specified and authorization is enabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						Port:                         42,
						InsecureDisableAuthorization: pointer.Ptr(false),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when it's disabled and using manual port",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						Port:                         42,
						InsecureDisableAuthorization: pointer.Ptr(true),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup with authorization when manual port is specified and authorization is enabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						Port:                         42,
						InsecureDisableAuthorization: pointer.Ptr(false),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when it's disabled",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
				},
				Spec: scyllav1.ScyllaClusterSpec{
					Alternator: &scyllav1.AlternatorSpec{
						InsecureDisableAuthorization: pointer.Ptr(true),
					},
				},
			},
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"scylla/cluster": "foo",
						"user-label":     "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo"
rpc_address: "0.0.0.0"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca/tls.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer featuregatetesting.SetFeatureGateDuringTest(
				t,
				utilfeature.DefaultMutableFeatureGate,
				features.AutomaticTLSCertificates,
				tc.enableTLSFeatureGate,
			)()

			got, err := MakeManagedScyllaDBConfig(tc.sc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedCM) {
				t.Errorf("expected and actual configmaps differ:\n%s", cmp.Diff(tc.expectedCM, got))
			}
		})
	}
}
