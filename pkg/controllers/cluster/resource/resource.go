package resource

import (
	"fmt"
	"path"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	officialScyllaRepo             = "scylladb/scylla"
	officialScyllaManagerAgentRepo = "scylladb/scylla-manager-agent"
)

func HeadlessServiceForCluster(c *scyllav1.ScyllaCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            naming.HeadlessServiceNameForCluster(c),
			Namespace:       c.Namespace,
			Labels:          naming.ClusterLabels(c),
			OwnerReferences: []metav1.OwnerReference{util.NewControllerRef(c)},
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

func MemberServiceForPod(pod *corev1.Pod, cluster *scyllav1.ScyllaCluster) *corev1.Service {
	labels := naming.ClusterLabels(cluster)
	labels[naming.DatacenterNameLabel] = pod.Labels[naming.DatacenterNameLabel]
	labels[naming.RackNameLabel] = pod.Labels[naming.RackNameLabel]
	// If Member is seed, add the appropriate label
	if strings.HasSuffix(pod.Name, "-0") || strings.HasSuffix(pod.Name, "-1") {
		labels[naming.SeedLabel] = ""
	}
	rackName := pod.Labels[naming.RackNameLabel]
	if replaceAddr, ok := cluster.Status.Racks[rackName].ReplaceAddressFirstBoot[pod.Name]; ok && replaceAddr != "" {
		labels[naming.ReplaceLabel] = replaceAddr
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{util.NewControllerRef(cluster)},
			Labels:          labels,
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 naming.StatefulSetPodLabel(pod.Name),
			Ports:                    memberServicePorts(cluster),
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
			Name: "agent-api",
			Port: 10001,
		},
	}
	if cluster.Spec.Alternator.Enabled() {
		ports = append(ports, corev1.ServicePort{
			Name: "alternator",
			Port: cluster.Spec.Alternator.Port,
		})
	} else {
		ports = append(ports, corev1.ServicePort{
			Name: "cql",
			Port: 9042,
		}, corev1.ServicePort{
			Name: "cql-ssl",
			Port: 9142,
		}, corev1.ServicePort{
			Name: "thrift",
			Port: 9160,
		})
	}
	return ports
}

func StatefulSetForRack(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, sidecarImage string) *appsv1.StatefulSet {
	rackLabels := naming.RackLabels(r, c)
	placement := r.Placement
	if placement == nil {
		placement = &scyllav1.PlacementSpec{}
	}
	opt := true
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            naming.StatefulSetNameForRack(r, c),
			Namespace:       c.Namespace,
			Labels:          rackLabels,
			OwnerReferences: []metav1.OwnerReference{util.NewControllerRef(c)},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: util.RefFromInt32(0),
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.HeadlessServiceNameForCluster(c),
			Selector: &metav1.LabelSelector{
				MatchLabels: rackLabels,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
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
							Name: "scylla-agent-config-volume",
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
					},
					Tolerations: placement.Tolerations,
					InitContainers: []corev1.Container{
						{
							Name:            "sidecar-injection",
							Image:           sidecarImage,
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"/bin/sh",
								"-c",
								fmt.Sprintf("cp -a /usr/bin/scylla-operator %s", naming.SharedDirName),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("200M"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("200M"),
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
							ImagePullPolicy: "IfNotPresent",
							Ports:           containerPorts(c),
							// TODO: unprivileged entrypoint
							Command: []string{
								path.Join(naming.SharedDirName, "scylla-operator"),
								"sidecar",
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "CPU",
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
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_NICE"},
								},
							},
							LivenessProbe: &corev1.Probe{
								// Initial delay should be big, because scylla runs benchmarks
								// to tune the IO settings.
								InitialDelaySeconds: int32(400),
								TimeoutSeconds:      int32(5),
								PeriodSeconds:       int32(10),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromInt(naming.ProbePort),
										Path: naming.LivenessProbePath,
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: int32(30),
								TimeoutSeconds:      int32(5),
								// TODO: Investigate if it's optimal to call status every 10 seconds
								PeriodSeconds:    int32(10),
								SuccessThreshold: int32(3),
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
					ServiceAccountName: naming.ServiceAccountNameForMembers(c),
					Affinity: &corev1.Affinity{
						NodeAffinity:    placement.NodeAffinity,
						PodAffinity:     placement.PodAffinity,
						PodAntiAffinity: placement.PodAntiAffinity,
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   naming.PVCTemplateName,
						Labels: rackLabels,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: r.Storage.StorageClassName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(r.Storage.Capacity),
							},
						},
					},
				},
			},
		},
	}

	sysctlContainer := sysctlInitContainer(c.Spec.Sysctls)
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
	return sts
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

func sysctlInitContainer(sysctls []string) *corev1.Container {
	if len(sysctls) == 0 {
		return nil
	}
	opt := true
	return &corev1.Container{
		Name:            "sysctl-buddy",
		Image:           "busybox:1.31.1",
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: &opt,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("200M"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("200M"),
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
		ImagePullPolicy: "IfNotPresent",
		Args: []string{
			"-c",
			naming.ScyllaAgentConfigDefaultFile,
			"-c",
			naming.ScyllaAgentConfigDirName + "/scylla-manager-agent.yaml",
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
				Name:      "scylla-agent-config-volume",
				MountPath: naming.ScyllaAgentConfigDirName,
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

func ImageForCluster(c *scyllav1.ScyllaCluster) string {
	repo := officialScyllaRepo
	if c.Spec.Repository != nil {
		repo = *c.Spec.Repository
	}
	return fmt.Sprintf("%s:%s", repo, c.Spec.Version)
}

func agentImageForCluster(c *scyllav1.ScyllaCluster) string {
	repo := officialScyllaManagerAgentRepo
	if c.Spec.AgentRepository != nil {
		repo = *c.Spec.AgentRepository
	}
	version := "latest"
	if c.Spec.AgentVersion != nil {
		version = *c.Spec.AgentVersion
	}
	return fmt.Sprintf("%s:%s", repo, version)
}

func stringOrDefault(str, def string) string {
	if str != "" {
		return str
	}
	return def
}
