// Copyright (C) 2021 ScyllaDB

package resource

import (
	"fmt"
	"os"
	"path"
	"strconv"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func ScyllaOperatorNodeTuningNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func DefaultScyllaNodeConfig() *scyllav1alpha1.ScyllaNodeConfig {
	return &scyllav1alpha1.ScyllaNodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-node-config",
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
		},
		Spec: scyllav1alpha1.ScyllaNodeConfigSpec{
			Placement: scyllav1alpha1.ScyllaNodeConfigPlacement{
				NodeSelector: map[string]string{},
			},
		},
	}
}

func ScyllaNodeConfigServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func ScyllaNodeConfigClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"scylla.scylladb.com"},
				Resources: []string{"scyllanodeconfigs"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	}
}

func ScyllaNodeConfigClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     naming.NodeConfigAppName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
				Name:      naming.NodeConfigAppName,
			},
		},
	}
}

func ScyllaNodeConfigDaemonSet(snc *scyllav1alpha1.ScyllaNodeConfig, operatorImage, scyllaImage string) *appsv1.DaemonSet {
	labels := map[string]string{
		"app.kubernetes.io/name":   naming.NodeConfigAppName,
		naming.NodeConfigNameLabel: snc.Name,
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            snc.Name,
			Namespace:       naming.ScyllaOperatorNodeTuningNamespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{util.NewNodeConfigControllerRef(snc)},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: naming.NodeConfigAppName,
					// Required for getting the right iface name to tune
					HostNetwork:  true,
					NodeSelector: snc.Spec.Placement.NodeSelector,
					Affinity:     &snc.Spec.Placement.Affinity,
					Tolerations:  snc.Spec.Placement.Tolerations,
					Volumes: []corev1.Volume{
						hostDirVolume("hostfs", "/"),
					},
					Containers: []corev1.Container{
						{
							Name:            naming.NodeConfigAppName,
							Image:           operatorImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"node-config",
								"--node-name=$(NODE_NAME)",
								fmt.Sprintf("--scylla-node-config-name=%s", snc.Name),
								fmt.Sprintf("--scylla-image=%s", scyllaImage),
								fmt.Sprintf("--disable-optimizations=%s", strconv.FormatBool(snc.Spec.DisableOptimizations)),
								//TODO: add to Spec
								fmt.Sprintf("--loglevel=%d", 4),
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								volumeMount("hostfs", naming.HostFilesystemDirName, false),
							},
						},
					},
				},
			},
		},
	}
}

func PerftuneConfigMap(snc *scyllav1alpha1.ScyllaNodeConfig, scyllaPod *corev1.Pod, data []byte) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PerftuneResultName(string(scyllaPod.UID)),
			Namespace: scyllaPod.Namespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: snc.Name,
			},
			OwnerReferences: []metav1.OwnerReference{util.NewPodOwnerReference(scyllaPod)},
		},
	}

	if data != nil {
		cm.BinaryData = map[string][]byte{
			naming.PerftuneCommandName: data,
		}
	}

	return cm
}

func PerftuneJob(snc *scyllav1alpha1.ScyllaNodeConfig, nodeName, image, ifaceName, irqMask string, dataHostPaths []string, disableWritebackCache bool) *batchv1.Job {
	args := []string{
		"--tune", "system", "--tune-clock",
		"--tune", "net", "--nic", ifaceName, "--irq-cpu-mask", irqMask,
	}

	if len(dataHostPaths) > 0 {
		args = append(args, "--tune", "disks")
	}
	for _, hostPath := range dataHostPaths {
		args = append(args, "--dir", path.Join(naming.HostFilesystemDirName, hostPath))
	}

	if disableWritebackCache {
		args = append(args, "--write-back-cache", "false")
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:       snc.Name,
		naming.NodeConfigControllerLabel: nodeName,
	}

	perftuneJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            naming.PerftuneJobName(nodeName),
			Namespace:       naming.ScyllaOperatorNodeTuningNamespace,
			OwnerReferences: []metav1.OwnerReference{util.NewNodeConfigControllerRef(snc)},
			Labels:          labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations:   snc.Spec.Placement.Tolerations,
					NodeName:      nodeName,
					RestartPolicy: corev1.RestartPolicyOnFailure,
					HostPID:       true,
					HostNetwork:   true,
					Containers: []corev1.Container{
						{
							Name:            naming.PerftuneContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/opt/scylladb/scripts/perftune.py"},
							Args:            args,
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								volumeMount("hostfs", naming.HostFilesystemDirName, false),
								volumeMount("etc-systemd", "/etc/systemd", false),
								volumeMount("host-sys-class", "/sys/class", false),
								volumeMount("host-sys-devices", "/sys/devices", false),
								volumeMount("host-lib-systemd-system", "/lib/systemd/system", true),
								volumeMount("host-var-run-dbus", "/var/run/dbus", true),
								volumeMount("host-run-systemd-system", "/run/systemd/system", true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						hostDirVolume("hostfs", "/"),
						hostDirVolume("etc-systemd", "/etc/systemd"),
						hostDirVolume("host-sys-class", "/sys/class"),
						hostDirVolume("host-sys-devices", "/sys/devices"),
						hostDirVolume("host-lib-systemd-system", "/lib/systemd/system"),
						hostDirVolume("host-var-run-dbus", "/var/run/dbus"),
						hostDirVolume("host-run-systemd-system", "/run/systemd/system"),
					},
				},
			},
		},
	}

	// Host node might not be running irqbalance. Mount config only when it's present on the host.
	if _, err := os.Stat(path.Join(naming.HostFilesystemDirName, "/etc/sysconfig/irqbalance")); err == nil {
		perftuneJob.Spec.Template.Spec.Volumes = append(perftuneJob.Spec.Template.Spec.Volumes,
			hostFileVolume("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance"),
		)
		perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts,
			volumeMount("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance", false),
		)
	}

	return perftuneJob
}

func volumeMount(name, mountPath string, readonly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readonly,
	}
}

func hostVolume(name, hostPath string, volumeType *corev1.HostPathType) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: volumeType,
			},
		},
	}
}

func hostDirVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathDirectory
	return hostVolume(name, hostPath, &volumeType)
}

func hostFileVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathFile
	return hostVolume(name, hostPath, &volumeType)
}
