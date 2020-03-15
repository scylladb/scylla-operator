package resource

import (
	"fmt"
	"github.com/scylladb/scylla-operator/cmd/options"
	"path"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/controller/endpoint"
)

const (
	officialScyllaRepo             = "scylladb/scylla"
	officialScyllaManagerAgentRepo = "scylladb/scylla-manager-agent"
)

func HeadlessServiceForCluster(c *scyllav1alpha1.Cluster) *corev1.Service {
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
			},
		},
	}
}

func MemberServiceForPod(pod *corev1.Pod, cluster *scyllav1alpha1.Cluster) *corev1.Service {

	labels := naming.ClusterLabels(cluster)
	labels[naming.DatacenterNameLabel] = pod.Labels[naming.DatacenterNameLabel]
	labels[naming.RackNameLabel] = pod.Labels[naming.RackNameLabel]
	// If Member is seed, add the appropriate label
	if strings.HasSuffix(pod.Name, "-0") || strings.HasSuffix(pod.Name, "-1") {
		labels[naming.SeedLabel] = ""
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{util.NewControllerRef(cluster)},
			Labels:          labels,
			Annotations:     map[string]string{endpoint.TolerateUnreadyEndpointsAnnotation: "true"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: naming.StatefulSetPodLabel(pod.Name),
			Ports: []corev1.ServicePort{
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
					Name: "thrift",
					Port: 9160,
				},
				{
					Name: "cql-ssl",
					Port: 9142,
				},
			},
			PublishNotReadyAddresses: true,
		},
	}
}

func StatefulSetForRack(r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster, sidecarImage string) *appsv1.StatefulSet {
	rackLabels := naming.RackLabels(r, c)
	placement := r.Placement
	if placement == nil {
		placement = &scyllav1alpha1.PlacementSpec{}
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
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: stringOrDefault(r.ScyllaAgentConfig, "scylla-agent-config"),
									},
									Optional: &opt,
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
								fmt.Sprintf("cp -a /sidecar/* %s", naming.SharedDirName),
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
									Name:          "cql",
									ContainerPort: 9042,
								},
								{
									Name:          "thrift",
									ContainerPort: 9160,
								},
								{
									Name:          "jolokia",
									ContainerPort: 8778,
								},
								{
									Name:          "prometheus",
									ContainerPort: 9180,
								},
							},
							// TODO: unprivileged entrypoint
							Command: []string{
								path.Join(naming.SharedDirName, "tini"),
								"--",
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
											"nodetool",
											"drain",
										},
									},
								},
							},
						},
					},
					ServiceAccountName: naming.ServiceAccountNameForMembers(c),
					Affinity:           &placement.Affinity,
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

	// If the scylla-agent feature-flag is enabled, add the Scylla Agent sidecar
	if options.GetOperatorOptions().EnableScyllaAgentSidecar {
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, *agentContainer(c))
	}
	return sts
}

func agentContainer(c *scyllav1alpha1.Cluster) *corev1.Container {
	return &corev1.Container{
		Name:            "scylla-manager-agent",
		Image:           agentImageForCluster(c),
		ImagePullPolicy: "IfNotPresent",
		Args: []string{
			"-c",
			"/etc/scylla-manager-agent/scylla-manager-agent.yaml",
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
	}
}

func ImageForCluster(c *scyllav1alpha1.Cluster) string {
	repo := officialScyllaRepo
	if c.Spec.Repository != nil {
		repo = *c.Spec.Repository
	}
	return fmt.Sprintf("%s:%s", repo, c.Spec.Version)
}

func agentImageForCluster(c *scyllav1alpha1.Cluster) string {
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
