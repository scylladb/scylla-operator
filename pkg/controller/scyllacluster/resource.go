package scyllacluster

import (
	"fmt"
	"maps"
	"path"
	"sort"
	"strconv"
	"strings"

	scylladbassets "github.com/scylladb/scylla-operator/assets/scylladb"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
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
)

const (
	scyllaAgentConfigVolumeName              = "scylla-agent-config-volume"
	scyllaAgentAuthTokenVolumeName           = "scylla-agent-auth-token-volume"
	scylladbServingCertsVolumeName           = "scylladb-serving-certs"
	scylladbClientCAVolumeName               = "scylladb-client-ca"
	scylladbUserAdminVolumeName              = "scylladb-user-admin"
	scylladbAlternatorServingCertsVolumeName = "scylladb-alternator-serving-certs"
)

const (
	rootUID int64 = 0
	rootGID int64 = 0
)

const (
	portNameCQL              = "cql"
	portNameCQLSSL           = "cql-ssl"
	portNameCQLShardAware    = "cql-shard-aware"
	portNameCQLSSLShardAware = "cql-ssl-shard-aware"
	portNameThrift           = "thrift"

	alternatorInsecurePort     = 8000
	alternatorInsecurePortName = "alternator"
	alternatorTLSPort          = 8043
	alternatorTLSPortName      = "alternator-tls"
)

func IdentityService(c *scyllav1.ScyllaCluster) *corev1.Service {
	svcLabels := map[string]string{}
	maps.Copy(svcLabels, c.Labels)
	maps.Copy(svcLabels, naming.ClusterLabels(c))
	svcLabels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeIdentity)

	svcAnnotations := map[string]string{}
	maps.Copy(svcAnnotations, c.Annotations)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.IdentityServiceName(c),
			Namespace:   c.Namespace,
			Labels:      svcLabels,
			Annotations: svcAnnotations,
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

func MemberService(sc *scyllav1.ScyllaCluster, rackName, name string, oldService *corev1.Service, jobs map[string]*batchv1.Job) (*corev1.Service, error) {
	svcLabels := map[string]string{}

	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService.Labels != nil {
		maps.Copy(svcLabels, sc.Spec.ExposeOptions.NodeService.Labels)
	} else {
		maps.Copy(svcLabels, sc.Labels)
	}

	maps.Copy(svcLabels, naming.ClusterLabels(sc))
	svcLabels[naming.DatacenterNameLabel] = sc.Spec.Datacenter.Name
	svcLabels[naming.RackNameLabel] = rackName
	svcLabels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeMember)

	var replaceAddr string
	var hasReplaceLabel bool
	if oldService != nil {
		replaceAddr, hasReplaceLabel = oldService.Labels[naming.ReplaceLabel]
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

	svcAnnotations := map[string]string{}
	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService.Annotations != nil {
		maps.Copy(svcAnnotations, sc.Spec.ExposeOptions.NodeService.Annotations)
	} else {
		maps.Copy(svcAnnotations, sc.Annotations)
	}

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

	svc := &corev1.Service{
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

	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService != nil {
		ns := sc.Spec.ExposeOptions.NodeService

		switch ns.Type {
		case scyllav1.NodeServiceTypeClusterIP:
			svc.Spec.Type = corev1.ServiceTypeClusterIP
		case scyllav1.NodeServiceTypeLoadBalancer:
			svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		case scyllav1.NodeServiceTypeHeadless:
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			svc.Spec.ClusterIP = corev1.ClusterIPNone
		default:
			return nil, fmt.Errorf("unsupported node service type %q", ns.Type)
		}

		svc.Annotations = helpers.MergeMaps(svcAnnotations, ns.Annotations)
		svc.Spec.InternalTrafficPolicy = copyReferencedValue(ns.InternalTrafficPolicy)
		svc.Spec.AllocateLoadBalancerNodePorts = copyReferencedValue(ns.AllocateLoadBalancerNodePorts)
		svc.Spec.LoadBalancerClass = copyReferencedValue(ns.LoadBalancerClass)
		svc.Spec.ExternalTrafficPolicy = getValueOrDefault(ns.ExternalTrafficPolicy, "")
	}

	return svc, nil
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
			Name: portNameThrift,
			Port: 9160,
		},
	}

	if cluster.Spec.Alternator != nil {
		ports = append(ports, corev1.ServicePort{
			Name: alternatorTLSPortName,
			Port: alternatorTLSPort,
		})

		enableHTTP := cluster.Spec.Alternator.InsecureEnableHTTP
		if cluster.Spec.Alternator.Port != 0 || (enableHTTP != nil && *enableHTTP) {
			insecurePort := int32(alternatorInsecurePort)
			if cluster.Spec.Alternator.Port != 0 {
				insecurePort = cluster.Spec.Alternator.Port
			}

			ports = append(ports, corev1.ServicePort{
				Name: alternatorInsecurePortName,
				Port: insecurePort,
			})
		}
	}

	return ports
}

// StatefulSetForRack make a StatefulSet for the rack.
// existingSts may be nil if it doesn't exist yet.
func StatefulSetForRack(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, existingSts *appsv1.StatefulSet, sidecarImage string, rackOrdinal int, inputsHash string) (*appsv1.StatefulSet, error) {
	selectorLabels, err := naming.RackSelectorLabels(r, c)
	if err != nil {
		return nil, fmt.Errorf("can't get selector labels: %w", err)
	}

	requiredLabels := map[string]string{}
	requiredLabels[naming.RackOrdinalLabel] = strconv.Itoa(rackOrdinal)
	requiredLabels[naming.ScyllaVersionLabel] = c.Spec.Version
	maps.Copy(requiredLabels, selectorLabels)

	rackLabels := map[string]string{}
	maps.Copy(rackLabels, c.Labels)
	maps.Copy(rackLabels, requiredLabels)

	rackTemplateLabels := map[string]string{}
	if c.Spec.PodMetadata != nil && c.Spec.PodMetadata.Labels != nil {
		maps.Copy(rackTemplateLabels, c.Spec.PodMetadata.Labels)
	} else {
		maps.Copy(rackTemplateLabels, c.Labels)
	}
	maps.Copy(rackTemplateLabels, requiredLabels)

	rackAnnotations := map[string]string{}
	maps.Copy(rackAnnotations, c.Annotations)

	rackTemplateAnnotations := map[string]string{}
	if c.Spec.PodMetadata != nil && c.Spec.PodMetadata.Annotations != nil {
		maps.Copy(rackTemplateAnnotations, c.Spec.PodMetadata.Annotations)
	} else {
		maps.Copy(rackTemplateAnnotations, c.Annotations)
	}
	rackTemplateAnnotations[naming.PrometheusScrapeAnnotation] = naming.LabelValueTrue
	rackTemplateAnnotations[naming.PrometheusPortAnnotation] = "9180"
	rackTemplateAnnotations[naming.InputsHashAnnotation] = inputsHash

	// VolumeClaims are not allowed to be edited by StatufulSet validation,
	// which means we have to keep them static.
	// ScyllaClusters forbid rack storage changes, but we have to be careful
	// when defaulting the values from other places.

	var existingDataPVCTemplate *corev1.PersistentVolumeClaim
	if existingSts != nil {
		pvc, _, ok := slices.Find(existingSts.Spec.VolumeClaimTemplates, func(pvc corev1.PersistentVolumeClaim) bool {
			return pvc.Name == naming.PVCTemplateName
		})
		if !ok {
			return nil, fmt.Errorf("can't find data PVC template %q in existing %q StatefulSet spec", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		existingDataPVCTemplate = &pvc
	}

	dataVolumeClaimLabels := map[string]string{}
	if r.Storage.Metadata != nil && r.Storage.Metadata.Labels != nil {
		maps.Copy(dataVolumeClaimLabels, r.Storage.Metadata.Labels)
	} else if existingSts == nil {
		maps.Copy(dataVolumeClaimLabels, c.Labels)
	} else {
		if existingDataPVCTemplate == nil {
			return nil, fmt.Errorf("data PVC template %q in existing %q StatefulSet spec is missing", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		maps.Copy(dataVolumeClaimLabels, existingDataPVCTemplate.Labels)
	}
	maps.Copy(dataVolumeClaimLabels, selectorLabels)

	dataVolumeClaimAnnotations := map[string]string{}
	if r.Storage.Metadata != nil && r.Storage.Metadata.Annotations != nil {
		maps.Copy(dataVolumeClaimAnnotations, r.Storage.Metadata.Annotations)
	} else if existingSts == nil {
		maps.Copy(dataVolumeClaimAnnotations, c.Annotations)
	} else {
		if existingDataPVCTemplate == nil {
			return nil, fmt.Errorf("data PVC template %q in existing %q StatefulSet spec is missing", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		maps.Copy(dataVolumeClaimAnnotations, existingDataPVCTemplate.Annotations)
	}

	placement := r.Placement
	if placement == nil {
		placement = &scyllav1.PlacementSpec{}
	}
	opt := true

	storageCapacity, err := resource.ParseQuantity(r.Storage.Capacity)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %v", r.Storage.Capacity, err)
	}

	// Assume kube-proxy notices readiness change and reconcile Endpoints within this period
	kubeProxyEndpointsSyncPeriodSeconds := 5
	loadBalancerSyncPeriodSeconds := 60

	readinessFailureThreshold := 1
	readinessPeriodSeconds := 10
	minReadySeconds := kubeProxyEndpointsSyncPeriodSeconds
	minTerminationGracePeriodSeconds := readinessFailureThreshold*readinessPeriodSeconds + kubeProxyEndpointsSyncPeriodSeconds

	if c.Spec.ExposeOptions != nil && c.Spec.ExposeOptions.NodeService != nil && c.Spec.ExposeOptions.NodeService.Type == scyllav1.NodeServiceTypeLoadBalancer {
		// Any "upstream" Load Balancer should notice Endpoint readiness change within this period.
		minTerminationGracePeriodSeconds = loadBalancerSyncPeriodSeconds
		minReadySeconds = loadBalancerSyncPeriodSeconds
	}

	if c.Spec.MinTerminationGracePeriodSeconds != nil {
		minTerminationGracePeriodSeconds = int(*c.Spec.MinTerminationGracePeriodSeconds)
	}
	if c.Spec.MinReadySeconds != nil {
		minReadySeconds = int(*c.Spec.MinReadySeconds)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.StatefulSetNameForRack(r, c),
			Namespace:   c.Namespace,
			Labels:      rackLabels,
			Annotations: rackAnnotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Ptr(r.Members),
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.IdentityServiceName(c),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: pointer.Ptr(int32(0)),
				},
			},
			MinReadySeconds: int32(minReadySeconds),
			// Template for Pods
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      rackTemplateLabels,
					Annotations: rackTemplateAnnotations,
				},
				Spec: corev1.PodSpec{
					HostNetwork: c.Spec.Network.HostNetworking,
					DNSPolicy:   c.Spec.Network.GetDNSPolicy(),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  pointer.Ptr(rootUID),
						RunAsGroup: pointer.Ptr(rootGID),
					},
					ReadinessGates: c.Spec.ReadinessGates,
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
								Name: "scylladb-managed-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: naming.GetScyllaDBManagedConfigCMName(c.Name),
										},
										Optional: pointer.Ptr(false),
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
						if c.Spec.Alternator != nil {
							volumes = append(volumes, corev1.Volume{
								Name: scylladbAlternatorServingCertsVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: naming.GetScyllaClusterAlternatorLocalServingCertName(c.Name),
										Optional:   pointer.Ptr(false),
									},
								},
							})
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
									fmt.Sprintf("--nodes-broadcast-address-type=%s", func() scyllav1.BroadcastAddressType {
										if c.Spec.ExposeOptions != nil && c.Spec.ExposeOptions.BroadcastOptions != nil {
											return c.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
										}
										return scyllav1.BroadcastAddressTypeServiceClusterIP
									}()),
									fmt.Sprintf("--clients-broadcast-address-type=%s", func() scyllav1.BroadcastAddressType {
										if c.Spec.ExposeOptions != nil && c.Spec.ExposeOptions.BroadcastOptions != nil {
											return c.Spec.ExposeOptions.BroadcastOptions.Clients.Type
										}
										return scyllav1.BroadcastAddressTypeServiceClusterIP
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
										Name:      "scylladb-managed-config",
										MountPath: naming.ScyllaDBManagedConfigDir,
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

								if c.Spec.Alternator != nil {
									mounts = append(mounts, corev1.VolumeMount{
										Name:      scylladbAlternatorServingCertsVolumeName,
										MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs",
										ReadOnly:  true,
									})
								}

								return mounts
							}(),
							// Add CAP_SYS_NICE as instructed by scylla logs
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  pointer.Ptr(rootUID),
								RunAsGroup: pointer.Ptr(rootGID),
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
										Port: intstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
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
										Port: intstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
										Path: naming.LivenessProbePath,
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								// TODO: Lower the timeout when we fix probes. We have temporarily changed them from 5s
								// to 30s to survive cluster overload.
								// Relevant issue: https://github.com/scylladb/scylla-operator/issues/844
								TimeoutSeconds:   int32(30),
								FailureThreshold: int32(readinessFailureThreshold),
								PeriodSeconds:    int32(readinessPeriodSeconds),
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
										Path: naming.ReadinessProbePath,
									},
								},
							},
							// Before a Scylla Pod is stopped, execute nodetool drain to
							// flush the memtable to disk, finish existing requests and stop listening for connections.
							// Sleep is required to give chance to Load Balancers to acknowledge Pod going down with their
							// probes.
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
											fmt.Sprintf("nodetool drain & sleep %d & wait", minTerminationGracePeriodSeconds),
										},
									},
								},
							},
						},
						{
							// ScyllaDB doesn't provide readiness or liveness probe,
							// so we use our own probe sidecar to expose such endpoints.
							Name:            "scylladb-api-status-probe",
							Image:           sidecarImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/scylla-operator",
								"serve-probes",
								"scylladb-api-status",
								fmt.Sprintf("--port=%d", naming.ScyllaDBAPIStatusProbePort),
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
					},
					ServiceAccountName: naming.MemberServiceAccountNameForScyllaCluster(c.Name),
					Affinity: &corev1.Affinity{
						NodeAffinity:    placement.NodeAffinity,
						PodAffinity:     placement.PodAffinity,
						PodAntiAffinity: placement.PodAntiAffinity,
					},
					ImagePullSecrets:              c.Spec.ImagePullSecrets,
					TerminationGracePeriodSeconds: pointer.Ptr(int64(900)),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        naming.PVCTemplateName,
						Labels:      dataVolumeClaimLabels,
						Annotations: dataVolumeClaimAnnotations,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: r.Storage.StorageClassName,
						Resources: corev1.VolumeResourceRequirements{
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
		sts.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Ptr(*sts.Spec.Replicas)
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
	}

	if c.Spec.Alternator != nil {
		ports = append(ports, corev1.ContainerPort{
			Name:          alternatorTLSPortName,
			ContainerPort: alternatorTLSPort,
		})

		enableHTTP := c.Spec.Alternator.InsecureEnableHTTP
		if c.Spec.Alternator.Port != 0 || (enableHTTP != nil && *enableHTTP) {
			insecurePort := int32(alternatorInsecurePort)
			if c.Spec.Alternator.Port != 0 {
				insecurePort = c.Spec.Alternator.Port
			}
			ports = append(ports, corev1.ContainerPort{
				Name:          alternatorInsecurePortName,
				ContainerPort: insecurePort,
			})
		}
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

	selectorLabels := naming.ClusterLabels(c)

	labels := map[string]string{}
	maps.Copy(labels, c.Labels)
	maps.Copy(labels, selectorLabels)

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PodDisruptionBudgetName(c),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
			Labels:      labels,
			Annotations: maps.Clone(c.Annotations),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       metav1.SetAsLabelSelector(selectorLabels),
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

			annotations := map[string]string{}
			if ip.ingressOptions.Annotations != nil {
				maps.Copy(annotations, ip.ingressOptions.Annotations)
			} else {
				maps.Copy(annotations, c.Annotations)
			}

			labels := map[string]string{}
			maps.Copy(labels, c.Labels)
			maps.Copy(labels, naming.ClusterLabels(c))

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
					Annotations: annotations,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: pointer.Ptr(ip.ingressOptions.IngressClassName),
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

	labels := map[string]string{}
	maps.Copy(labels, c.Labels)
	maps.Copy(labels, naming.ClusterLabels(c))

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.AgentAuthTokenSecretName(c.Name),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, scyllaClusterControllerGVK),
			},
			Labels:      labels,
			Annotations: maps.Clone(c.Annotations),
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

func getValueOrDefault[T any](v *T, def T) T {
	if v != nil {
		return *v
	}
	return def
}

func copyReferencedValue[T any](v *T) *T {
	if v != nil {
		return pointer.Ptr(*v)
	}
	return nil
}

func MakeServiceAccount(sc *scyllav1.ScyllaCluster) *corev1.ServiceAccount {
	labels := map[string]string{}
	maps.Copy(labels, sc.Labels)
	maps.Copy(labels, naming.ClusterLabels(sc))

	annotations := map[string]string{}
	maps.Copy(annotations, sc.Annotations)

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.MemberServiceAccountNameForScyllaCluster(sc.Name),
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func MakeRoleBinding(sc *scyllav1.ScyllaCluster) *rbacv1.RoleBinding {
	saName := naming.MemberServiceAccountNameForScyllaCluster(sc.Name)

	labels := map[string]string{}
	maps.Copy(labels, sc.Labels)
	maps.Copy(labels, naming.ClusterLabels(sc))

	annotations := map[string]string{}
	maps.Copy(annotations, sc.Annotations)

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
			Labels:      labels,
			Annotations: annotations,
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

			jobLabels := map[string]string{}
			maps.Copy(jobLabels, sc.Labels)
			maps.Copy(jobLabels, map[string]string{
				naming.ClusterNameLabel: sc.Name,
				naming.NodeJobLabel:     svcName,
				naming.NodeJobTypeLabel: string(naming.JobTypeCleanup),
			})

			annotations := map[string]string{}
			maps.Copy(annotations, sc.Annotations)
			annotations[naming.CleanupJobTokenRingHashAnnotation] = currentTokenRingHash

			var tolerations []corev1.Toleration
			var affinity *corev1.Affinity
			if rack.Placement != nil {
				tolerations = rack.Placement.Tolerations
				affinity = &corev1.Affinity{
					NodeAffinity:    rack.Placement.NodeAffinity,
					PodAffinity:     rack.Placement.PodAffinity,
					PodAntiAffinity: rack.Placement.PodAntiAffinity,
				}
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
					Selector:       nil,
					ManualSelector: pointer.Ptr(false),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      jobLabels,
							Annotations: annotations,
						},
						Spec: corev1.PodSpec{
							Tolerations:   tolerations,
							Affinity:      affinity,
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

func MakeManagedScyllaDBConfig(sc *scyllav1.ScyllaCluster) (*corev1.ConfigMap, error) {
	cm, _, err := scylladbassets.ScyllaDBManagedConfigTemplate.RenderObject(
		map[string]any{
			"Namespace":         sc.Namespace,
			"Name":              naming.GetScyllaDBManagedConfigCMName(sc.Name),
			"ClusterName":       sc.Name,
			"ManagedConfigName": naming.ScyllaDBManagedConfigName,
			"EnableTLS":         utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates),
			"Spec":              sc.Spec,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't render managed scylladb config: %w", err)
	}

	cm.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         scyllaClusterControllerGVK.GroupVersion().String(),
			Kind:               scyllaClusterControllerGVK.Kind,
			Name:               sc.Name,
			UID:                sc.UID,
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
		},
	})

	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	maps.Copy(cm.Labels, sc.Labels)

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	maps.Copy(cm.Annotations, sc.Annotations)

	return cm, nil
}
