package scyllacluster

import (
	"fmt"
	"path"
	"sort"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

const (
	scyllaAgentConfigVolumeName    = "scylla-agent-config-volume"
	scyllaAgentAuthTokenVolumeName = "scylla-agent-auth-token-volume"
	scylladbServingCertsVolumeName = "scylladb-serving-certs"
	scylladbClientCAVolumeName     = "scylladb-client-ca"
	scylladbUserAdminVolumeName    = "scylladb-user-admin"
)

const (
	rootUID = 0
	rootGID = 0
)

const (
	portNameCQL              = "cql"
	portNameCQLSSL           = "cql-ssl"
	portNameCQLShardAware    = "cql-shard-aware"
	portNameCQLSSLShardAware = "cql-ssl-shard-aware"
	portNameAlternator       = "alternator"
	portNameThrift           = "thrift"
)

func IdentityService(c *scyllav1.ScyllaCluster) *corev1.Service {
	labels := naming.ClusterLabels(c)
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeIdentity)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.HeadlessServiceNameForCluster(c),
			Namespace: c.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: naming.ClusterLabels(c),
			Ports:    servicePorts(c),
		},
	}
}

func MemberService(sc *scyllav1.ScyllaCluster, rackName, name string, oldService *corev1.Service, jobs map[string]*batchv1.Job) *corev1.Service {
	svcLabels := naming.ClusterLabels(sc)
	svcLabels[naming.DatacenterNameLabel] = sc.Spec.Datacenter.Name
	svcLabels[naming.RackNameLabel] = rackName
	svcLabels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeMember)

	// Copy the old replace label, if present.
	var replaceAddr string
	var hasReplaceLabel bool
	if oldService != nil {
		replaceAddr, hasReplaceLabel = oldService.Labels[naming.ReplaceLabel]
		if hasReplaceLabel {
			svcLabels[naming.ReplaceLabel] = replaceAddr
		}
	}

	// Only new service should get the replace address, old service keeps "" until deleted.
	if !hasReplaceLabel || len(replaceAddr) != 0 {
		rackStatus, ok := sc.Status.Racks[rackName]
		if ok {
			replaceAddr := rackStatus.ReplaceAddressFirstBoot[name]
			if len(replaceAddr) != 0 {
				svcLabels[naming.ReplaceLabel] = replaceAddr
			}
		}
	}

	svcAnnotations := make(map[string]string)
	if oldService != nil {
		_, hasLastCleanedUpRingHash := oldService.Annotations[naming.LastCleanedUpTokenRingHashAnnotation]
		currentTokenRingHash, hasCurrentRingHash := oldService.Annotations[naming.CurrentTokenRingHashAnnotation]
		if !hasLastCleanedUpRingHash && hasCurrentRingHash {
			svcAnnotations[naming.LastCleanedUpTokenRingHashAnnotation] = currentTokenRingHash
		}
	}

	cleanupJob, ok := jobs[naming.CleanupJobForService(name)]
	if ok {
		if len(cleanupJob.Annotations[naming.CleanupJobTokenRingHashAnnotation]) != 0 && cleanupJob.Status.CompletionTime != nil {
			svcAnnotations[naming.LastCleanedUpTokenRingHashAnnotation] = cleanupJob.Annotations[naming.CleanupJobTokenRingHashAnnotation]
		}
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
			Labels:      svcLabels,
			Annotations: svcAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 naming.StatefulSetPodLabel(name),
			Ports:                    servicePorts(sc),
			PublishNotReadyAddresses: true,
		},
	}
}

func servicePorts(cluster *scyllav1.ScyllaCluster) []corev1.ServicePort {
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
			Name: portNameCQL,
			Port: 9042,
		},
		{
			Name: portNameCQLSSL,
			Port: 9142,
		},
		{
			Name: portNameCQLShardAware,
			Port: 19042,
		},
		{
			Name: portNameCQLSSLShardAware,
			Port: 19142,
		},
	}

	if cluster.Spec.Alternator.Enabled() {
		ports = append(ports, corev1.ServicePort{
			Name: portNameAlternator,
			Port: cluster.Spec.Alternator.Port,
		})
	} else {
		ports = append(ports, corev1.ServicePort{
			Name: portNameThrift,
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
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(r.Members),
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.HeadlessServiceNameForCluster(c),
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: pointer.Int32(0),
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
						}

						if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
							volumes = append(volumes, []corev1.Volume{
								{
									Name: scylladbServingCertsVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalServingCertName(c.Name),
										},
									},
								},
								{
									Name: scylladbClientCAVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalClientCAName(c.Name),
										},
									},
								},
								{
									Name: scylladbUserAdminVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalUserAdminCertName(c.Name),
										},
									},
								},
							}...)
						}

						return volumes
					}(),
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
							Command: func() []string {
								cmd := []string{
									path.Join(naming.SharedDirName, "scylla-operator"),
									"sidecar",
									fmt.Sprintf("--feature-gates=%s", func() string {
										features := utilfeature.DefaultMutableFeatureGate.GetAll()
										res := make([]string, 0, len(features))
										for name := range features {
											res = append(res, fmt.Sprintf("%s=%t", name, utilfeature.DefaultMutableFeatureGate.Enabled(name)))
										}
										sort.Strings(res)
										return strings.Join(res, ",")
									}()),
									"--service-name=$(SERVICE_NAME)",
									"--cpu-count=$(CPU_COUNT)",
									// TODO: make it configurable
									"--loglevel=2",
								}

								if len(c.Spec.ExternalSeeds) > 0 {
									cmd = append(cmd, fmt.Sprintf("--external-seeds=%s", strings.Join(c.Spec.ExternalSeeds, ",")))
								}

								return cmd
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
											ContainerName: naming.ScyllaContainerName,
											Resource:      "limits.cpu",
											Divisor:       resource.MustParse("1"),
										},
									},
								},
							},
							Resources: r.Resources,
							VolumeMounts: func() []corev1.VolumeMount {
								mounts := []corev1.VolumeMount{
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
								}

								if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
									mounts = append(mounts, []corev1.VolumeMount{
										{
											Name:      scylladbServingCertsVolumeName,
											MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs",
											ReadOnly:  true,
										},
										{
											Name:      scylladbClientCAVolumeName,
											MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/client-ca",
											ReadOnly:  true,
										},
										{
											Name:      scylladbUserAdminVolumeName,
											MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/user-admin",
											ReadOnly:  true,
										},
									}...)
								}

								return mounts
							}(),
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
								ProbeHandler: corev1.ProbeHandler{
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
								ProbeHandler: corev1.ProbeHandler{
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
								ProbeHandler: corev1.ProbeHandler{
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
								PreStop: &corev1.LifecycleHandler{
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
					TerminationGracePeriodSeconds: pointer.Int64(900),
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
		sts.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Int32(*sts.Spec.Replicas)
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

func MakePodDisruptionBudget(c *scyllav1.ScyllaCluster) *policyv1.PodDisruptionBudget {
	maxUnavailable := intstr.FromInt(1)
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PodDisruptionBudgetName(c),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
			Labels: naming.ClusterLabels(c),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       metav1.SetAsLabelSelector(naming.ClusterLabels(c)),
		},
	}
}

func MakeIngresses(c *scyllav1.ScyllaCluster, services map[string]*corev1.Service) []*networkingv1.Ingress {
	// Don't create Ingresses if cluster isn't exposed.
	if c.Spec.ExposeOptions == nil {
		return nil
	}

	type params struct {
		ingressNameSuffix     string
		portName              string
		protocolSubdomainFunc func(string) string
		memberSubdomainFunc   func(string, string) string
		ingressOptions        *scyllav1.IngressOptions
	}
	var ingressParams []params

	if c.Spec.ExposeOptions.CQL != nil && isIngressEnabled(c.Spec.ExposeOptions.CQL.Ingress) {
		ingressParams = append(ingressParams, params{
			ingressNameSuffix:     "cql",
			portName:              portNameCQLSSL,
			protocolSubdomainFunc: naming.GetCQLProtocolSubDomain,
			memberSubdomainFunc:   naming.GetCQLHostIDSubDomain,
			ingressOptions:        c.Spec.ExposeOptions.CQL.Ingress,
		})
	}

	var ingresses []*networkingv1.Ingress

	for _, ip := range ingressParams {
		for _, service := range services {
			var hosts []string
			labels := naming.ClusterLabels(c)

			switch naming.ScyllaServiceType(service.Labels[naming.ScyllaServiceTypeLabel]) {
			case naming.ScyllaServiceTypeIdentity:
				for _, domain := range c.Spec.DNSDomains {
					hosts = append(hosts, ip.protocolSubdomainFunc(domain))
				}
				labels[naming.ScyllaIngressTypeLabel] = string(naming.ScyllaIngressTypeAnyNode)

			case naming.ScyllaServiceTypeMember:
				hostID, ok := service.Annotations[naming.HostIDAnnotation]
				if !ok {
					klog.V(4).Infof("Service %q is missing HostID annotation, postponing Ingress creation until it's available", naming.ObjRef(service))
					continue
				}

				if len(hostID) == 0 {
					klog.Warningf("Can't create Ingress for Service %s because it has unexpected empty HostID annotation", klog.KObj(service))
					continue
				}

				for _, domain := range c.Spec.DNSDomains {
					hosts = append(hosts, ip.memberSubdomainFunc(hostID, domain))
				}
				labels[naming.ScyllaIngressTypeLabel] = string(naming.ScyllaIngressTypeNode)

			default:
				klog.Warningf("Unsupported Scylla service type %q, not creating Ingress for it", service.Labels[naming.ScyllaServiceTypeLabel])
				continue
			}

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        fmt.Sprintf("%s-%s", service.Name, ip.ingressNameSuffix),
					Namespace:   c.Namespace,
					Labels:      labels,
					Annotations: ip.ingressOptions.Annotations,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: pointer.String(ip.ingressOptions.IngressClassName),
				},
			}

			pathPrefix := networkingv1.PathTypePrefix
			for _, host := range hosts {
				ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: service.Name,
											Port: networkingv1.ServiceBackendPort{
												Name: ip.portName,
											},
										},
									},
								},
							},
						},
					},
				})
			}

			ingresses = append(ingresses, ingress)
		}
	}

	sort.Slice(ingresses, func(i, j int) bool {
		return ingresses[i].GetName() < ingresses[j].GetName()
	})

	return ingresses
}

func isIngressEnabled(ingressOptions *scyllav1.IngressOptions) bool {
	if ingressOptions == nil {
		return false
	}
	return ingressOptions.Disabled == nil || !*ingressOptions.Disabled
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
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
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

func MakeServiceAccount(sc *scyllav1.ScyllaCluster) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.MemberServiceAccountNameForScyllaCluster(sc.Name),
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
			Labels: naming.ClusterLabels(sc),
		},
	}
}

func MakeRoleBinding(sc *scyllav1.ScyllaCluster) *rbacv1.RoleBinding {
	saName := naming.MemberServiceAccountNameForScyllaCluster(sc.Name)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
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

func MakeJobs(sc *scyllav1.ScyllaCluster, services map[string]*corev1.Service, image string) ([]*batchv1.Job, []metav1.Condition) {
	var jobs []*batchv1.Job
	var progressingConditions []metav1.Condition

	for _, rack := range sc.Spec.Datacenter.Racks {
		for i := int32(0); i < rack.Members; i++ {
			svcName := naming.MemberServiceName(rack, sc, int(i))
			svc, ok := services[svcName]
			if !ok {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForService",
					Message:            fmt.Sprintf("Waiting for Service %q", naming.ManualRef(sc.Namespace, svcName)),
					ObservedGeneration: sc.Generation,
				})
				continue
			}

			currentTokenRingHash, ok := svc.Annotations[naming.CurrentTokenRingHashAnnotation]
			if !ok {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForServiceState",
					Message:            fmt.Sprintf("Service %q is missing current token ring hash annotation", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				continue
			}

			if len(currentTokenRingHash) == 0 {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "UnexpectedServiceState",
					Message:            fmt.Sprintf("Service %q has unexpected empty current token ring hash annotation, can't create cleanup Job", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				klog.Warningf("Can't create cleanup Job for Service %s because it has unexpected empty current token ring hash annotation", klog.KObj(svc))
				continue
			}

			lastCleanedUpTokenRingHash, ok := svc.Annotations[naming.LastCleanedUpTokenRingHashAnnotation]
			if !ok {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForServiceState",
					Message:            fmt.Sprintf("Service %q is missing last cleaned up token ring hash annotation", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				continue
			}

			if len(lastCleanedUpTokenRingHash) == 0 {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "UnexpectedServiceState",
					Message:            fmt.Sprintf("Service %q has unexpected empty last cleaned up token ring hash annotation, can't create cleanup Job", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				klog.Warningf("Can't create cleanup Job for Service %s because it has unexpected empty last cleaned up token ring hash annotation", klog.KObj(svc))
				continue
			}

			if currentTokenRingHash == lastCleanedUpTokenRingHash {
				klog.V(4).Infof("Node %q already cleaned up", naming.ObjRef(svc))
				continue
			}

			klog.InfoS("Node requires a cleanup", "Node", naming.ObjRef(svc), "CurrentHash", currentTokenRingHash, "LastCleanedUpHash", lastCleanedUpTokenRingHash)

			jobLabels := map[string]string{
				naming.ClusterNameLabel: sc.Name,
				naming.NodeJobLabel:     svcName,
				naming.NodeJobTypeLabel: string(naming.JobTypeCleanup),
			}
			annotations := map[string]string{
				naming.CleanupJobTokenRingHashAnnotation: currentTokenRingHash,
			}

			jobs = append(jobs, &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      naming.CleanupJobForService(svc.Name),
					Namespace: sc.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
					},
					Labels:      jobLabels,
					Annotations: annotations,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      jobLabels,
							Annotations: annotations,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:            naming.CleanupContainerName,
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Args: []string{
										"cleanup-job",
										"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
										fmt.Sprintf("--node-address=%s", fmt.Sprintf("%s.%s.svc", svcName, sc.Namespace)),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "scylla-manager-agent-token",
											ReadOnly:  true,
											MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
											SubPath:   naming.ScyllaAgentAuthTokenFileName,
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "scylla-manager-agent-token",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.AgentAuthTokenSecretName(sc.Name),
										},
									},
								},
							},
						},
					},
				},
			})
		}
	}

	return jobs, progressingConditions
}
