// Copyright (C) 2025 ScyllaDB

package nodetune

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_makeJobsForNode(t *testing.T) {
	t.Parallel()

	const (
		testNamespace     = "scylla-operator-node-tuning"
		testNodeName      = "test-node"
		testNodeUID       = types.UID("test-node-uid")
		testScyllaImage   = "docker.io/scylladb/scylla:2025.1.5"
		testOperatorImage = "docker.io/scylladb/scylla-operator:latest"
	)

	getTestControllerRef := func() *metav1.OwnerReference {
		return &metav1.OwnerReference{
			APIVersion:         "apps/v1",
			Kind:               "DaemonSet",
			Name:               "test-node-setup-daemonset",
			UID:                "test-node-setup-daemonset-uid",
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
		}
	}

	testSelfPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-self-pod",
			Namespace: testNamespace,
			UID:       types.UID("test-self-pod-uid"),
			OwnerReferences: []metav1.OwnerReference{
				*getTestControllerRef(),
			},
		},
	}

	getTestNodeConfig := func() *scyllav1alpha1.NodeConfig {
		return &scyllav1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: scyllav1alpha1.NodeConfigSpec{
				Placement: scyllav1alpha1.NodeConfigPlacement{
					NodeSelector: map[string]string{"scylla.scylladb.com/node-type": "scylla"},
				},
				DisableOptimizations: false,
				LocalDiskSetup:       nil,
				Sysctls: []corev1.Sysctl{
					{
						Name:  "fs.aio-max-nr",
						Value: "30000000",
					},
					{
						Name:  "fs.file-max",
						Value: "9223372036854775807",
					},
					{
						Name:  "fs.nr_open",
						Value: "1073741816",
					},
					{
						Name:  "fs.inotify.max_user_instances",
						Value: "1200",
					},
					{
						Name:  "vm.swappiness",
						Value: "1",
					},
					{
						Name:  "vm.vfs_cache_pressure",
						Value: "2000",
					},
				},
			},
		}
	}

	testSysctlConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sysctl-rh5ie",
			Namespace: "scylla-operator-node-tuning",
		},
		Data: map[string]string{
			"99-override_scylla-operator_scylladb_com.conf": strings.TrimPrefix(`
fs.aio-max-nr = 30000000
fs.file-max = 9223372036854775807
fs.nr_open = 1073741816
fs.inotify.max_user_instances = 1200
vm.swappiness = 1
vm.vfs_cache_pressure = 2000
`, "\n"),
		},
	}

	getTestPerftuneJob := func() *batchv1.Job {
		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "perftune-node-test-node-uid",
				Namespace: "scylla-operator-node-tuning",
				Annotations: map[string]string{
					"scylla-operator.scylladb.com/node-config-job-for-node": "test-node",
				},
				Labels: map[string]string{
					"scylla-operator.scylladb.com/node-config-job-for-node-uid": "test-node-uid",
					"scylla-operator.scylladb.com/node-config-job-type":         "NodePerftune",
					"scylla-operator.scylladb.com/node-config-name":             "test",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "DaemonSet",
						Name:               "test-node-setup-daemonset",
						UID:                "test-node-setup-daemonset-uid",
						Controller:         pointer.Ptr(true),
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: pointer.Ptr(int32(2147483647)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"scylla-operator.scylladb.com/node-config-job-for-node": "test-node",
						},
						Labels: map[string]string{
							"scylla-operator.scylladb.com/node-config-job-for-node-uid": "test-node-uid",
							"scylla-operator.scylladb.com/node-config-job-type":         "NodePerftune",
							"scylla-operator.scylladb.com/node-config-name":             "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "perftune",
								Image:   "docker.io/scylladb/scylla:2025.1.5",
								Command: []string{"/opt/scylladb/scripts/perftune.py"},
								Args:    []string{"--tune=system", "--tune-clock"},
								Env: []corev1.EnvVar{
									{
										Name:  "SYSTEMD_IGNORE_CHROOT",
										Value: "1",
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "host-sys-class",
										MountPath: "/sys/class",
									},
									{
										Name:      "host-sys-devices",
										MountPath: "/sys/devices",
									},
								},
								ImagePullPolicy: corev1.PullIfNotPresent,
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Ptr(true),
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "host-sys-class",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/sys/class",
										Type: pointer.Ptr(corev1.HostPathDirectory),
									},
								},
							},
							{
								Name: "host-sys-devices",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/sys/devices",
										Type: pointer.Ptr(corev1.HostPathDirectory),
									},
								},
							},
						},
						RestartPolicy:      corev1.RestartPolicyOnFailure,
						ServiceAccountName: "perftune",
						NodeName:           "test-node",
						HostNetwork:        true,
						HostPID:            true,
					},
				},
			},
		}
	}

	getTestSysctlsJob := func() *batchv1.Job {
		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sysctls-node-test-node-uid-h4ebl",
				Namespace: "scylla-operator-node-tuning",
				Annotations: map[string]string{
					"scylla-operator.scylladb.com/node-config-job-for-node": "test-node",
					"scylla-operator.scylladb.com/inputs-hash":              "jan9/49AiCtnp5zEurCeq93vAJVPIJupBHsPBRomejW9062PT+xSxJ/mMt6breAzTPmC6WlU3VI61mhX5/mgpg==",
				},
				Labels: map[string]string{
					"scylla-operator.scylladb.com/node-config-job-for-node-uid": "test-node-uid",
					"scylla-operator.scylladb.com/node-config-job-type":         "NodeSysctls",
					"scylla-operator.scylladb.com/node-config-name":             "test",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "DaemonSet",
						Name:               "test-node-setup-daemonset",
						UID:                "test-node-setup-daemonset-uid",
						Controller:         pointer.Ptr(true),
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: pointer.Ptr(int32(2147483647)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"scylla-operator.scylladb.com/node-config-job-for-node": "test-node",
							"scylla-operator.scylladb.com/inputs-hash":              "jan9/49AiCtnp5zEurCeq93vAJVPIJupBHsPBRomejW9062PT+xSxJ/mMt6breAzTPmC6WlU3VI61mhX5/mgpg==",
						},
						Labels: map[string]string{
							"scylla-operator.scylladb.com/node-config-job-for-node-uid": "test-node-uid",
							"scylla-operator.scylladb.com/node-config-job-type":         "NodeSysctls",
							"scylla-operator.scylladb.com/node-config-name":             "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "sysctls",
								Image: "docker.io/scylladb/scylla-operator:latest",
								Command: []string{
									"/usr/bin/bash",
									"-euExo",
									"pipefail",
									"-O",
									"inherit_errexit",
									"-c",
								},
								Args: []string{`
cp /var/run/configmaps/99-override_scylla-operator_scylladb_com.conf /host/etc/sysctl.d/99-override_scylla-operator_scylladb_com.conf
sysctl --load /host/etc/sysctl.d/99-override_scylla-operator_scylladb_com.conf
`},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
								ImagePullPolicy: corev1.PullIfNotPresent,
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Ptr(true),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:             "hostfs",
										MountPath:        "/host",
										MountPropagation: pointer.Ptr(corev1.MountPropagationBidirectional),
									},
									{
										Name:      "sysctl-config",
										MountPath: "/var/run/configmaps/99-override_scylla-operator_scylladb_com.conf",
										SubPath:   "99-override_scylla-operator_scylladb_com.conf",
										ReadOnly:  true,
									},
								},
							},
						},
						RestartPolicy:      corev1.RestartPolicyOnFailure,
						ServiceAccountName: "sysctls",
						NodeName:           "test-node",
						Volumes: []corev1.Volume{
							{
								Name: "hostfs",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/",
										Type: pointer.Ptr(corev1.HostPathDirectory),
									},
								},
							},
							{
								Name: "sysctl-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "test-sysctl-rh5ie",
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	tt := []struct {
		name            string
		nc              *scyllav1alpha1.NodeConfig
		controllerRef   *metav1.OwnerReference
		namespace       string
		nodeName        string
		nodeUID         types.UID
		scyllaImage     string
		operatorImage   string
		selfPod         *corev1.Pod
		sysctlConfigMap *corev1.ConfigMap
		expected        []*batchv1.Job
		expectedErr     error
	}{
		{
			name:            "all jobs",
			nc:              getTestNodeConfig(),
			controllerRef:   getTestControllerRef(),
			namespace:       testNamespace,
			nodeName:        testNodeName,
			nodeUID:         testNodeUID,
			scyllaImage:     testScyllaImage,
			operatorImage:   testOperatorImage,
			selfPod:         testSelfPod,
			sysctlConfigMap: testSysctlConfigMap,
			expected: []*batchv1.Job{
				getTestSysctlsJob(),
				getTestPerftuneJob(),
			},
			expectedErr: nil,
		},
		{
			name: "optimizations disabled",
			nc: func() *scyllav1alpha1.NodeConfig {
				nc := getTestNodeConfig()
				nc.Spec.DisableOptimizations = true
				return nc
			}(),
			controllerRef:   getTestControllerRef(),
			namespace:       testNamespace,
			nodeName:        testNodeName,
			nodeUID:         testNodeUID,
			scyllaImage:     testScyllaImage,
			operatorImage:   testOperatorImage,
			selfPod:         testSelfPod,
			sysctlConfigMap: testSysctlConfigMap,
			expected: []*batchv1.Job{
				getTestSysctlsJob(),
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeJobsForNode(t.Context(), tc.nc, tc.controllerRef, tc.namespace, tc.nodeName, tc.nodeUID, tc.scyllaImage, tc.operatorImage, tc.selfPod, tc.sysctlConfigMap)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual jobs differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
