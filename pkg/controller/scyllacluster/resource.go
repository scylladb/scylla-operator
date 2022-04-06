package scyllacluster

import (
	"fmt"
	"path"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
)

const (
	scyllaAgentConfigVolumeName    = "scylla-agent-config-volume"
	scyllaAgentAuthTokenVolumeName = "scylla-agent-auth-token-volume"
)

const (
	rootUID = 0
	rootGID = 0
)

func HeadlessServiceForCluster(c *scyllav1.ScyllaCluster) *corev1.Service {
	labels := naming.ClusterLabels(c)
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeIdentity)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.HeadlessServiceNameForCluster(c),
			Namespace: c.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, controllerGVK),
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Type:      corev1.ServiceTypeClusterIP,
			Selector:  naming.ClusterLabels(c),
			Ports: []corev1.ServicePort{
				{
					Name: "prometheus",
					Port: 9180,
				},
				{
					Name: "agent-prometheus",
					Port: 5090,
				},
			},
		},
	}
}

func MemberService(sc *scyllav1.ScyllaCluster, rackName, name string, oldService *corev1.Service) *corev1.Service {
	labels := naming.ClusterLabels(sc)
	labels[naming.DatacenterNameLabel] = sc.Spec.Datacenter.Name
	labels[naming.RackNameLabel] = rackName
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeMember)

	// Copy the old replace label, if present.
	var replaceAddr string
	var hasReplaceLabel bool
	if oldService != nil {
		replaceAddr, hasReplaceLabel = oldService.Labels[naming.ReplaceLabel]
		if hasReplaceLabel {
			labels[naming.ReplaceLabel] = replaceAddr
		}

		// Copy the maintenance label, if present
		oldMaintenanceLabel, oldMaintenanceLabelPresent := oldService.Labels[naming.NodeMaintenanceLabel]
		if oldMaintenanceLabelPresent {
			labels[naming.NodeMaintenanceLabel] = oldMaintenanceLabel
		}
	}

	// Only new service should get the replace address, old service keeps "" until deleted.
	if !hasReplaceLabel || len(replaceAddr) != 0 {
		rackStatus, ok := sc.Status.Racks[rackName]
		if ok {
			replaceAddr := rackStatus.ReplaceAddressFirstBoot[name]
			if len(replaceAddr) != 0 {
				labels[naming.ReplaceLabel] = replaceAddr
			}
		}
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, controllerGVK),
			},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 naming.StatefulSetPodLabel(name),
			Ports:                    memberServicePorts(sc),
			PublishNotReadyAddresses: true,
		},
	}
}

func memberServicePorts(cluster *scyllav1.ScyllaCluster) []corev1.ServicePort {
	ports := []corev1.ServicePort{
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
			Name: "agent-api",
			Port: 10001,
		},
		{
			Name: "node-exporter",
			Port: 9100,
		},
	}
	if cluster.Spec.Alternator.Enabled() {
		ports = append(ports, corev1.ServicePort{
			Name: "alternator",
			Port: cluster.Spec.Alternator.Port,
		})
	} else {
		ports = append(ports, corev1.ServicePort{
			Name: "thrift",
			Port: 9160,
		})
	}
	return ports
}

// StatefulSetForRack make a StatefulSet for the rack.
// existingSts may be nil if it doesn't exist yet.
func StatefulSetForRack(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, existingSts *appsv1.StatefulSet, sidecarImage string) (*appsv1.StatefulSet, error) {
	matchLabels := naming.RackLabels(r, c)
	rackLabels := naming.RackLabels(r, c)
	rackLabels[naming.ScyllaVersionLabel] = c.Spec.Version

	placement := r.Placement
	if placement == nil {
		placement = &scyllav1.PlacementSpec{}
	}
	opt := true

	storageCapacity, err := resource.ParseQuantity(r.Storage.Capacity)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %v", r.Storage.Capacity, err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.StatefulSetNameForRack(r, c),
			Namespace: c.Namespace,
			Labels:    rackLabels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, controllerGVK),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32Ptr(r.Members),
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.HeadlessServiceNameForCluster(c),
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: pointer.Int32Ptr(0),
				},
			},
			// Template for Pods
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: rackLabels,
					Annotations: map[string]string{
						naming.PrometheusScrapeAnnotation: naming.LabelValueTrue,
						naming.PrometheusPortAnnotation:   "9180",
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: c.Spec.Network.HostNetworking,
					DNSPolicy:   c.Spec.Network.GetDNSPolicy(),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  pointer.Int64(rootUID),
						RunAsGroup: pointer.Int64(rootGID),
					},
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
										Name: stringOrDefault(r.ScyllaConfig, "scylla-config"),
									},
									Optional: &opt,
								},
							},
						},
						{
							Name: scyllaAgentConfigVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: stringOrDefault(r.ScyllaAgentConfig, "scylla-agent-config-secret"),
									Optional:   &opt,
								},
							},
						},
						{
							Name: "scylla-client-config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "scylla-client-config-secret",
									Optional:   &opt,
								},
							},
						},
						{
							Name: scyllaAgentAuthTokenVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: naming.AgentAuthTokenSecretName(c.Name),
								},
							},
						},
					},
					Tolerations: placement.Tolerations,
					InitContainers: []corev1.Container{
						{
							Name:            naming.SidecarInjectorContainerName,
							Image:           sidecarImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/bin/sh",
								"-c",
								fmt.Sprintf("cp -a /usr/bin/scylla-operator %s", naming.SharedDirName),
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
									MountPath: naming.SharedDirName,
									ReadOnly:  false,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            naming.ScyllaContainerName,
							Image:           ImageForCluster(c),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           containerPorts(c),
							// TODO: unprivileged entrypoint
							Command: []string{
								path.Join(naming.SharedDirName, "scylla-operator"),
								"sidecar",
								"--service-name=$(SERVICE_NAME)",
								"--cpu-count=$(CPU_COUNT)",
								// TODO: make it configurable
								"--loglevel=4",
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
											ContainerName: naming.ScyllaContainerName,
											Resource:      "limits.cpu",
											Divisor:       resource.MustParse("1"),
										},
									},
								},
							},
							Resources: r.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      naming.PVCTemplateName,
									MountPath: naming.DataDir,
								},
								{
									Name:      "shared",
									MountPath: naming.SharedDirName,
									ReadOnly:  true,
								},
								{
									Name:      "scylla-config-volume",
									MountPath: naming.ScyllaConfigDirName,
									ReadOnly:  true,
								},
								{
									Name:      "scylla-client-config-volume",
									MountPath: naming.ScyllaClientConfigDirName,
									ReadOnly:  true,
								},
							},
							// Add CAP_SYS_NICE as instructed by scylla logs
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  pointer.Int64(rootUID),
								RunAsGroup: pointer.Int64(rootGID),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_NICE"},
								},
							},
							StartupProbe: &corev1.Probe{
								// Initial delay should be big, because scylla runs benchmarks
								// to tune the IO settings.
								// TODO: Lower the timeout when we fix probes. We have temporarily changed them from 5s
								// to 30s to survive cluster overload.
								// Relevant issue: https://github.com/scylladb/scylla-operator/issues/844
								TimeoutSeconds:   int32(30),
								FailureThreshold: int32(40),
								PeriodSeconds:    int32(10),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromInt(naming.ProbePort),
										Path: naming.LivenessProbePath,
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								// TODO: Lower the timeout when we fix probes. Currently we need them raised
								// 		 because scylla doesn't respond under load. (#844)
								TimeoutSeconds:   int32(10),
								FailureThreshold: int32(12),
								PeriodSeconds:    int32(10),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromInt(naming.ProbePort),
										Path: naming.LivenessProbePath,
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								// TODO: Lower the timeout when we fix probes. We have temporarily changed them from 5s
								// to 30s to survive cluster overload.
								// Relevant issue: https://github.com/scylladb/scylla-operator/issues/844
								TimeoutSeconds:   int32(30),
								FailureThreshold: int32(1),
								PeriodSeconds:    int32(10),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromInt(naming.ProbePort),
										Path: naming.ReadinessProbePath,
									},
								},
							},
							// Before a Scylla Pod is stopped, execute nodetool drain to
							// flush the memtable to disk and stop listening for connections.
							// This is necessary to ensure we don't lose any data and we don't
							// need to replay the commitlog in the next startup.
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh", "-c", "PID=$(pgrep -x scylla);supervisorctl stop scylla; while kill -0 $PID; do sleep 1; done;",
										},
									},
								},
							},
						},
					},
					ServiceAccountName: naming.MemberServiceAccountNameForScyllaCluster(c.Name),
					Affinity: &corev1.Affinity{
						NodeAffinity:    placement.NodeAffinity,
						PodAffinity:     placement.PodAffinity,
						PodAntiAffinity: placement.PodAntiAffinity,
					},
					ImagePullSecrets:              c.Spec.ImagePullSecrets,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(900),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   naming.PVCTemplateName,
						Labels: matchLabels,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: r.Storage.StorageClassName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageCapacity,
							},
						},
					},
				},
			},
		},
	}

	if len(c.Spec.ForceRedeploymentReason) != 0 {
		sts.Spec.Template.Annotations[naming.ForceRedeploymentReasonAnnotation] = c.Spec.ForceRedeploymentReason
	}

	if existingSts != nil {
		sts.ResourceVersion = existingSts.ResourceVersion
		if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType &&
			existingSts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType &&
			existingSts.Spec.UpdateStrategy.RollingUpdate != nil &&
			existingSts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			*sts.Spec.UpdateStrategy.RollingUpdate.Partition = *existingSts.Spec.UpdateStrategy.RollingUpdate.Partition
		}
	}

	// Make sure we adjust if it was scaled in between.
	if *sts.Spec.UpdateStrategy.RollingUpdate.Partition > *sts.Spec.Replicas {
		sts.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Int32Ptr(*sts.Spec.Replicas)
	}

	sysctlContainer := sysctlInitContainer(c.Spec.Sysctls, sidecarImage)
	if sysctlContainer != nil {
		sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, *sysctlContainer)
	}
	for _, VolumeMount := range r.VolumeMounts {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			sts.Spec.Template.Spec.Containers[0].VolumeMounts, *VolumeMount.DeepCopy())
	}
	for _, Volume := range r.Volumes {
		sts.Spec.Template.Spec.Volumes = append(
			sts.Spec.Template.Spec.Volumes, *Volume.DeepCopy())
	}
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, agentContainer(r, c))
	return sts, nil
}

func containerPorts(c *scyllav1.ScyllaCluster) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
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
	}

	if c.Spec.Alternator.Enabled() {
		ports = append(ports, corev1.ContainerPort{
			Name:          "alternator",
			ContainerPort: c.Spec.Alternator.Port,
		})
	} else {
		ports = append(ports, corev1.ContainerPort{
			Name:          "cql",
			ContainerPort: 9042,
		}, corev1.ContainerPort{
			Name:          "cql-ssl",
			ContainerPort: 9142,
		}, corev1.ContainerPort{
			Name:          "thrift",
			ContainerPort: 9160,
		})
	}

	return ports
}

func sysctlInitContainer(sysctls []string, image string) *corev1.Container {
	if len(sysctls) == 0 {
		return nil
	}
	opt := true
	return &corev1.Container{
		Name:            "sysctl-buddy",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &opt,
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
		Command: []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf("sysctl -w %s", strings.Join(sysctls, " ")),
		},
	}
}

func agentContainer(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) corev1.Container {
	cnt := corev1.Container{
		Name:            "scylla-manager-agent",
		Image:           agentImageForCluster(c),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"-c",
			naming.ScyllaAgentConfigDefaultFile,
			"-c",
			path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentConfigFileName),
			"-c",
			path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentAuthTokenFileName),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "agent-rest-api",
				ContainerPort: 10001,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      naming.PVCTemplateName,
				MountPath: naming.DataDir,
			},
			{
				Name:      scyllaAgentConfigVolumeName,
				MountPath: path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentConfigFileName),
				SubPath:   naming.ScyllaAgentConfigFileName,
				ReadOnly:  true,
			},
			{
				Name:      scyllaAgentAuthTokenVolumeName,
				MountPath: path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentAuthTokenFileName),
				SubPath:   naming.ScyllaAgentAuthTokenFileName,
				ReadOnly:  true,
			},
		},
		Resources: r.AgentResources,
	}

	for _, vm := range r.AgentVolumeMounts {
		cnt.VolumeMounts = append(cnt.VolumeMounts, *vm.DeepCopy())
	}

	return cnt
}

func MakePodDisruptionBudget(c *scyllav1.ScyllaCluster) *v1beta1.PodDisruptionBudget {
	maxUnavailable := intstr.FromInt(1)
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PodDisruptionBudgetName(c),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, controllerGVK),
			},
			Labels: naming.ClusterLabels(c),
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       metav1.SetAsLabelSelector(naming.ClusterLabels(c)),
		},
	}
}

func MakeAgentAuthTokenSecret(c *scyllav1.ScyllaCluster, authToken string) (*corev1.Secret, error) {
	data, err := helpers.GetAgentAuthTokenConfig(authToken)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.AgentAuthTokenSecretName(c.Name),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, controllerGVK),
			},
			Labels: naming.ClusterLabels(c),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			naming.ScyllaAgentAuthTokenFileName: data,
		},
	}, nil
}

func ImageForCluster(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s:%s", c.Spec.Repository, c.Spec.Version)
}

func agentImageForCluster(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s:%s", c.Spec.AgentRepository, c.Spec.AgentVersion)
}

func stringOrDefault(str, def string) string {
	if str != "" {
		return str
	}
	return def
}

func MakeServiceAccount(sc *scyllav1.ScyllaCluster, serviceAccounts map[string]*corev1.ServiceAccount, serviceAccountLister corev1listers.ServiceAccountLister) (*corev1.ServiceAccount, error) {
	annotations := map[string]string{}
	name := naming.MemberServiceAccountNameForScyllaCluster(sc.Name)

	existing, found := serviceAccounts[name]
	if !found {
		// In 1.7.0 we started managing ServiceAccounts. Because previously these were created by users
		// there's a high chance that they don't match our selector, hence are not available in serviceAccounts.
		// Ref: https://github.com/scylladb/scylla-operator/issues/932
		//
		// Cache fetch happens only once - when SA is being adopted, or it's missing.
		// It can be removed in 1.8.0 because all SAs will be already adopted, and they will match our selector.
		// TODO: https://github.com/scylladb/scylla-operator/issues/934
		var err error
		existing, err = serviceAccountLister.ServiceAccounts(sc.Namespace).Get(name)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("can't get %s service account: %w", naming.ManualRef(sc.Namespace, name), err)
		}
	}

	if existing != nil {
		// We need to copy annotations stored in user SA because these may control integrations with k8s provider services
		// like IAM management.
		for k, v := range existing.Annotations {
			annotations[k] = v
		}
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, controllerGVK),
			},
			Labels:      naming.ClusterLabels(sc),
			Annotations: annotations,
		},
	}, nil
}

func MakeRoleBinding(sc *scyllav1.ScyllaCluster) *rbacv1.RoleBinding {
	saName := naming.MemberServiceAccountNameForScyllaCluster(sc.Name)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, controllerGVK),
			},
			Labels: naming.ClusterLabels(sc),
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: sc.Namespace,
				Name:      saName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     naming.ScyllaClusterMemberClusterRoleName,
		},
	}
}
