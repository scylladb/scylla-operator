// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeScyllaOperatorNodeTuningNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func makeNodeConfigServiceAccount() *corev1.ServiceAccount {
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

func NodeConfigClusterRole() *rbacv1.ClusterRole {
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
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"scylla.scylladb.com"},
				Resources: []string{"nodeconfigs"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"scylla.scylladb.com"},
				Resources: []string{"nodeconfigs/status"},
				Verbs:     []string{"update"},
			},
		},
	}
}

func makeNodeConfigClusterRoleBinding() *rbacv1.ClusterRoleBinding {
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

func makeNodeSetupDaemonSet(nc *scyllav1alpha1.NodeConfig, operatorBinaryPath, operatorImage, scyllaImage string) *appsv1.DaemonSet {
	if nc.Spec.LocalDiskSetup == nil && nc.Spec.DisableOptimizations {
		return nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":   naming.NodeConfigAppName,
		naming.NodeConfigNameLabel: nc.Name,
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-node-setup", nc.Name),
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nc, nodeConfigControllerGVK),
			},
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
					NodeSelector: nc.Spec.Placement.NodeSelector,
					Affinity:     &nc.Spec.Placement.Affinity,
					Tolerations:  nc.Spec.Placement.Tolerations,
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
					},
					Containers: []corev1.Container{
						{
							Name:            naming.NodeConfigAppName,
							Image:           operatorImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/bash",
								"-euExo",
								"pipefail",
								"-O",
								"inherit_errexit",
								"-c",
							},
							Args: []string{
								`
# Create a temporary root that will represent the host file system and devices.
cd "$( mktemp -d )"

# Skip mounting entire /var, /run, /tmp to prevent from polluting host with files created in runtime in the container.
# Required directories originating from these are mounted explicitly below.
for d in $( find /host -mindepth 1 -maxdepth 1 -type d -not -path /host/var -not -path /host/run -not -path /host/tmp -printf '%f\n' ); do
	mkdir -p "./${d}"
	mount --rbind "/host/${d}" "./${d}"
done

for f in $( find /host -mindepth 1 -maxdepth 1 -type f -printf '%f\n' ); do
	touch "./${f}"
	mount --bind "/host/${f}" "./${f}"
done

find /host -mindepth 1 -maxdepth 1 -type l -exec cp -P "{}" ./ \;

# Required for node setup
for dir in "run/udev" "run/mdadm" "run/dbus"; do
	if [ -d "/host/${dir}" ]; then
		mkdir -p "./${dir}"
		mount --rbind "/host/${dir}" "./${dir}"
	fi
done

# Required by CRI client
for dir in "run/crio" "run/containerd"; do
	if [ -d "/host/${dir}" ]; then
		mkdir -p "./${dir}"
		mount --rbind "/host/${dir}" "./${dir}"
	fi
done

if [ -f "/host/run/dockershim.sock" ]; then
	touch "./run/dockershim.sock"
	mount --bind "/host/run/dockershim.sock" "./run/dockershim.sock"
fi

# Required by node tuning
if [ -d "/host/var/lib/kubelet" ]; then
	mkdir -p "./var/lib/kubelet"
	mount --rbind "/host/var/lib/kubelet" "./var/lib/kubelet"
fi

# Mount operator binary
mkdir -p ./scylla-operator
touch ./scylla-operator/scylla-operator
mount --bind ` + operatorBinaryPath + ` ./scylla-operator/scylla-operator

# Mount service account
mkdir -p "./run/secrets/kubernetes.io/serviceaccount"
for f in ca.crt token; do
	touch "./run/secrets/kubernetes.io/serviceaccount/${f}"
	mount --bind "/run/secrets/kubernetes.io/serviceaccount/${f}" "./run/secrets/kubernetes.io/serviceaccount/${f}"
done

# Create special symlink
if [ -L "/host/var/run" ]; then
	mkdir -p "./var"
	ln -s "../run" "./var/run"
fi

exec chroot ./ /scylla-operator/scylla-operator node-setup-daemon \
--namespace="$(NAMESPACE)" \
--pod-name="$(POD_NAME)" \
--node-name="$(NODE_NAME)" \
--node-config-name=` + fmt.Sprintf("%q", nc.Name) + ` \
--node-config-uid=` + fmt.Sprintf("%q", nc.UID) + ` \
--scylla-image=` + fmt.Sprintf("%q", scyllaImage) + ` \
--disable-optimizations=` + fmt.Sprintf("%t", nc.Spec.DisableOptimizations) + ` \
--loglevel=` + fmt.Sprintf("%d", 4) + `
							`},
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
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
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "hostfs",
									MountPath:        "/host",
									MountPropagation: pointer.Ptr(corev1.MountPropagationBidirectional),
								},
							},
						},
					},
				},
			},
		},
	}
}
