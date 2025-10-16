// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"bytes"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
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

func makePerftuneServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.PerftuneServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func makeSysctlsServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.SysctlsServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func makeRlimitsServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.RlimitsJobServiceAccountName,
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
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets/finalizers"},
				Verbs:     []string{"update"},
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
			{
				APIGroups:     []string{"security.openshift.io"},
				ResourceNames: []string{"privileged"},
				Resources:     []string{"securitycontextconstraints"},
				Verbs:         []string{"use"},
			},
		},
	}
}

func makePerftuneRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.PerftuneServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
			},
		},
	}
}

func makeSysctlsRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.SysctlsServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
			},
		},
	}
}

func makeRlimitsRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.RlimitsJobServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
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

func makePerftuneRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.PerftuneServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     naming.PerftuneServiceAccountName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
				Name:      naming.PerftuneServiceAccountName,
			},
		},
	}
}

func makeSysctlsRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.SysctlsServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     naming.SysctlsServiceAccountName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
				Name:      naming.SysctlsServiceAccountName,
			},
		},
	}
}

func makeRlimitsRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Name:      naming.RlimitsJobServiceAccountName,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     naming.RlimitsJobServiceAccountName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
				Name:      naming.RlimitsJobServiceAccountName,
			},
		},
	}
}

func makeNodeSetupDaemonSet(nc *scyllav1alpha1.NodeConfig, operatorImage, scyllaImage string) *appsv1.DaemonSet {
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
					ServiceAccountName:           naming.NodeConfigAppName,
					AutomountServiceAccountToken: pointer.Ptr(false),
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
						{
							Name: "kube-api-access",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: pointer.Ptr[int32](420),
									Sources: []corev1.VolumeProjection{
										{
											ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
												Path: "token",
											},
										},
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "kube-root-ca.crt",
												},
												Items: []corev1.KeyToPath{
													{

														Key:  corev1.ServiceAccountRootCAKey,
														Path: corev1.ServiceAccountRootCAKey,
													},
												},
											},
										},
									},
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
# It shall contain identical file tree as the host, so all symlinks keep working,
# and we'll add additional "virtual" data into /scylla-operator folder.
# This will avoid polluting the node with the extra data and avoids the need to clean up. 
cd "$( mktemp -d )"

for d in $( find /host -mindepth 1 -maxdepth 1 -type d -printf '%f\n' ); do
	mkdir -p "./${d}"
	mount --rbind "/host/${d}" "./${d}"
done

for f in $( find /host -mindepth 1 -maxdepth 1 -type f -printf '%f\n' ); do
	touch "./${f}"
	mount --bind "/host/${f}" "./${f}"
done

find /host -mindepth 1 -maxdepth 1 -type l -exec cp -P "{}" ./ \;

# Create /scylla-operator directory for additional files.
mkdir './scylla-operator'

# Mount operator binary
mkdir -p './scylla-operator/usr/bin/'
touch './scylla-operator/usr/bin/scylla-operator'
mount --bind {,./scylla-operator}/usr/bin/scylla-operator

# Mount container run to propagate secrets and configmaps.
mkdir './scylla-operator/run'
mount --rbind {,./scylla-operator}/run
							
# Mount container tmp.
mkdir './scylla-operator/tmp'
mount --rbind {,./scylla-operator}/tmp
export TMPDIR='/scylla-operator/tmp'

cat > ./scylla-operator/run/secrets/kubernetes.io/serviceaccount.kubeconfig <<EOF
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: "/scylla-operator/run/secrets/kubernetes.io/serviceaccount/ca.crt"
    server: "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}"
contexts:
- name: default
  context:
    cluster: local
    namespace: "${NAMESPACE}"
    user: sa
current-context: default
users:
- name: sa
  user:
    tokenFile: "/scylla-operator/run/secrets/kubernetes.io/serviceaccount/token"
EOF

exec chroot ./ /scylla-operator/usr/bin/scylla-operator node-setup-daemon \
--kubeconfig=/scylla-operator/run/secrets/kubernetes.io/serviceaccount.kubeconfig \
--namespace="$(NAMESPACE)" \
--pod-name="$(POD_NAME)" \
--node-name="$(NODE_NAME)" \
--node-config-name=` + fmt.Sprintf("%q", nc.Name) + ` \
--node-config-uid=` + fmt.Sprintf("%q", nc.UID) + ` \
--scylla-image=` + fmt.Sprintf("%q", scyllaImage) + ` \
--operator-image=` + fmt.Sprintf("%q", operatorImage) + ` \
` + fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()) + `
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
								{
									Name:             "kube-api-access",
									MountPath:        "/run/secrets/kubernetes.io/serviceaccount/",
									MountPropagation: pointer.Ptr(corev1.MountPropagationNone),
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeConfigMaps(nc *scyllav1alpha1.NodeConfig) ([]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap

	sysctlConfigMap, err := makeSysctlConfigMap(nc)
	if err != nil {
		return nil, fmt.Errorf("can't make sysctl configmap: %w", err)
	}
	configMaps = append(configMaps, sysctlConfigMap)

	return configMaps, nil
}

func makeSysctlConfigMap(nc *scyllav1alpha1.NodeConfig) (*corev1.ConfigMap, error) {
	name, err := naming.NodeConfigSysctlConfigMapName(nc)
	if err != nil {
		return nil, fmt.Errorf("can't get sysctl configmap name: %w", err)
	}

	data, err := makeSysctlData(nc.Spec.Sysctls)
	if err != nil {
		return nil, fmt.Errorf("can't make sysctl configmap data: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.KubernetesNameLabel: naming.NodeConfigAppName,
				naming.NodeConfigNameLabel: nc.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nc, nodeConfigControllerGVK),
			},
		},
		Data: data,
	}
	return cm, nil
}

func makeSysctlData(sysctls []corev1.Sysctl) (map[string]string, error) {
	var err error
	var confBuf bytes.Buffer
	for _, s := range sysctls {
		_, err = confBuf.WriteString(fmt.Sprintf("%s = %s\n", s.Name, s.Value))
		if err != nil {
			return nil, fmt.Errorf("can't write string: %w", err)
		}
	}

	data := map[string]string{
		naming.SysctlConfigFileName: confBuf.String(),
	}

	return data, nil
}
