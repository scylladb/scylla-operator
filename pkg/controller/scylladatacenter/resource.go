// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
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
	scyllaConfigVolumeName         = "scylla-config-volume"
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

func IdentityService(sd *scyllav1alpha1.ScyllaDatacenter) *corev1.Service {
	labels := naming.ClusterLabels(sd)
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeIdentity)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.HeadlessServiceNameForDatacenter(sd),
			Namespace: sd.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: naming.ClusterLabels(sd),
			Ports:    servicePorts(sd),
		},
	}
}

func MemberService(sd *scyllav1alpha1.ScyllaDatacenter, rackName, name string, oldService *corev1.Service) *corev1.Service {
	labels := naming.ClusterLabels(sd)
	labels[naming.DatacenterNameLabel] = sd.Spec.DatacenterName
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
		rackStatus, ok := sd.Status.Racks[rackName]
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
			Namespace: sd.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 naming.StatefulSetPodLabel(name),
			Ports:                    servicePorts(sd),
			PublishNotReadyAddresses: true,
		},
	}
}

func servicePorts(sd *scyllav1alpha1.ScyllaDatacenter) []corev1.ServicePort {
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
	if sd.Spec.Scylla.AlternatorOptions != nil && sd.Spec.Scylla.AlternatorOptions.Enabled != nil && *sd.Spec.Scylla.AlternatorOptions.Enabled {
		ports = append(ports, corev1.ServicePort{
			Name: portNameAlternator,
			Port: 8000,
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
func StatefulSetForRack(r scyllav1alpha1.RackSpec, sd *scyllav1alpha1.ScyllaDatacenter, existingSts *appsv1.StatefulSet, sidecarImage string) (*appsv1.StatefulSet, error) {
	matchLabels := naming.RackLabels(r, sd)
	rackLabels := naming.RackLabels(r, sd)

	// TODO: figure out from registry
	imageParts := strings.Split(sd.Spec.Scylla.Image, ":")
	if len(imageParts) == 2 {
		rackLabels[naming.ScyllaVersionLabel] = imageParts[1]
	}

	placement := r.Placement
	if placement == nil {
		placement = &scyllav1alpha1.Placement{}
	}
	opt := true

	var dnsPolicy = corev1.DNSClusterFirstWithHostNet
	if sd.Spec.Network != nil {
		dnsPolicy = sd.Spec.Network.DNSPolicy
	}

	scyllaResources := r.Scylla.Resources

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.StatefulSetNameForRack(r, sd),
			Namespace: sd.Namespace,
			Labels:    rackLabels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: r.Nodes,
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.HeadlessServiceNameForDatacenter(sd),
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
					DNSPolicy: dnsPolicy,
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
										SecretName: naming.AgentAuthTokenSecretName(sd.Name),
									},
								},
							},
						}

						if r.Scylla != nil && r.Scylla.CustomConfigMapRef != nil {
							volumes = append(volumes, corev1.Volume{
								Name: scyllaConfigVolumeName,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: r.Scylla.CustomConfigMapRef.Name,
										},
										Optional: pointer.Bool(true),
									},
								},
							})
						}

						if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
							volumes = append(volumes, []corev1.Volume{
								{
									Name: scylladbServingCertsVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalServingCertName(sd.Name),
										},
									},
								},
								{
									Name: scylladbClientCAVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalClientCAName(sd.Name),
										},
									},
								},
								{
									Name: scylladbUserAdminVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalUserAdminCertName(sd.Name),
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
							Image:           sd.Spec.Scylla.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           containerPorts(sd),
							// TODO: unprivileged entrypoint
							Command: []string{
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
							Resources: *scyllaResources,
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
										Name:      "scylla-client-config-volume",
										MountPath: naming.ScyllaClientConfigDirName,
										ReadOnly:  true,
									},
								}
								if r.Scylla != nil && r.Scylla.CustomConfigMapRef != nil {
									mounts = append(mounts, corev1.VolumeMount{
										Name:      scyllaConfigVolumeName,
										MountPath: naming.ScyllaConfigDirName,
										ReadOnly:  true,
									})
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
					ServiceAccountName: naming.MemberServiceAccountNameForScyllaDatacenter(sd.Name),
					Affinity: &corev1.Affinity{
						NodeAffinity:    placement.NodeAffinity,
						PodAffinity:     placement.PodAffinity,
						PodAntiAffinity: placement.PodAntiAffinity,
					},
					ImagePullSecrets:              sd.Spec.ImagePullSecrets,
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
						StorageClassName: r.Scylla.Storage.StorageClassName,
						Resources:        r.Scylla.Storage.Resources,
					},
				},
			},
		},
	}

	if len(sd.Spec.ForceRedeploymentReason) != 0 {
		sts.Spec.Template.Annotations[naming.ForceRedeploymentReasonAnnotation] = sd.Spec.ForceRedeploymentReason
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

	for _, volume := range r.UnsupportedVolumes {
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, volume)
	}

	if r.Scylla != nil {
		for _, vm := range r.Scylla.UnsupportedVolumeMounts {
			sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(sts.Spec.Template.Spec.Containers[0].VolumeMounts, vm)
		}
	}
	if sd.Spec.ScyllaManagerAgent != nil {
		cnt, volumes := agentContainer(sd.Spec.ScyllaManagerAgent.Image, r.ScyllaManagerAgent.Resources, r.ScyllaManagerAgent.CustomConfigSecretRef, r.ScyllaManagerAgent.UnsupportedVolumeMounts)
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, cnt)
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, volumes...)
	}

	if err := applyScyllaClusterV1Annotations(sd, sts, r, sidecarImage); err != nil {
		return nil, fmt.Errorf("can't apply scyllav1.ScyllaCluster properties from annotation: %w", err)
	}

	return sts, nil
}

func containerPorts(sd *scyllav1alpha1.ScyllaDatacenter) []corev1.ContainerPort {
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

	if sd.Spec.Scylla.AlternatorOptions != nil && sd.Spec.Scylla.AlternatorOptions.Enabled != nil && *sd.Spec.Scylla.AlternatorOptions.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "alternator",
			ContainerPort: 8000,
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

func agentContainer(image string, resources *corev1.ResourceRequirements, customSecretRef *corev1.LocalObjectReference, volumeMounts []corev1.VolumeMount) (corev1.Container, []corev1.Volume) {
	var volumes []corev1.Volume
	cnt := corev1.Container{
		Name:            naming.ScyllaManagerAgentContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"-c",
			naming.ScyllaAgentConfigDefaultFile,
			"-c",
			path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentAuthTokenFileName),
		},
		Resources: *resources,
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
				Name:      scyllaAgentAuthTokenVolumeName,
				MountPath: path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentAuthTokenFileName),
				SubPath:   naming.ScyllaAgentAuthTokenFileName,
				ReadOnly:  true,
			},
		},
	}

	if customSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: scyllaAgentConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: customSecretRef.Name,
					Optional:   pointer.Bool(true),
				},
			},
		})

		cnt.VolumeMounts = append(cnt.VolumeMounts, corev1.VolumeMount{
			Name:      scyllaAgentConfigVolumeName,
			MountPath: path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentConfigFileName),
			SubPath:   naming.ScyllaAgentConfigFileName,
			ReadOnly:  true,
		})

		cnt.Args = append(cnt.Args, "-c", path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentConfigFileName))
	}
	cnt.VolumeMounts = append(cnt.VolumeMounts, volumeMounts...)

	return cnt, volumes
}

func MakePodDisruptionBudget(sd *scyllav1alpha1.ScyllaDatacenter) *policyv1.PodDisruptionBudget {
	maxUnavailable := intstr.FromInt(1)
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PodDisruptionBudgetName(sd),
			Namespace: sd.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
			Labels: naming.ClusterLabels(sd),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       metav1.SetAsLabelSelector(naming.ClusterLabels(sd)),
		},
	}
}

func MakeIngresses(sd *scyllav1alpha1.ScyllaDatacenter, services map[string]*corev1.Service) []*networkingv1.Ingress {
	// Don't create Ingresses if cluster isn't exposed.
	if sd.Spec.ExposeOptions == nil {
		return nil
	}

	type params struct {
		ingressNameSuffix     string
		portName              string
		identitySubdomainFunc func(string) string
		memberSubdomainFunc   func(string, string) string
		ingressOptions        *scyllav1alpha1.IngressOptions
	}
	var ingressParams []params

	if sd.Spec.ExposeOptions.CQL != nil && isIngressEnabled(sd.Spec.ExposeOptions.CQL.Ingress) {
		ingressParams = append(ingressParams, params{
			ingressNameSuffix:     "cql",
			portName:              portNameCQLSSL,
			identitySubdomainFunc: naming.GetCQLProtocolSubDomain,
			memberSubdomainFunc:   naming.GetCQLHostIDSubDomain,
			ingressOptions:        sd.Spec.ExposeOptions.CQL.Ingress,
		})
	}

	var ingresses []*networkingv1.Ingress

	for _, ip := range ingressParams {
		for _, service := range services {
			var hosts []string
			labels := naming.ClusterLabels(sd)

			switch naming.ScyllaServiceType(service.Labels[naming.ScyllaServiceTypeLabel]) {
			case naming.ScyllaServiceTypeIdentity:
				for _, domain := range sd.Spec.DNSDomains {
					hosts = append(hosts, ip.identitySubdomainFunc(domain))
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

				for _, domain := range sd.Spec.DNSDomains {
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
					Namespace:   sd.Namespace,
					Labels:      labels,
					Annotations: ip.ingressOptions.Annotations,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
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

func isIngressEnabled(ingressOptions *scyllav1alpha1.IngressOptions) bool {
	if ingressOptions == nil {
		return false
	}
	return ingressOptions.Disabled == nil || !*ingressOptions.Disabled
}

func MakeAgentAuthTokenSecret(sd *scyllav1alpha1.ScyllaDatacenter, authToken string) (*corev1.Secret, error) {
	data, err := helpers.GetAgentAuthTokenConfig(authToken)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.AgentAuthTokenSecretName(sd.Name),
			Namespace: sd.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
			Labels: naming.ClusterLabels(sd),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			naming.ScyllaAgentAuthTokenFileName: data,
		},
	}, nil
}

func MakeServiceAccount(sd *scyllav1alpha1.ScyllaDatacenter) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.MemberServiceAccountNameForScyllaDatacenter(sd.Name),
			Namespace: sd.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
			Labels: naming.ClusterLabels(sd),
		},
	}
}

func MakeRoleBinding(sd *scyllav1alpha1.ScyllaDatacenter) *rbacv1.RoleBinding {
	saName := naming.MemberServiceAccountNameForScyllaDatacenter(sd.Name)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: sd.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sd, scyllaDatacenterControllerGVK),
			},
			Labels: naming.ClusterLabels(sd),
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: sd.Namespace,
				Name:      saName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     naming.ScyllaDatacenterMemberClusterRoleName,
		},
	}
}

func applyScyllaClusterV1Annotations(sd *scyllav1alpha1.ScyllaDatacenter, sts *appsv1.StatefulSet, r scyllav1alpha1.RackSpec, sidecarImage string) error {
	marshalledSCV1, ok := sd.Annotations[naming.ScyllaClusterV1Annotation]
	if !ok {
		return nil
	}

	scv1 := &scyllav1.ScyllaCluster{}
	if err := json.NewDecoder(bytes.NewBufferString(marshalledSCV1)).Decode(scv1); err != nil {
		return fmt.Errorf("can't decode scyllav1.ScyllaCluster from annotation: %w", err)
	}

	sts.Spec.Template.Spec.HostNetwork = scv1.Spec.Network.HostNetworking

	if len(scv1.Spec.Sysctls) > 0 {
		if container := sysctlInitContainer(scv1.Spec.Sysctls, sidecarImage); container != nil {
			sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, *container)
		}
	}

	return nil
}

func stringOrDefault(str, def string) string {
	if str != "" {
		return str
	}
	return def
}
