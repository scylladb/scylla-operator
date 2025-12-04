package scylladbdatacenter

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"sort"
	"strconv"
	"strings"

	scylladbassets "github.com/scylladb/scylla-operator/assets/scylladb"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/pkg/semver"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilintstr "k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	scyllaAgentConfigVolumeName              = "scylla-agent-config-volume"
	scyllaManagedAgentConfigVolumeName       = "scylla-managed-agent-config-volume"
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

var (
	// Annotation keys excluded from propagation to underlying resources.
	nonPropagatedAnnotationKeys = []string{
		// As ScyllaDBDatacenter may be a managed object (when a user is using scyllav1.ScyllaCluster API), its managed
		// hash shouldn't be propagated into dependency objects to avoid triggering unnecessary double rollouts.
		naming.ManagedHash,
		// This annotation is set by controllers to override the name of the internal ScyllaDBManagerClusterRegistration
		// object created on migration from scyllav1.ScyllaCluster object. Setting it shouldn't trigger a rollout as it
		// doesn't affect the ScyllaDB cluster itself.
		naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation,
	}

	// Label keys excluded from propagation to underlying resources.
	nonPropagatedLabelKeys = []string{
		// This label is used to designate the object to be registered with the global instance of ScyllaDBManager.
		// Setting it shouldn't trigger a rollout as it doesn't affect the ScyllaDB cluster itself.
		naming.GlobalScyllaDBManagerRegistrationLabel,
	}
)

func IdentityService(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*corev1.Service, error) {
	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeIdentity)

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	servicePorts, err := getServicePorts(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get service ports: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.IdentityServiceName(sdc),
			Namespace:   sdc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: naming.ClusterLabels(sdc),
			Ports:    servicePorts,
		},
	}

	// Configure IP families based on the datacenter configuration
	if len(sdc.Spec.IPFamilies) > 0 {
		// Use the explicit IP families list if provided (supports dual-stack)
		svc.Spec.IPFamilies = sdc.Spec.IPFamilies
		if sdc.Spec.IPFamilyPolicy != nil {
			svc.Spec.IPFamilyPolicy = sdc.Spec.IPFamilyPolicy
		}
	} else {
		ipFamily := sdc.Spec.GetIPFamily()
		if ipFamily == corev1.IPv6Protocol {
			svc.Spec.IPFamilies = []corev1.IPFamily{ipFamily}
			svc.Spec.IPFamilyPolicy = pointer.Ptr(corev1.IPFamilyPolicySingleStack)
		}
		// Note: When ipFamily is IPv4 (default), we rely on Kubernetes defaults
		// This supports backward compatibility and follows the principle of explicit IPv6 opt-in
	}

	return svc, nil
}

func MemberService(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName, name string, oldService *corev1.Service, jobs map[string]*batchv1.Job) (*corev1.Service, error) {
	labels := map[string]string{}

	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.NodeService.Labels != nil {
		maps.Copy(labels, sdc.Spec.ExposeOptions.NodeService.Labels)
	} else {
		sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
		maps.Copy(labels, sdcLabels)
	}

	maps.Copy(labels, naming.ClusterLabels(sdc))
	labels[naming.DatacenterNameLabel] = naming.GetScyllaDBDatacenterGossipDatacenterName(sdc)
	labels[naming.RackNameLabel] = rackName
	labels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeMember)

	annotations := map[string]string{}

	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.NodeService.Annotations != nil {
		maps.Copy(annotations, sdc.Spec.ExposeOptions.NodeService.Annotations)
	} else {
		sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)
		maps.Copy(annotations, sdcAnnotations)
	}

	if oldService != nil {
		_, hasLastCleanedUpRingHash := oldService.Annotations[naming.LastCleanedUpTokenRingHashAnnotation]
		currentTokenRingHash, hasCurrentRingHash := oldService.Annotations[naming.CurrentTokenRingHashAnnotation]
		if !hasLastCleanedUpRingHash && hasCurrentRingHash {
			annotations[naming.LastCleanedUpTokenRingHashAnnotation] = currentTokenRingHash
		}
	}

	cleanupJob, ok := jobs[naming.CleanupJobForService(name)]
	if ok {
		if len(cleanupJob.Annotations[naming.CleanupJobTokenRingHashAnnotation]) != 0 && cleanupJob.Status.CompletionTime != nil {
			annotations[naming.LastCleanedUpTokenRingHashAnnotation] = cleanupJob.Annotations[naming.CleanupJobTokenRingHashAnnotation]
		}
	}

	servicePorts, err := getServicePorts(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get service ports: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sdc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 naming.StatefulSetPodLabel(name),
			Ports:                    servicePorts,
			PublishNotReadyAddresses: true,
		},
	}

	// Configure IP families based on the datacenter configuration
	if len(sdc.Spec.IPFamilies) > 0 {
		// Use the explicit IP families list if provided (supports dual-stack)
		svc.Spec.IPFamilies = sdc.Spec.IPFamilies
		if sdc.Spec.IPFamilyPolicy != nil {
			svc.Spec.IPFamilyPolicy = sdc.Spec.IPFamilyPolicy
		}
	} else {
		ipFamily := sdc.Spec.GetIPFamily()
		if ipFamily == corev1.IPv6Protocol {
			svc.Spec.IPFamilies = []corev1.IPFamily{ipFamily}
			svc.Spec.IPFamilyPolicy = pointer.Ptr(corev1.IPFamilyPolicySingleStack)
		}
	}

	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.NodeService != nil {
		ns := sdc.Spec.ExposeOptions.NodeService

		switch ns.Type {
		case scyllav1alpha1.NodeServiceTypeClusterIP:
			svc.Spec.Type = corev1.ServiceTypeClusterIP
		case scyllav1alpha1.NodeServiceTypeLoadBalancer:
			svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		case scyllav1alpha1.NodeServiceTypeHeadless:
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			svc.Spec.ClusterIP = corev1.ClusterIPNone
		default:
			return nil, fmt.Errorf("unsupported node service type %q", ns.Type)
		}

		svc.Annotations = helpers.MergeMaps(annotations, ns.Annotations)
		svc.Spec.InternalTrafficPolicy = copyReferencedValue(ns.InternalTrafficPolicy)
		svc.Spec.AllocateLoadBalancerNodePorts = copyReferencedValue(ns.AllocateLoadBalancerNodePorts)
		svc.Spec.LoadBalancerClass = copyReferencedValue(ns.LoadBalancerClass)
		svc.Spec.ExternalTrafficPolicy = getValueOrDefault(ns.ExternalTrafficPolicy, "")
	}

	rackSpec, _, ok := oslices.Find(sdc.Spec.Racks, func(rs scyllav1alpha1.RackSpec) bool {
		return rs.Name == rackName
	})
	if !ok {
		return nil, fmt.Errorf("can't find rack spec having %q name", rackName)
	}

	rackSpec = applyRackTemplateOnRackSpec(sdc.Spec.RackTemplate, rackSpec)

	if rackSpec.ExposeOptions != nil && rackSpec.ExposeOptions.NodeService != nil {
		svc.Labels = helpers.MergeMaps(svc.Labels, rackSpec.ExposeOptions.NodeService.Labels)
		svc.Annotations = helpers.MergeMaps(svc.Annotations, rackSpec.ExposeOptions.NodeService.Annotations)
	}

	return svc, nil
}

func getServicePorts(sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]corev1.ServicePort, error) {
	ports := []corev1.ServicePort{
		{
			Name: "inter-node-communication",
			Port: scylla.DefaultStoragePort,
		},
		{
			Name: "ssl-inter-node-communication",
			Port: scylla.DefaultStoragePortSSL,
		},
		{
			Name: portNameCQL,
			Port: scylla.DefaultNativeTransportPort,
		},
		{
			Name: portNameCQLSSL,
			Port: scylla.DefaultNativeTransportPortSSL,
		},
		{
			Name: portNameCQLShardAware,
			Port: scylla.DefaultShardAwareNativeTransportPort,
		},
		{
			Name: portNameCQLSSLShardAware,
			Port: scylla.DefaultShardAwareNativeTransportPortSSL,
		},
		{
			Name: "jmx-monitoring",
			Port: 7199,
		},
		{
			Name: "agent-api",
			Port: scylla.DefaultScyllaManagerAgentPort,
		},
		{
			Name: "prometheus",
			Port: scylla.DefaultScyllaDBMetricsPort,
		},
		{
			Name: "agent-prometheus",
			Port: scylla.DefaultScyllaDBManagerAgentMetricsPort,
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

	if sdc.Spec.ScyllaDB.AlternatorOptions != nil {
		ports = append(ports, corev1.ServicePort{
			Name: alternatorTLSPortName,
			Port: alternatorTLSPort,
		})

		var alternatorPort int
		var enableHTTP bool
		var err error

		alternatorPortAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation]
		if ok {
			alternatorPort, err = strconv.Atoi(alternatorPortAnnotation)
			if err != nil {
				return nil, fmt.Errorf("can't parse alternator port annotation %q: %w", alternatorPortAnnotation, err)
			}
		}
		enableHTTPAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation]
		if ok {
			enableHTTP, err = strconv.ParseBool(enableHTTPAnnotation)
			if err != nil {
				return nil, fmt.Errorf("can't parse enable http annotation %q: %w", enableHTTPAnnotation, err)
			}
		}
		if alternatorPort != 0 || enableHTTP {
			insecurePort := int32(alternatorInsecurePort)
			if alternatorPort != 0 {
				insecurePort = int32(alternatorPort)
			}

			ports = append(ports, corev1.ServicePort{
				Name: alternatorInsecurePortName,
				Port: insecurePort,
			})
		}
	}

	return ports, nil
}

// StatefulSetForRack make a StatefulSet for the rack.
// existingSts may be nil if it doesn't exist yet.
func StatefulSetForRack(rack scyllav1alpha1.RackSpec, sdc *scyllav1alpha1.ScyllaDBDatacenter, existingSts *appsv1.StatefulSet, sidecarImage string, rackOrdinal int, inputsHash string) (*appsv1.StatefulSet, error) {
	selectorLabels, err := naming.RackSelectorLabels(rack, sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get selector labels: %w", err)
	}

	scyllaDBVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		return nil, fmt.Errorf("can't get version of image %q: %w", sdc.Spec.ScyllaDB.Image, err)
	}

	if sdc.Spec.RackTemplate != nil {
		rack = applyRackTemplateOnRackSpec(sdc.Spec.RackTemplate, rack)
	}

	requiredLabels := map[string]string{}
	requiredLabels[naming.RackOrdinalLabel] = strconv.Itoa(rackOrdinal)
	requiredLabels[naming.ScyllaVersionLabel] = scyllaDBVersion
	requiredLabels[naming.PodTypeLabel] = string(naming.PodTypeScyllaDBNode)
	maps.Copy(requiredLabels, selectorLabels)

	sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)

	rackLabels := map[string]string{}
	maps.Copy(rackLabels, sdcLabels)
	maps.Copy(rackLabels, requiredLabels)

	rackTemplateLabels := map[string]string{}
	if sdc.Spec.Metadata != nil && sdc.Spec.Metadata.Labels != nil {
		maps.Copy(rackTemplateLabels, sdc.Spec.Metadata.Labels)
	} else {
		maps.Copy(rackTemplateLabels, sdcLabels)
	}
	maps.Copy(rackTemplateLabels, requiredLabels)

	sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	rackAnnotations := map[string]string{}
	maps.Copy(rackAnnotations, sdcAnnotations)

	rackTemplateAnnotations := map[string]string{}
	if sdc.Spec.Metadata != nil && sdc.Spec.Metadata.Annotations != nil {
		maps.Copy(rackTemplateAnnotations, sdc.Spec.Metadata.Annotations)
	} else {
		maps.Copy(rackTemplateAnnotations, sdcAnnotations)
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
		pvc, _, ok := oslices.Find(existingSts.Spec.VolumeClaimTemplates, func(pvc corev1.PersistentVolumeClaim) bool {
			return pvc.Name == naming.PVCTemplateName
		})
		if !ok {
			return nil, fmt.Errorf("can't find data PVC template %q in existing %q StatefulSet spec", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		existingDataPVCTemplate = &pvc
	}

	dataVolumeClaimLabels := map[string]string{}
	if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil && rack.ScyllaDB.Storage.Metadata != nil && rack.ScyllaDB.Storage.Metadata.Labels != nil {
		maps.Copy(dataVolumeClaimLabels, rack.ScyllaDB.Storage.Metadata.Labels)
	} else if existingSts == nil {
		maps.Copy(dataVolumeClaimLabels, sdcLabels)
	} else {
		if existingDataPVCTemplate == nil {
			return nil, fmt.Errorf("data PVC template %q in existing %q StatefulSet spec is missing", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		maps.Copy(dataVolumeClaimLabels, existingDataPVCTemplate.Labels)
	}
	maps.Copy(dataVolumeClaimLabels, selectorLabels)

	dataVolumeClaimAnnotations := map[string]string{}
	if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil && rack.ScyllaDB.Storage.Metadata != nil && rack.ScyllaDB.Storage.Metadata.Annotations != nil {
		maps.Copy(dataVolumeClaimAnnotations, rack.ScyllaDB.Storage.Metadata.Annotations)
	} else if existingSts == nil {
		maps.Copy(dataVolumeClaimAnnotations, sdcAnnotations)
	} else {
		if existingDataPVCTemplate == nil {
			return nil, fmt.Errorf("data PVC template %q in existing %q StatefulSet spec is missing", naming.PVCTemplateName, naming.ObjRef(existingSts))
		}
		maps.Copy(dataVolumeClaimAnnotations, existingDataPVCTemplate.Annotations)
	}

	placement := rack.Placement
	if placement == nil {
		placement = &scyllav1alpha1.Placement{}
	}
	opt := true

	var storageCapacity resource.Quantity
	if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil {
		storageCapacity, err = resource.ParseQuantity(rack.ScyllaDB.Storage.Capacity)
		if err != nil {
			return nil, fmt.Errorf("cannot parse storage capacity %q: %v", rack.ScyllaDB.Storage.Capacity, err)
		}
	}

	// Assume kube-proxy notices readiness change and reconcile Endpoints within this period
	kubeProxyEndpointsSyncPeriodSeconds := 5
	loadBalancerSyncPeriodSeconds := 60

	readinessFailureThreshold := 1
	readinessPeriodSeconds := 10
	minReadySeconds := kubeProxyEndpointsSyncPeriodSeconds
	minTerminationGracePeriodSeconds := readinessFailureThreshold*readinessPeriodSeconds + kubeProxyEndpointsSyncPeriodSeconds

	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.NodeService != nil && sdc.Spec.ExposeOptions.NodeService.Type == scyllav1alpha1.NodeServiceTypeLoadBalancer {
		// Any "upstream" Load Balancer should notice Endpoint readiness change within this period.
		minTerminationGracePeriodSeconds = loadBalancerSyncPeriodSeconds
		minReadySeconds = loadBalancerSyncPeriodSeconds
	}

	if sdc.Spec.MinTerminationGracePeriodSeconds != nil {
		minTerminationGracePeriodSeconds = int(*sdc.Spec.MinTerminationGracePeriodSeconds)
	}
	if sdc.Spec.MinReadySeconds != nil {
		minReadySeconds = int(*sdc.Spec.MinReadySeconds)
	}

	scyllaContainerPorts, err := containerPorts(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get scylla container ports: %w", err)
	}

	rackNodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
	if err != nil {
		return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
	}

	initContainers, err := makeInitContainers(sdc, sidecarImage)
	if err != nil {
		return nil, fmt.Errorf("can't make init containers: %w", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.StatefulSetNameForRack(rack, sdc),
			Namespace:   sdc.Namespace,
			Labels:      rackLabels,
			Annotations: rackAnnotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: rackNodeCount,
			// Use a common Headless Service for all StatefulSets
			ServiceName: naming.IdentityServiceName(sdc),
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
					HostNetwork: func() bool {
						_, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterHostNetworkingAnnotation]
						return ok
					}(),
					DNSPolicy: func() corev1.DNSPolicy {
						if sdc.Spec.DNSPolicy != nil {
							return *sdc.Spec.DNSPolicy
						}
						return corev1.DNSClusterFirstWithHostNet
					}(),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  pointer.Ptr(rootUID),
						RunAsGroup: pointer.Ptr(rootGID),
					},
					ReadinessGates: sdc.Spec.ReadinessGates,
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
											Name: func() string {
												if rack.ScyllaDB != nil && rack.ScyllaDB.CustomConfigMapRef != nil {
													return *rack.ScyllaDB.CustomConfigMapRef
												}
												return "scylla-config"
											}(),
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
											Name: naming.GetScyllaDBManagedConfigCMName(sdc.Name),
										},
										Optional: pointer.Ptr(false),
									},
								},
							},
							{
								Name: "scylladb-snitch-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: naming.GetScyllaDBRackSnitchConfigCMName(sdc, &rack),
										},
										Optional: pointer.Ptr(false),
									},
								},
							},
							{
								Name: scyllaManagedAgentConfigVolumeName,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: naming.GetScyllaDBManagerAgentConfigCMName(sdc.Name),
										},
										Optional: pointer.Ptr(false),
									},
								},
							},
							{
								Name: scyllaAgentConfigVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: func() string {
											if rack.ScyllaDBManagerAgent != nil && rack.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
												return *rack.ScyllaDBManagerAgent.CustomConfigSecretRef
											}
											return "scylla-agent-config-secret"
										}(),
										Optional: &opt,
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
										SecretName: naming.AgentAuthTokenSecretName(sdc),
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
											SecretName: naming.GetScyllaClusterLocalServingCertName(sdc.Name),
										},
									},
								},
								{
									Name: scylladbClientCAVolumeName,
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: naming.GetScyllaClusterLocalClientCAName(sdc.Name),
											},
										},
									},
								},
								{
									Name: scylladbUserAdminVolumeName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: naming.GetScyllaClusterLocalUserAdminCertName(sdc.Name),
										},
									},
								},
							}...)
						}
						if sdc.Spec.ScyllaDB.AlternatorOptions != nil {
							volumes = append(volumes, corev1.Volume{
								Name: scylladbAlternatorServingCertsVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: naming.GetScyllaClusterAlternatorLocalServingCertName(sdc.Name),
										Optional:   pointer.Ptr(false),
									},
								},
							})
						}

						return volumes
					}(),
					Tolerations:    placement.Tolerations,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						// ScyllaDB container depends on the availability of the operator binary in the shared volume.
						{
							Name:            naming.ScyllaContainerName,
							Image:           sdc.Spec.ScyllaDB.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           scyllaContainerPorts,
							// TODO: unprivileged entrypoint
							Command: func() []string {
								var positionalArgs []string

								if len(sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments) > 0 {
									positionalArgs = append(positionalArgs, sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments...)
								}

								if sdc.Spec.ScyllaDB.EnableDeveloperMode != nil && *sdc.Spec.ScyllaDB.EnableDeveloperMode {
									positionalArgs = append(positionalArgs, "--developer-mode=1")
								} else {
									positionalArgs = append(positionalArgs, "--developer-mode=0")
								}

								cmd := []string{
									"/usr/bin/bash",
									"-euEo",
									"pipefail",
									"-O",
									"inherit_errexit",
									"-c",
									strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM

printf 'INFO %s ignition - Waiting for /mnt/shared/ignition.done\n' "$( date '+%Y-%m-%d %H:%M:%S,%3N' )" > /dev/stderr
until [[ -f "/mnt/shared/ignition.done" ]]; do
  sleep 1 &
  wait
done
printf 'INFO %s ignition - Ignited. Starting ScyllaDB...\n' "$( date '+%Y-%m-%d %H:%M:%S,%3N' )" > /dev/stderr

# TODO: This is where we should start ScyllaDB directly after the sidecar split #1942 
exec /mnt/shared/scylla-operator sidecar \
--feature-gates=` + func() string {
										features := utilfeature.DefaultMutableFeatureGate.GetAll()
										res := make([]string, 0, len(features))
										for name := range features {
											res = append(res, fmt.Sprintf("%s=%t", name, utilfeature.DefaultMutableFeatureGate.Enabled(name)))
										}
										sort.Strings(res)
										return strings.Join(res, ",")
									}() + ` \
--nodes-broadcast-address-type=` + func() string {
										if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
											return string(sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type)
										}
										return string(scyllav1alpha1.ScyllaDBDatacenterDefaultNodesBroadcastAddressType)
									}() + ` \
--clients-broadcast-address-type=` + func() string {
										if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
											return string(sdc.Spec.ExposeOptions.BroadcastOptions.Clients.Type)
										}
										return string(scyllav1alpha1.ScyllaDBDatacenterDefaultClientsBroadcastAddressType)
									}() + ` \
--service-name=$(SERVICE_NAME) \
--cpu-count=$(CPU_COUNT) \
--scylla-localhost-address=` + func() string {
										if sdc.Spec.GetIPFamily() == corev1.IPv6Protocol {
											return "::1"
										}
										return "127.0.0.1"
									}() + ` \
--ip-family=` + string(sdc.Spec.GetIPFamily()) + ` \
` + fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()) + ` \
` +
										func() string {
											var optionalArgs []string

											if len(sdc.Spec.ScyllaDB.ExternalSeeds) > 0 {
												optionalArgs = append(optionalArgs, fmt.Sprintf("--external-seeds=%s", strings.Join(sdc.Spec.ScyllaDB.ExternalSeeds, ",")))
											}

											return strings.Join(optionalArgs, ` \`)
										}() +
										` -- "$@"`,
									),
								}

								cmd = append(cmd, "--")
								cmd = append(cmd, positionalArgs...)

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
							Resources: func() corev1.ResourceRequirements {
								if rack.ScyllaDB != nil && rack.ScyllaDB.Resources != nil {
									return *rack.ScyllaDB.Resources
								}
								return corev1.ResourceRequirements{}
							}(),
							VolumeMounts: func() []corev1.VolumeMount {
								mounts := []corev1.VolumeMount{
									{
										Name:      naming.PVCTemplateName,
										MountPath: naming.DataDir,
									},
									{
										Name:      "shared",
										MountPath: naming.SharedDirName,
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
										Name:      "scylladb-snitch-config",
										ReadOnly:  true,
										MountPath: naming.ScyllaDBSnitchConfigDir,
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
											MountPath: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca",
											ReadOnly:  true,
										},
										{
											Name:      scylladbUserAdminVolumeName,
											MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/user-admin",
											ReadOnly:  true,
										},
									}...)
								}

								if sdc.Spec.ScyllaDB.AlternatorOptions != nil {
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
										Port: apimachineryutilintstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
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
										Port: apimachineryutilintstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
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
										Port: apimachineryutilintstr.FromInt(naming.ScyllaDBAPIStatusProbePort),
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
											strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM
trap 'rm -f /mnt/shared/ignition.done' EXIT

nodetool drain &
sleep ` + strconv.Itoa(minTerminationGracePeriodSeconds) + ` &
wait
`),
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
								fmt.Sprintf("--scylla-localhost-address=%s", func() string {
									if sdc.Spec.GetIPFamily() == corev1.IPv6Protocol {
										return "::1"
									}
									return "127.0.0.1"
								}()),
								fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()),
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
										Port: apimachineryutilintstr.FromInt32(naming.ScyllaDBAPIStatusProbePort),
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
						{
							Name:            naming.ScyllaDBIgnitionContainerName,
							Image:           sidecarImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/scylla-operator",
								"run-ignition",
								"--service-name=$(SERVICE_NAME)",
								fmt.Sprintf("--nodes-broadcast-address-type=%s", func() scyllav1alpha1.BroadcastAddressType {
									if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
										return sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
									}
									return scyllav1alpha1.BroadcastAddressTypeServiceClusterIP
								}()),
								fmt.Sprintf("--clients-broadcast-address-type=%s", func() scyllav1alpha1.BroadcastAddressType {
									if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
										return sdc.Spec.ExposeOptions.BroadcastOptions.Clients.Type
									}
									return scyllav1alpha1.BroadcastAddressTypeServiceClusterIP
								}()),
								fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()),
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
									HTTPGet: &corev1.HTTPGetAction{
										Port: apimachineryutilintstr.FromInt32(naming.ScyllaDBIgnitionProbePort),
										Path: naming.ReadinessProbePath,
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "shared",
									MountPath: naming.SharedDirName,
									ReadOnly:  false,
								},
							},
						},
					},
					ServiceAccountName: naming.MemberServiceAccountNameForScyllaDBDatacenter(sdc.Name),
					Affinity: &corev1.Affinity{
						NodeAffinity:    placement.NodeAffinity,
						PodAffinity:     placement.PodAffinity,
						PodAntiAffinity: placement.PodAntiAffinity,
					},
					ImagePullSecrets:              sdc.Spec.ImagePullSecrets,
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
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: func() *string {
							if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil {
								return rack.ScyllaDB.Storage.StorageClassName
							}
							return nil
						}(),
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

	if sdc.Spec.ForceRedeploymentReason != nil && len(*sdc.Spec.ForceRedeploymentReason) != 0 {
		sts.Spec.Template.Annotations[naming.ForceRedeploymentReasonAnnotation] = *sdc.Spec.ForceRedeploymentReason
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

	if rack.ScyllaDB != nil {
		for _, vm := range rack.ScyllaDB.VolumeMounts {
			sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(sts.Spec.Template.Spec.Containers[0].VolumeMounts, *vm.DeepCopy())
		}
		for _, v := range rack.ScyllaDB.Volumes {
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, *v.DeepCopy())
		}
	}

	if rack.ScyllaDBManagerAgent != nil {
		for _, v := range rack.ScyllaDBManagerAgent.Volumes {
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, *v.DeepCopy())
		}
	}

	agentContainer, err := getScyllaDBManagerAgentContainer(rack, sdc)
	if err != nil {
		return nil, fmt.Errorf("can't create scylladb manager agent container: %w", err)
	}

	if agentContainer != nil {
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, *agentContainer)
	}

	return sts, nil
}

func containerPorts(sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]corev1.ContainerPort, error) {
	ports := []corev1.ContainerPort{
		{
			Name:          "intra-node",
			ContainerPort: scylla.DefaultStoragePort,
		},
		{
			Name:          "tls-intra-node",
			ContainerPort: scylla.DefaultStoragePortSSL,
		},
		{
			Name:          "cql",
			ContainerPort: scylla.DefaultNativeTransportPort,
		},
		{
			Name:          "cql-ssl",
			ContainerPort: scylla.DefaultNativeTransportPortSSL,
		},
		{
			Name:          "jmx",
			ContainerPort: 7199,
		},
		{
			Name:          "prometheus",
			ContainerPort: scylla.DefaultScyllaDBMetricsPort,
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

	if sdc.Spec.ScyllaDB.AlternatorOptions != nil {
		ports = append(ports, corev1.ContainerPort{
			Name:          alternatorTLSPortName,
			ContainerPort: alternatorTLSPort,
		})

		var alternatorPort int
		var enableHTTP bool
		var err error

		alternatorPortAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation]
		if ok {
			alternatorPort, err = strconv.Atoi(alternatorPortAnnotation)
			if err != nil {
				return nil, fmt.Errorf("can't parse alternator port annotation %q: %w", alternatorPortAnnotation, err)
			}
		}
		enableHTTPAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation]
		if ok {
			enableHTTP, err = strconv.ParseBool(enableHTTPAnnotation)
			if err != nil {
				return nil, fmt.Errorf("can't parse enable http annotation %q: %w", enableHTTPAnnotation, err)
			}
		}
		if alternatorPort != 0 || enableHTTP {
			insecurePort := int32(alternatorInsecurePort)
			if alternatorPort != 0 {
				insecurePort = int32(alternatorPort)
			}

			ports = append(ports, corev1.ContainerPort{
				Name:          alternatorInsecurePortName,
				ContainerPort: insecurePort,
			})
		}
	}

	return ports, nil
}

func makeInitContainers(sdc *scyllav1alpha1.ScyllaDBDatacenter, sidecarImage string) ([]corev1.Container, error) {
	var initContainers []corev1.Container

	sidecarInjectionCointainer := makeSidecarInjectionContainer(sidecarImage)
	initContainers = append(initContainers, *sidecarInjectionCointainer)

	sysctlContainer, ok, err := makeSysctlInitContainer(sdc, sidecarImage)
	if err != nil {
		return nil, fmt.Errorf("can't get sysctl init container: %w", err)
	}
	if ok {
		initContainers = append(initContainers, *sysctlContainer)
	}

	bootstrapBarrierContainer, ok, err := makeScyllaDBBootstrapBarrierInitContainer(sdc, sidecarImage)
	if err != nil {
		return nil, fmt.Errorf("can't make ScyllaDB bootstrap barrier init container: %w", err)
	}
	if ok {
		initContainers = append(initContainers, *bootstrapBarrierContainer)
	}

	return initContainers, nil
}

// makeSidecarInjectionContainer creates an init container that copies the operator binary to a shared volume.
// This allows other containers to use the operator binary without it being available in their proper container images.
func makeSidecarInjectionContainer(sidecarImage string) *corev1.Container {
	return &corev1.Container{
		Name:            naming.SidecarInjectorContainerName,
		Image:           sidecarImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf("cp -a /usr/bin/scylla-operator '%s'", naming.SharedDirName),
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
	}
}

func makeSysctlInitContainer(sdc *scyllav1alpha1.ScyllaDBDatacenter, sidecarImage string) (*corev1.Container, bool, error) {
	sysctlsAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation]
	if !ok {
		return nil, false, nil
	}

	var sysctls []string
	err := json.NewDecoder(strings.NewReader(sysctlsAnnotation)).Decode(&sysctls)
	if err != nil {
		return nil, false, fmt.Errorf("can't decode sysctl annotation %q: %w", sysctlsAnnotation, err)
	}

	opt := true
	return &corev1.Container{
		Name:            "sysctl-buddy",
		Image:           sidecarImage,
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
	}, true, nil
}

// makeScyllaDBBootstrapBarrierInitContainer creates an init container that blocks proceeding with ScyllaDB startup until bootstrap preconditions are met.
// It depends on the availability of the operator binary in a shared volume, as well as `scylla sstable query` command in the ScyllaDB container image.
func makeScyllaDBBootstrapBarrierInitContainer(sdc *scyllav1alpha1.ScyllaDBDatacenter, image string) (*corev1.Container, bool, error) {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(features.BootstrapSynchronisation) {
		return nil, false, nil
	}

	scyllaDBVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		return nil, false, fmt.Errorf("can't get version of image %q: %w", sdc.Spec.ScyllaDB.Image, err)
	}
	sv := semver.NewScyllaVersion(scyllaDBVersion)
	if !sv.SupportFeatureUnsafe(semver.ScyllaDBVersionRequiredForBootstrapSynchronisation) {
		klog.V(4).InfoS("Not including bootstrap barrier init container as ScyllaDB version does not support it", "ScyllaDBDatacenter", naming.ObjRef(sdc), "ScyllaDBVersion", scyllaDBVersion, "ScyllaDBVersionRequiredForBootstrapSynchronisation", semver.ScyllaDBVersionRequiredForBootstrapSynchronisation)
		return nil, false, nil
	}

	c := &corev1.Container{
		Name:            "scylladb-bootstrap-barrier",
		Image:           sdc.Spec.ScyllaDB.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/mnt/shared/scylla-operator",
			"run-bootstrap-barrier",
			"--service-name=$(SERVICE_NAME)",
			fmt.Sprintf("--scylla-data-dir=%s", path.Join(naming.DataDir, "/data")),
			fmt.Sprintf("--selector-label-value=%s", naming.ScyllaDBDatacenterNodesStatusReportSelectorLabelValue(sdc)),
			func() string {
				allowNonReportingHostIDsForSingleReport := false
				if len(sdc.Spec.ScyllaDB.ExternalSeeds) > 0 {
					// We assume that non-empty external seeds determine a multi-datacenter cluster.
					// To handle non-automated multi-datacenter deployment model, we allow non-reporting host IDs to be present in status reports if there are no reports from other datacenters.
					allowNonReportingHostIDsForSingleReport = true
				}

				return fmt.Sprintf("--single-report-allow-non-reporting-host-ids=%t", allowNonReportingHostIDsForSingleReport)
			}(),
			fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()),
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      naming.PVCTemplateName,
				MountPath: naming.DataDir,
				ReadOnly:  true,
			},
			{
				Name:      "shared",
				MountPath: naming.SharedDirName,
				ReadOnly:  true,
			},
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
	}
	return c, true, nil
}

func getScyllaDBManagerAgentContainer(r scyllav1alpha1.RackSpec, sdc *scyllav1alpha1.ScyllaDBDatacenter) (*corev1.Container, error) {
	if sdc.Spec.ScyllaDBManagerAgent == nil {
		return nil, nil
	}

	if sdc.Spec.ScyllaDBManagerAgent.Image == nil {
		return nil, fmt.Errorf("ScyllaDBDatacneter %q is missing scylla manager agent image", naming.ObjRef(sdc))
	}

	cnt := &corev1.Container{
		Name:            naming.ScyllaManagerAgentContainerName,
		Image:           *sdc.Spec.ScyllaDBManagerAgent.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		// There is no point in starting scylla-manager before ScyllaDB is tuned and ignited. The manager agent fails after 60 attempts and hits backoff unnecessarily.
		Command: []string{
			"/usr/bin/bash",
			"-euEo",
			"pipefail",
			"-O",
			"inherit_errexit",
			"-c",
			strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM

printf '{"L":"INFO","T":"%s","M":"Waiting for /mnt/shared/ignition.done"}\n' "$( date -u '+%Y-%m-%dT%H:%M:%S,%3NZ' )" > /dev/stderr
until [[ -f "/mnt/shared/ignition.done" ]]; do
  sleep 1 &
  wait
done
printf '{"L":"INFO","T":"%s","M":"Ignited. Starting ScyllaDB Manager Agent"}\n' "$( date -u '+%Y-%m-%dT%H:%M:%S,%3NZ' )" > /dev/stderr

exec scylla-manager-agent \
-c ` + fmt.Sprintf("%q ", naming.ScyllaAgentConfigDefaultFile) + `\
-c ` + fmt.Sprintf("%q ", path.Join(naming.ScyllaManagedAgentConfigDirName, naming.ScyllaAgentConfigFileName)) + `\
-c ` + fmt.Sprintf("%q ", path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentConfigFileName)) + `\
-c ` + fmt.Sprintf("%q ", path.Join(naming.ScyllaAgentConfigDirName, naming.ScyllaAgentAuthTokenFileName)) + `
`),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "agent-rest-api",
				ContainerPort: 10001,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: apimachineryutilintstr.FromInt32(10001),
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      naming.PVCTemplateName,
				MountPath: naming.DataDir,
			},
			{
				Name:      scyllaManagedAgentConfigVolumeName,
				MountPath: path.Join(naming.ScyllaManagedAgentConfigDirName, naming.ScyllaAgentConfigFileName),
				SubPath:   naming.ScyllaAgentConfigFileName,
				ReadOnly:  true,
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
			{
				Name:      "shared",
				MountPath: naming.SharedDirName,
				ReadOnly:  true,
			},
		},
		Resources: func() corev1.ResourceRequirements {
			if r.ScyllaDBManagerAgent != nil && r.ScyllaDBManagerAgent.Resources != nil {
				return *r.ScyllaDBManagerAgent.Resources
			}
			return corev1.ResourceRequirements{}
		}(),
	}

	if r.ScyllaDBManagerAgent != nil {
		for _, vm := range r.ScyllaDBManagerAgent.VolumeMounts {
			cnt.VolumeMounts = append(cnt.VolumeMounts, *vm.DeepCopy())
		}
	}

	return cnt, nil
}

func MakePodDisruptionBudget(sdc *scyllav1alpha1.ScyllaDBDatacenter) *policyv1.PodDisruptionBudget {
	maxUnavailable := apimachineryutilintstr.FromInt(1)

	selectorLabels := naming.ClusterLabels(sdc)

	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, selectorLabels)

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	// Ignore any Job Pods that share the selector with ScyllaDB Pods, they shouldn't be accounted for PDB.
	selector := metav1.SetAsLabelSelector(selectorLabels)
	selector.MatchExpressions = append(selector.MatchExpressions, metav1.LabelSelectorRequirement{
		Key:      "batch.kubernetes.io/job-name",
		Operator: metav1.LabelSelectorOpDoesNotExist,
	})

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PodDisruptionBudgetName(sdc),
			Namespace: sdc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       selector,
		},
	}
}

func MakeIngresses(sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service) []*networkingv1.Ingress {
	// Don't create Ingresses if cluster isn't exposed.
	if sdc.Spec.ExposeOptions == nil {
		return nil
	}

	type params struct {
		ingressNameSuffix     string
		portName              string
		protocolSubdomainFunc func(string) string
		memberSubdomainFunc   func(string, string) string
		ingressOptions        *scyllav1alpha1.CQLExposeIngressOptions
	}
	var ingressParams []params

	if sdc.Spec.ExposeOptions.CQL != nil && sdc.Spec.ExposeOptions.CQL.Ingress != nil {
		ingressParams = append(ingressParams, params{
			ingressNameSuffix:     "cql",
			portName:              portNameCQLSSL,
			protocolSubdomainFunc: naming.GetCQLProtocolSubDomain,
			memberSubdomainFunc:   naming.GetCQLHostIDSubDomain,
			ingressOptions:        sdc.Spec.ExposeOptions.CQL.Ingress,
		})
	}

	var ingresses []*networkingv1.Ingress

	sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)

	sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	for _, ip := range ingressParams {
		for _, service := range services {
			var hosts []string

			annotations := map[string]string{}
			if ip.ingressOptions.Annotations != nil {
				maps.Copy(annotations, ip.ingressOptions.Annotations)
			} else {
				maps.Copy(annotations, sdcAnnotations)
			}

			labels := map[string]string{}
			maps.Copy(labels, sdcLabels)
			maps.Copy(labels, naming.ClusterLabels(sdc))

			switch naming.ScyllaServiceType(service.Labels[naming.ScyllaServiceTypeLabel]) {
			case naming.ScyllaServiceTypeIdentity:
				for _, domain := range sdc.Spec.DNSDomains {
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

				for _, domain := range sdc.Spec.DNSDomains {
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
					Namespace:   sdc.Namespace,
					Labels:      labels,
					Annotations: annotations,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
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

func makeAgentAuthTokenSecret(sdc *scyllav1alpha1.ScyllaDBDatacenter, agentAuthToken string) (*corev1.Secret, error) {
	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	agentAuthTokenConfig, err := helpers.GetAgentAuthTokenConfig(agentAuthToken)
	if err != nil {
		return nil, fmt.Errorf("can't get agent auth token config for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.AgentAuthTokenSecretName(sdc),
			Namespace: sdc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			naming.ScyllaAgentAuthTokenFileName: agentAuthTokenConfig,
		},
	}, nil
}

func ImageForCluster(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s:%s", c.Spec.Repository, c.Spec.Version)
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

func MakeServiceAccount(sdc *scyllav1alpha1.ScyllaDBDatacenter) *corev1.ServiceAccount {
	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.MemberServiceAccountNameForScyllaDBDatacenter(sdc.Name),
			Namespace: sdc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func MakeRoleBinding(sdc *scyllav1alpha1.ScyllaDBDatacenter) *rbacv1.RoleBinding {
	saName := naming.MemberServiceAccountNameForScyllaDBDatacenter(sdc.Name)

	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: sdc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
			Labels:      labels,
			Annotations: annotations,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: sdc.Namespace,
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

func MakeJobs(sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service, podLister corev1listers.PodLister, image string) ([]*batchv1.Job, []metav1.Condition, error) {
	var jobs []*batchv1.Job
	var progressingConditions []metav1.Condition

	for _, rack := range sdc.Spec.Racks {
		rackNodes, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			return jobs, progressingConditions, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
		}

		for i := int32(0); i < *rackNodes; i++ {
			svcName := naming.MemberServiceName(rack, sdc, int(i))
			svc, ok := services[svcName]
			if !ok {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForService",
					Message:            fmt.Sprintf("Waiting for Service %q", naming.ManualRef(sdc.Namespace, svcName)),
					ObservedGeneration: sdc.Generation,
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
					ObservedGeneration: sdc.Generation,
				})
				continue
			}

			if len(currentTokenRingHash) == 0 {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "UnexpectedServiceState",
					Message:            fmt.Sprintf("Service %q has unexpected empty current token ring hash annotation, can't create cleanup Job", naming.ObjRef(svc)),
					ObservedGeneration: sdc.Generation,
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
					ObservedGeneration: sdc.Generation,
				})
				continue
			}

			if len(lastCleanedUpTokenRingHash) == 0 {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               jobControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "UnexpectedServiceState",
					Message:            fmt.Sprintf("Service %q has unexpected empty last cleaned up token ring hash annotation, can't create cleanup Job", naming.ObjRef(svc)),
					ObservedGeneration: sdc.Generation,
				})
				klog.Warningf("Can't create cleanup Job for Service %s because it has unexpected empty last cleaned up token ring hash annotation", klog.KObj(svc))
				continue
			}

			if currentTokenRingHash == lastCleanedUpTokenRingHash {
				klog.V(4).Infof("Node %q already cleaned up", naming.ObjRef(svc))
				continue
			}

			klog.InfoS("Node requires a cleanup", "Node", naming.ObjRef(svc), "CurrentHash", currentTokenRingHash, "LastCleanedUpHash", lastCleanedUpTokenRingHash)

			labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)

			maps.Copy(labels, map[string]string{
				naming.ClusterNameLabel: sdc.Name,
				naming.NodeJobLabel:     svcName,
				naming.NodeJobTypeLabel: string(naming.JobTypeCleanup),
			})

			podLabels := maps.Clone(labels)
			podLabels[naming.PodTypeLabel] = string(naming.PodTypeCleanupJob)

			annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)
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

			pod, err := podLister.Pods(sdc.Namespace).Get(naming.PodNameFromService(svc))
			if err != nil {
				return jobs, progressingConditions, fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(sdc.Namespace, naming.PodNameFromService(svc)), err)
			}

			clientBroadcastAddress, err := controllerhelpers.GetScyllaClientBroadcastHost(sdc, svc, pod)
			if err != nil {
				return jobs, progressingConditions, fmt.Errorf("can't get node address of %q Pod: %w", naming.ManualRef(sdc.Namespace, naming.PodNameFromService(svc)), err)
			}

			jobs = append(jobs, &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      naming.CleanupJobForService(svc.Name),
					Namespace: sdc.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
					},
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: batchv1.JobSpec{
					Selector:       nil,
					ManualSelector: pointer.Ptr(false),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      podLabels,
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
										fmt.Sprintf("--node-address=%s", clientBroadcastAddress),
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
											SecretName: naming.AgentAuthTokenSecretName(sdc),
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

	return jobs, progressingConditions, nil
}

func MakeManagedScyllaDBConfigMaps(sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]*corev1.ConfigMap, error) {
	var managedCMs []*corev1.ConfigMap

	scyllaDBConfigCM, err := MakeManagedScyllaDBConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't make managed scylladb config: %w", err)
	}

	managedCMs = append(managedCMs, scyllaDBConfigCM)

	scyllaDBSnitchConfigCMs, err := MakeManagedScyllaDBSnitchConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't make managed scylladb snitch config: %w", err)
	}

	managedCMs = append(managedCMs, scyllaDBSnitchConfigCMs...)

	scyllaDBManagerAgentConfigCM, err := MakeManagedScyllaDBManagerAgentConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't make managed scylladb manager agent config: %w", err)
	}

	managedCMs = append(managedCMs, scyllaDBManagerAgentConfigCM)

	return managedCMs, nil
}

func MakeManagedScyllaDBSnitchConfig(sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]*corev1.ConfigMap, error) {
	snitchConfigsCMs := make([]*corev1.ConfigMap, 0, len(sdc.Spec.Racks))

	sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	for _, rack := range sdc.Spec.Racks {
		cm, _, err := scylladbassets.ScyllaDBSnitchConfigTemplate.Get().RenderObject(
			map[string]any{
				"Namespace":        sdc.Namespace,
				"Name":             naming.GetScyllaDBRackSnitchConfigCMName(sdc, &rack),
				"SnitchConfigName": naming.ScyllaRackDCPropertiesName,
				"DatacenterName":   naming.GetScyllaDBDatacenterGossipDatacenterName(sdc),
				"RackName":         rack.Name,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("can't render scylladb snitch config for rack %q: %w", rack.Name, err)
		}

		cm.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         scyllav1alpha1.ScyllaDBDatacenterGVK.GroupVersion().String(),
				Kind:               scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name:               sdc.Name,
				UID:                sdc.UID,
				Controller:         pointer.Ptr(true),
				BlockOwnerDeletion: pointer.Ptr(true),
			},
		})

		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		maps.Copy(cm.Labels, sdcLabels)
		maps.Copy(cm.Labels, naming.ClusterLabels(sdc))

		if cm.Annotations == nil {
			cm.Annotations = map[string]string{}
		}

		maps.Copy(cm.Annotations, sdcAnnotations)

		snitchConfigsCMs = append(snitchConfigsCMs, cm)
	}

	return snitchConfigsCMs, nil
}

func MakeManagedScyllaDBConfig(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*corev1.ConfigMap, error) {
	alternatorPortAnnotation := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation]
	var alternatorPort int32

	if len(alternatorPortAnnotation) > 0 {
		ap, err := strconv.Atoi(alternatorPortAnnotation)
		if err != nil {
			return nil, fmt.Errorf("can't convert alternator port annotation %q to int: %w", alternatorPortAnnotation, err)
		}
		alternatorPort = int32(ap)
	}

	getBoolAnnotation := func(annotation string) *bool {
		v, ok := sdc.Annotations[annotation]
		if !ok {
			return nil
		}
		if v == "true" {
			return pointer.Ptr(true)
		}
		return pointer.Ptr(false)
	}

	cm, _, err := scylladbassets.ScyllaDBManagedConfigTemplate.Get().RenderObject(
		map[string]any{
			"Namespace":                              sdc.Namespace,
			"Name":                                   naming.GetScyllaDBManagedConfigCMName(sdc.Name),
			"ClusterName":                            sdc.Spec.ClusterName,
			"ManagedConfigName":                      naming.ScyllaDBManagedConfigName,
			"EnableTLS":                              utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates),
			"AlternatorInsecureDisableAuthorization": getBoolAnnotation(naming.TransformScyllaClusterToScyllaDBDatacenterInsecureDisableAuthorizationAnnotation),
			"AlternatorInsecureEnableHTTP":           getBoolAnnotation(naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation),
			"AlternatorPort":                         alternatorPort,
			"Spec":                                   sdc.Spec,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't render managed scylladb config: %w", err)
	}

	cm.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         scyllav1alpha1.ScyllaDBDatacenterGVK.GroupVersion().String(),
			Kind:               scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
			Name:               sdc.Name,
			UID:                sdc.UID,
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
		},
	})

	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(cm.Labels, sdcLabels)
	maps.Copy(cm.Labels, naming.ClusterLabels(sdc))

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)
	maps.Copy(cm.Annotations, sdcAnnotations)

	return cm, nil
}

func MakeManagedScyllaDBManagerAgentConfig(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*corev1.ConfigMap, error) {
	cm, _, err := scylladbassets.ScyllaDBManagerAgentConfigTemplate.Get().RenderObject(
		map[string]any{
			"Namespace":                 sdc.Namespace,
			"Name":                      naming.GetScyllaDBManagerAgentConfigCMName(sdc.Name),
			"ScyllaAgentConfigFileName": naming.ScyllaAgentConfigFileName,
			"IPFamily":                  sdc.Spec.GetIPFamily(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't render managed scylladb manager agent config: %w", err)
	}

	cm.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         scyllav1alpha1.ScyllaDBDatacenterGVK.GroupVersion().String(),
			Kind:               scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
			Name:               sdc.Name,
			UID:                sdc.UID,
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
		},
	})

	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	sdcLabels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(cm.Labels, sdcLabels)
	maps.Copy(cm.Labels, naming.ClusterLabels(sdc))

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	sdcAnnotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)
	maps.Copy(cm.Annotations, sdcAnnotations)

	return cm, nil
}

func applyRackTemplateOnRackSpec(rackTemplate *scyllav1alpha1.RackTemplate, rack scyllav1alpha1.RackSpec) scyllav1alpha1.RackSpec {
	if rackTemplate == nil {
		return rack
	}

	return scyllav1alpha1.RackSpec{
		Name: rack.Name,
		RackTemplate: scyllav1alpha1.RackTemplate{
			Nodes: func() *int32 {
				if rack.Nodes != nil {
					return rack.Nodes
				}
				return rackTemplate.Nodes
			}(),
			ScyllaDB: func() *scyllav1alpha1.ScyllaDBTemplate {
				return &scyllav1alpha1.ScyllaDBTemplate{
					Resources: func() *corev1.ResourceRequirements {
						limits := make(corev1.ResourceList)
						requests := make(corev1.ResourceList)

						if rackTemplate.ScyllaDB != nil && rackTemplate.ScyllaDB.Resources != nil {
							maps.Copy(limits, rackTemplate.ScyllaDB.Resources.Limits)
							maps.Copy(requests, rackTemplate.ScyllaDB.Resources.Requests)
						}
						if rack.ScyllaDB != nil && rack.ScyllaDB.Resources != nil {
							maps.Copy(limits, rack.ScyllaDB.Resources.Limits)
							maps.Copy(requests, rack.ScyllaDB.Resources.Requests)
						}

						return &corev1.ResourceRequirements{
							Limits:   limits,
							Requests: requests,
						}
					}(),
					Storage: func() *scyllav1alpha1.StorageOptions {
						return &scyllav1alpha1.StorageOptions{
							Metadata: func() *scyllav1alpha1.ObjectTemplateMetadata {
								labels := make(map[string]string)
								annotations := make(map[string]string)
								if rackTemplate.ScyllaDB != nil && rackTemplate.ScyllaDB.Storage != nil && rackTemplate.ScyllaDB.Storage.Metadata != nil {
									maps.Copy(labels, rackTemplate.ScyllaDB.Storage.Metadata.Labels)
									maps.Copy(annotations, rackTemplate.ScyllaDB.Storage.Metadata.Annotations)
								}
								if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil && rack.ScyllaDB.Storage.Metadata != nil {
									maps.Copy(labels, rack.ScyllaDB.Storage.Metadata.Labels)
									maps.Copy(annotations, rack.ScyllaDB.Storage.Metadata.Annotations)
								}
								return &scyllav1alpha1.ObjectTemplateMetadata{
									Labels:      labels,
									Annotations: annotations,
								}
							}(),
							Capacity: func() string {
								if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil && len(rack.ScyllaDB.Storage.Capacity) != 0 {
									return rack.ScyllaDB.Storage.Capacity
								}
								if rackTemplate.ScyllaDB != nil && rackTemplate.ScyllaDB.Storage != nil && len(rackTemplate.ScyllaDB.Storage.Capacity) != 0 {
									return rackTemplate.ScyllaDB.Storage.Capacity
								}
								return ""
							}(),
							StorageClassName: func() *string {
								if rack.ScyllaDB != nil && rack.ScyllaDB.Storage != nil && rack.ScyllaDB.Storage.StorageClassName != nil {
									return rack.ScyllaDB.Storage.StorageClassName
								}
								if rackTemplate.ScyllaDB != nil && rackTemplate.ScyllaDB.Storage != nil && rackTemplate.ScyllaDB.Storage.StorageClassName != nil {
									return rackTemplate.ScyllaDB.Storage.StorageClassName
								}
								return nil
							}(),
						}
					}(),
					CustomConfigMapRef: func() *string {
						if rack.ScyllaDB != nil && rack.ScyllaDB.CustomConfigMapRef != nil {
							return rack.ScyllaDB.CustomConfigMapRef
						}
						if rackTemplate.ScyllaDB != nil && rackTemplate.ScyllaDB.CustomConfigMapRef != nil {
							return rackTemplate.ScyllaDB.CustomConfigMapRef
						}
						return nil
					}(),
					Volumes: func() []corev1.Volume {
						var volumes []corev1.Volume
						if rackTemplate.ScyllaDB != nil {
							volumes = append(volumes, rackTemplate.ScyllaDB.Volumes...)
						}
						if rack.ScyllaDB != nil {
							volumes = append(volumes, rack.ScyllaDB.Volumes...)
						}
						return volumes
					}(),
					VolumeMounts: func() []corev1.VolumeMount {
						var volumeMounts []corev1.VolumeMount
						if rackTemplate.ScyllaDB != nil {
							volumeMounts = append(volumeMounts, rackTemplate.ScyllaDB.VolumeMounts...)
						}
						if rack.ScyllaDB != nil {
							volumeMounts = append(volumeMounts, rack.ScyllaDB.VolumeMounts...)
						}
						return volumeMounts
					}(),
				}
			}(),
			ScyllaDBManagerAgent: func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
				return &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					Resources: func() *corev1.ResourceRequirements {
						limits := make(corev1.ResourceList)
						requests := make(corev1.ResourceList)

						if rackTemplate.ScyllaDBManagerAgent != nil && rackTemplate.ScyllaDBManagerAgent.Resources != nil {
							maps.Copy(limits, rackTemplate.ScyllaDBManagerAgent.Resources.Limits)
							maps.Copy(requests, rackTemplate.ScyllaDBManagerAgent.Resources.Requests)
						}
						if rack.ScyllaDBManagerAgent != nil && rack.ScyllaDBManagerAgent.Resources != nil {
							maps.Copy(limits, rack.ScyllaDBManagerAgent.Resources.Limits)
							maps.Copy(requests, rack.ScyllaDBManagerAgent.Resources.Requests)
						}

						return &corev1.ResourceRequirements{
							Limits:   limits,
							Requests: requests,
						}
					}(),
					CustomConfigSecretRef: nil,
					Volumes: func() []corev1.Volume {
						var volumes []corev1.Volume
						if rackTemplate.ScyllaDBManagerAgent != nil {
							volumes = append(volumes, rackTemplate.ScyllaDBManagerAgent.Volumes...)
						}
						if rack.ScyllaDBManagerAgent != nil {
							volumes = append(volumes, rack.ScyllaDBManagerAgent.Volumes...)
						}
						return volumes
					}(),
					VolumeMounts: func() []corev1.VolumeMount {
						var volumeMounts []corev1.VolumeMount
						if rackTemplate.ScyllaDBManagerAgent != nil {
							volumeMounts = append(volumeMounts, rackTemplate.ScyllaDBManagerAgent.VolumeMounts...)
						}
						if rack.ScyllaDBManagerAgent != nil {
							volumeMounts = append(volumeMounts, rack.ScyllaDBManagerAgent.VolumeMounts...)
						}
						return volumeMounts
					}(),
				}
			}(),
			Placement: func() *scyllav1alpha1.Placement {
				if rack.Placement != nil {
					return rack.Placement
				}
				if rackTemplate != nil {
					return rackTemplate.Placement
				}

				topologyLabelSelector := make(map[string]string)
				maps.Copy(topologyLabelSelector, rackTemplate.TopologyLabelSelector)
				maps.Copy(topologyLabelSelector, rack.TopologyLabelSelector)

				return &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: func() []corev1.NodeSelectorRequirement {
										var reqs []corev1.NodeSelectorRequirement
										for k, v := range topologyLabelSelector {
											reqs = append(reqs, corev1.NodeSelectorRequirement{
												Key:      k,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{v},
											})
										}
										return reqs
									}(),
								},
							},
						},
					},
				}
			}(),
			ExposeOptions: func() *scyllav1alpha1.RackExposeOptions {
				if rackTemplate.ExposeOptions == nil && rack.ExposeOptions == nil {
					return nil
				}

				dst := &scyllav1alpha1.RackExposeOptions{}
				for _, reo := range []*scyllav1alpha1.RackExposeOptions{rackTemplate.ExposeOptions, rack.ExposeOptions} {
					if reo == nil {
						continue
					}

					if reo.NodeService != nil {
						if dst.NodeService == nil {
							dst.NodeService = &scyllav1alpha1.RackNodeServiceTemplate{
								ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
									Labels:      make(map[string]string),
									Annotations: make(map[string]string),
								},
							}
						}

						maps.Copy(dst.NodeService.Labels, reo.NodeService.Labels)
						maps.Copy(dst.NodeService.Annotations, reo.NodeService.Annotations)
					}
				}

				return dst
			}(),
		},
	}
}

func MakeUpgradeContextConfigMap(sdc *scyllav1alpha1.ScyllaDBDatacenter, uc *internalapi.DatacenterUpgradeContext) (*corev1.ConfigMap, error) {
	cmName := naming.UpgradeContextConfigMapName(sdc)

	data, err := uc.Encode()
	if err != nil {
		return nil, fmt.Errorf("can't encode upgrade context: %w", err)
	}

	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cmName,
			Namespace:   sdc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
		},
		Data: map[string]string{
			naming.UpgradeContextConfigMapKey: string(data),
		},
	}, nil
}

// cloneMapExcludingKeysOrEmpty creates a new map by copying the contents of the input map, excluding specified keys.
// If the input map is nil, it returns an empty map.
func cloneMapExcludingKeysOrEmpty[M ~map[K]V, S ~[]K, K comparable, V any](m M, excludedKeys S) M {
	r := map[K]V{}
	maps.Copy(r, m)
	for _, k := range excludedKeys {
		delete(r, k)
	}
	return r
}

func makeScyllaDBDatacenterNodesStatusReport(sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service, podLister corev1listers.PodLister) (*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, error) {
	var err error

	var errs []error
	var rackStatusReports []scyllav1alpha1.RackNodesStatusReport
	for _, rack := range sdc.Spec.Racks {
		rackNodesStatusReport, err := makeRackNodesStatusReport(sdc, &rack, services, podLister)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't make rack status report for rack %q of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err))
			continue
		}

		rackStatusReports = append(rackStatusReports, *rackNodesStatusReport)
	}
	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	name, err := naming.ScyllaDBDatacenterNodesStatusReportName(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get ScyllaDBDatacenterNodesStatusReport name for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}

	labels := cloneMapExcludingKeysOrEmpty(sdc.Labels, nonPropagatedLabelKeys)
	maps.Copy(labels, naming.ClusterLabels(sdc))
	labels[naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel] = naming.ScyllaDBDatacenterNodesStatusReportSelectorLabelValue(sdc)

	annotations := cloneMapExcludingKeysOrEmpty(sdc.Annotations, nonPropagatedAnnotationKeys)

	ssr := &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   sdc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
		},
		DatacenterName: naming.GetScyllaDBDatacenterGossipDatacenterName(sdc),
		Racks:          rackStatusReports,
	}
	return ssr, nil
}

func makeRackNodesStatusReport(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackSpec *scyllav1alpha1.RackSpec, services map[string]*corev1.Service, podLister corev1listers.PodLister) (*scyllav1alpha1.RackNodesStatusReport, error) {
	var errs []error

	desiredRackNodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rackSpec.Name)
	if err != nil {
		return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rackSpec.Name, naming.ObjRef(sdc), err)
	}

	actualRackNodeCount := int32(0)
	rackStatus, _, found := oslices.Find(sdc.Status.Racks, func(status scyllav1alpha1.RackStatus) bool {
		return status.Name == rackSpec.Name
	})
	if found && rackStatus.CurrentNodes != nil {
		actualRackNodeCount = *rackStatus.CurrentNodes
	}

	var nodeStatusReports []scyllav1alpha1.NodeStatusReport
	for ord := int32(0); ord < *desiredRackNodeCount; ord++ {
		isNodeExpectedInK8sState := ord < actualRackNodeCount
		nodeStatusReport, ok, err := makeNodeStatusReport(sdc, rackSpec, int(ord), services, podLister, isNodeExpectedInK8sState)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't make node status report for node %d of rack %q of ScyllaDBDatacenter %q: %w", ord, rackSpec.Name, naming.ObjRef(sdc), err))
			continue
		}
		if !ok {
			continue
		}

		nodeStatusReports = append(nodeStatusReports, *nodeStatusReport)
	}
	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	rackStatusReport := &scyllav1alpha1.RackNodesStatusReport{
		Name:  rackSpec.Name,
		Nodes: nodeStatusReports,
	}
	return rackStatusReport, nil
}

// makeNodeStatusReport creates a NodeStatusReport for a specific node in a rack.
// It returns an optional NodeStatusReport, a boolean indicating whether the NodeStatusReport is non-nil, and an error.
func makeNodeStatusReport(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackSpec *scyllav1alpha1.RackSpec, ordinal int, services map[string]*corev1.Service, podLister corev1listers.PodLister, isNodeExpectedInK8sState bool) (*scyllav1alpha1.NodeStatusReport, bool, error) {
	var hostID string
	svcName := naming.MemberServiceName(*rackSpec, sdc, ordinal)
	svc, svcExists := services[svcName]
	if svcExists {
		hostID = svc.Annotations[naming.HostIDAnnotation]
	}

	if !isNodeExpectedInK8sState && len(hostID) == 0 {
		// The node is not expected to be a part of the cluster in K8s state, and it has no known identity in ScyllaDB. Skip it.
		klog.V(5).InfoS("Node is not expected to be part of the cluster in Kubernetes state and has no known identity in ScyllaDB, skipping", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KRef(sdc.Namespace, svcName))
		return nil, false, nil
	}

	nodeStatusReport := &scyllav1alpha1.NodeStatusReport{
		Ordinal: ordinal,
	}

	if len(hostID) == 0 {
		// Host ID hasn't been propagated yet, report an empty status without a hostID.
		klog.V(4).InfoS("HostID of an expected node has nod been propagated yet, reporting an empty status", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KRef(sdc.Namespace, svcName))
		return nodeStatusReport, true, nil
	}
	nodeStatusReport.HostID = &hostID

	// HostID is non-empty, we can now safely use the svc object.
	podName := naming.PodNameFromService(svc)
	pod, err := podLister.Pods(sdc.Namespace).Get(podName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
		}

		// Pod is missing, report an empty status.
		klog.V(4).InfoS("Pod of an expected node is missing, reporting an empty status", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc), "Pod", klog.KRef(sdc.Namespace, podName))
		return nodeStatusReport, true, nil
	}

	nodeStatusReportAnnotationValue, ok := pod.Annotations[naming.NodeStatusReportAnnotation]
	if !ok {
		// The node might not have reported its status yet, report an empty status.
		klog.V(4).InfoS("Node status report annotation is missing on Pod of an expected node, reporting an empty status", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc), "Pod", klog.KObj(pod), "AnnotationKey", naming.NodeStatusReportAnnotation)
		return nodeStatusReport, true, nil
	}

	var internalNodeStatusReport internalapi.NodeStatusReport
	err = internalNodeStatusReport.Decode(strings.NewReader(nodeStatusReportAnnotationValue))
	if err != nil {
		return nil, false, fmt.Errorf("can't decode annotation %q of pod %q for ScyllaDBDatacenter %q: %w", naming.NodeStatusReportAnnotation, naming.ManualRef(sdc.Namespace, podName), naming.ObjRef(sdc), err)
	}

	if internalNodeStatusReport.Error != nil {
		// The node reported an error, report an empty status.
		klog.V(4).InfoS("Node reported an error in its status report, reporting an empty status", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc), "Pod", klog.KObj(pod), "Error", internalNodeStatusReport.Error)
		return nodeStatusReport, true, nil
	}

	nodeStatusReport.ObservedNodes = internalNodeStatusReport.ObservedNodes

	klog.V(5).InfoS("Successfully built a node status report for an expected node", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc), "Pod", klog.KObj(pod))
	return nodeStatusReport, true, nil
}
