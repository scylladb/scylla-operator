// Copyright (c) 2024 ScyllaDB.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScyllaDBDatacenterSpec defines the desired state of ScyllaDBDatacenter.
type ScyllaDBDatacenterSpec struct {
	// metadata controls shared metadata for all pods created based on this spec.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// clusterName specifies the name of the ScyllaDB cluster.
	// When joining two DCs, their cluster name must match.
	// This field is immutable.
	ClusterName string `json:"clusterName"`

	// datacenterName specifies the name of the ScyllaDB datacenter. Used as datacenter name in GossipingPropertyFileSnitch.
	// If empty, it's taken from the 'scylladbdatacenter.metadata.name'.
	// +optional
	DatacenterName *string `json:"datacenterName,omitempty"`

	// scyllaDB holds a specification of ScyllaDB.
	ScyllaDB ScyllaDB `json:"scyllaDB"`

	// scyllaDBManagerAgent holds a specification of ScyllaDB Manager Agent.
	// +optional
	ScyllaDBManagerAgent *ScyllaDBManagerAgent `json:"scyllaDBManagerAgent,omitempty"`

	// imagePullSecrets is an optional list of references to secrets in the same namespace
	// used for pulling any images used by this spec.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// dnsPolicy defines how a pod's DNS will be configured.
	// +optional
	DNSPolicy *corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// dnsDomains specifies a list of DNS domains this cluster is reachable by.
	// These domains are used when setting up the infrastructure, like certificates.
	// +optional
	DNSDomains []string `json:"dnsDomains,omitempty"`

	// forceRedeploymentReason specifies the latest redeployment reason.
	// Can be used to force a rolling restart of all racks in this DC by providing a unique string.
	// +optional
	ForceRedeploymentReason *string `json:"forceRedeploymentReason,omitempty"`

	// IPFamily specifies the IP family for this datacenter.
	// All services, broadcast addresses, and pod IPs will use this IP family.
	// +kubebuilder:validation:Enum=IPv4;IPv6
	// +kubebuilder:default="IPv4"
	// +optional
	IPFamily *corev1.IPFamily `json:"ipFamily,omitempty"`

	// ipFamilyPolicy specifies the IP family policy for services in this datacenter.
	// Supports: SingleStack, PreferDualStack, RequireDualStack.
	// +kubebuilder:validation:Enum=SingleStack;PreferDualStack;RequireDualStack
	// +optional
	IPFamilyPolicy *corev1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`

	// ipFamilies specifies the IP families to use for services in this datacenter.
	// Supports: IPv4, IPv6.
	// When set, this overrides the single IPFamily field for service configuration.
	// +optional
	IPFamilies []corev1.IPFamily `json:"ipFamilies,omitempty"`

	// exposeOptions specifies parameters related to exposing ScyllaDBDatacenter backends.
	// +optional
	ExposeOptions *ExposeOptions `json:"exposeOptions,omitempty"`

	// rackTemplate provides a template for every rack.
	// Every rack inherits properties specified in the template, unless it's overwritten on the rack level.
	// +optional
	RackTemplate *RackTemplate `json:"rackTemplate,omitempty"`

	// racks specify the racks in the datacenter.
	Racks []RackSpec `json:"racks"`

	// disableAutomaticOrphanedNodeReplacement controls if automatic orphan node replacement should be disabled.
	// +optional
	DisableAutomaticOrphanedNodeReplacement *bool `json:"disableAutomaticOrphanedNodeReplacement,omitempty"`

	// minTerminationGracePeriodSeconds specifies minimum duration in seconds to wait before every drained node is
	// terminated. This gives time to potential load balancer in front of a node to notice that node is not ready anymore
	// and stop forwarding new requests.
	// This applies only when node is terminated gracefully.
	// If not provided, Operator will determine this value.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	MinTerminationGracePeriodSeconds *int32 `json:"minTerminationGracePeriodSeconds,omitempty"`

	// minReadySeconds specifies the minimum number of seconds for which a newly created ScyllaDB node should be ready
	// for it to be considered available.
	// When used to control load balanced traffic, this can give the load balancer in front of a node enough time to
	// notice that the node is ready and start forwarding traffic in time. Because it all depends on timing, the order
	// is not guaranteed and, if possible, you should use readinessGates instead.
	// If not provided, Operator will determine this value.
	// +optional
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// readinessGates specifies custom readiness gates that will be evaluated for every ScyllaDB Pod readiness.
	// It's projected into every ScyllaDB Pod as its readinessGate. Refer to upstream documentation to learn more
	// about readiness gates.
	// +optional
	ReadinessGates []corev1.PodReadinessGate `json:"readinessGates,omitempty"`
}

type ObjectTemplateMetadata struct {
	// labels specify a custom key value map that gets merged with managed object labels.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations specify a custom key value map that gets merged with managed object annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type RackTemplate struct {
	// nodes specify the desired number of nodes in rack.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// placement describes restrictions for the nodes ScyllaDB is scheduled on.
	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// topologyLabelSelector specifies a label selector which will be used to target nodes at specified topology constraints.
	// Datacenter topologyLabelSelector is merged with rack topologyLabelSelector and then converted into nodeAffinity
	// targeting nodes having specified topology.
	// +optional
	TopologyLabelSelector map[string]string `json:"topologyLabelSelector,omitempty"`

	// scyllaDB specifies ScyllaDB properties for this rack.
	// These override the settings set on Datacenter level.
	// +optional
	ScyllaDB *ScyllaDBTemplate `json:"scyllaDB,omitempty"`

	// scyllaDBManagerAgent specifies ScyllaDB Manager Agent properties for this rack.
	// These override the settings set on Datacenter level.
	// +optional
	ScyllaDBManagerAgent *ScyllaDBManagerAgentTemplate `json:"scyllaDBManagerAgent,omitempty"`

	// exposeOptions specifies rack-specific parameters related to exposing ScyllaDBDatacenter backends.
	// +optional
	ExposeOptions *RackExposeOptions `json:"exposeOptions,omitempty"`
}

// ScyllaDBTemplate allows overriding a subset of ScyllaDB settings.
type ScyllaDBTemplate struct {
	// resources specify requirements for the ScyllaDB container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// storage specifies requirements for the containers
	// +optional
	Storage *StorageOptions `json:"storage,omitempty"`

	// customConfigMapRef specifies a reference to custom ScyllaDB configuration stored as ConfigMap.
	// Overrides upper level settings.
	// +optional
	CustomConfigMapRef *string `json:"customConfigMapRef,omitempty"`

	// volumes specify a list of volumes appended to ScyllaDB Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// volumeMounts specify a list of volume mounts appended to ScyllaDB container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// ScyllaDBManagerAgentTemplate allows to override a subset of ScyllaDBManagerAgent settings.
type ScyllaDBManagerAgentTemplate struct {
	// resources specify requirements for the ScyllaDB Manager Agent container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// customConfigSecretRef specifies a reference to custom ScyllaDB Manager Agent configuration stored as Secret.
	// +optional
	CustomConfigSecretRef *string `json:"customConfigSecretRef,omitempty"`

	// volumes specify a list of volumes appended to ScyllaDB Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// volumeMounts specify a list of volume mounts appended to ScyllaDB Manager Agent container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// RackSpec is the desired state for a ScyllaDB Rack.
type RackSpec struct {
	RackTemplate `json:",inline"`

	// name specifies the name of the ScyllaDB Rack. Used as rack name in GossipingPropertyFileSnitch.
	// This field is immutable.
	Name string `json:"name"`
}

// ScyllaDB holds configuration options related to ScyllaDB.
type ScyllaDB struct {
	// image holds a reference to the ScyllaDB container image.
	Image string `json:"image"`

	// externalSeeds specifies the external seeds to propagate to ScyllaDB binary on startup as "seeds" parameter of seed-provider.
	// +optional
	ExternalSeeds []string `json:"externalSeeds,omitempty"`

	// alternatorOptions designates this cluster an Alternator cluster.
	// +optional
	AlternatorOptions *AlternatorOptions `json:"alternatorOptions,omitempty"`

	// additionalScyllaDBArguments specify a list of arguments appended to the ScyllaDB binary during startup.
	// When set, ScyllaDB may behave unexpectedly, and every such setup is considered unsupported.
	// Instead, consider using customConfigMapRef for setting custom ScyllaDB configuration options.
	// +optional
	AdditionalScyllaDBArguments []string `json:"additionalScyllaDBArguments,omitempty"`

	// developerMode determines if the cluster runs in developer-mode.
	// +optional
	EnableDeveloperMode *bool `json:"enableDeveloperMode,omitempty"`
}

// StorageOptions describes options of storage.
type StorageOptions struct {
	// metadata controls shared metadata for the volume claim for this rack.
	// At this point, the values are applied only for the initial claim and are not reconciled during its lifetime.
	// Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// capacity describes the requested size of each persistent volume.
	Capacity string `json:"capacity"`

	// storageClassName specifies the name of a storageClass to request.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type TLSCertificateType string

const (
	TLSCertificateTypeOperatorManaged TLSCertificateType = "OperatorManaged"
	TLSCertificateTypeUserManaged     TLSCertificateType = "UserManaged"
)

type UserManagedTLSCertificateOptions struct {
	// secretName references a kubernetes.io/tls type secret containing the TLS cert and key.
	SecretName string `json:"secretName"`
}

type OperatorManagedTLSCertificateOptions struct {
	// additionalDNSNames represents external DNS names that the certificates should be signed for.
	// +optional
	AdditionalDNSNames []string `json:"additionalDNSNames,omitempty"`

	// additionalIPAddresses represents external IP addresses that the certificates should be signed for.
	// +optional
	AdditionalIPAddresses []string `json:"additionalIPAddresses,omitempty"`
}

type TLSCertificate struct {
	// type determines the source of this certificate.
	Type TLSCertificateType `json:"type"`

	// userManagedOptions specifies options for certificates manged by users.
	// +optional
	UserManagedOptions *UserManagedTLSCertificateOptions `json:"userManagedOptions,omitempty"`

	// operatorManagedOptions specifies options for certificates manged by the operator.
	// +optional
	OperatorManagedOptions *OperatorManagedTLSCertificateOptions `json:"operatorManagedOptions,omitempty"`
}

// AlternatorOptions holds Alternator settings.
type AlternatorOptions struct {
	// writeIsolation specifies the isolation level.
	WriteIsolation string `json:"writeIsolation,omitempty"`

	// servingCertificate references a TLS certificate for serving secure traffic.
	// +kubebuilder:default:={type:"OperatorManaged"}
	// +optional
	ServingCertificate *TLSCertificate `json:"servingCertificate,omitempty"`
}

// ScyllaDBManagerAgent holds configuration options related to ScyllaDB Manager Agent.
type ScyllaDBManagerAgent struct {
	// image holds a reference to the ScyllaDB Manager Agent container image.
	// +optional
	Image *string `json:"image,omitempty"`
}

type PodIPSourceType string

const (
	// StatusPodIPSource specifies that the PodIP is taken from Pod.Status.PodIP
	StatusPodIPSource PodIPSourceType = "Status"
)

type PodIPInterfaceOptions struct {
	// interfaceName specifies interface name within a Pod from which address is taken from.
	InterfaceName string `json:"interfaceName,omitempty"`
}

// PodIPAddressOptions hold options related to Pod IP address.
type PodIPAddressOptions struct {
	// sourceType specifies source of the Pod IP.
	// +kubebuilder:default:="Status"
	Source PodIPSourceType `json:"source"`
}

type BroadcastAddressType string

const (
	// BroadcastAddressTypePodIP selects the IP address from Pod.
	BroadcastAddressTypePodIP BroadcastAddressType = "PodIP"

	// BroadcastAddressTypeServiceClusterIP selects the IP address from Service.spec.ClusterIP
	BroadcastAddressTypeServiceClusterIP BroadcastAddressType = "ServiceClusterIP"

	// BroadcastAddressTypeServiceLoadBalancerIngress selects the IP address or Hostname from Service.status.ingress[0]
	BroadcastAddressTypeServiceLoadBalancerIngress BroadcastAddressType = "ServiceLoadBalancerIngress"
)

const (
	ScyllaDBDatacenterDefaultClientsBroadcastAddressType = BroadcastAddressTypeServiceClusterIP
	ScyllaDBDatacenterDefaultNodesBroadcastAddressType   = BroadcastAddressTypeServiceClusterIP
)

// BroadcastOptions hold options related to address broadcasted by ScyllaDB node.
type BroadcastOptions struct {
	// type specifies the address type that is broadcasted.
	Type BroadcastAddressType `json:"type"`

	// podIP holds options related to Pod IP address.
	// +optional
	PodIP *PodIPAddressOptions `json:"podIP,omitempty"`
}

// NodeBroadcastOptions hold options related to addresses broadcasted by ScyllaDB node.
type NodeBroadcastOptions struct {
	// nodes specify options related to the address that is broadcasted for communication with other nodes.
	// This field controls the `broadcast_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"PodIP"}
	Nodes BroadcastOptions `json:"nodes"`

	// clients specify options related to the address that is broadcasted for communication with clients.
	// This field controls the `broadcast_rpc_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"PodIP"}
	Clients BroadcastOptions `json:"clients"`
}

type NodeServiceType string

const (
	// NodeServiceTypeHeadless means nodes will be exposed via Headless Service.
	NodeServiceTypeHeadless NodeServiceType = "Headless"

	// NodeServiceTypeClusterIP means nodes will be exposed via ClusterIP Service.
	NodeServiceTypeClusterIP NodeServiceType = "ClusterIP"

	// NodeServiceTypeLoadBalancer means nodes will be exposed via LoadBalancer Service.
	NodeServiceTypeLoadBalancer NodeServiceType = "LoadBalancer"
)

type NodeServiceTemplate struct {
	ObjectTemplateMetadata `json:",inline"`

	// type specifies the Kubernetes Service type.
	Type NodeServiceType `json:"type"`

	// externalTrafficPolicy controls value of service.spec.externalTrafficPolicy of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`

	// allocateLoadBalancerNodePorts controls value of service.spec.allocateLoadBalancerNodePorts of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	AllocateLoadBalancerNodePorts *bool `json:"allocateLoadBalancerNodePorts,omitempty"`

	// loadBalancerClass controls value of service.spec.loadBalancerClass of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// internalTrafficPolicy controls value of service.spec.internalTrafficPolicy of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	InternalTrafficPolicy *corev1.ServiceInternalTrafficPolicy `json:"internalTrafficPolicy,omitempty"`
}

// RackExposeOptions hold options related to exposing rack of ScyllaDBDatacenter.
type RackExposeOptions struct {
	// nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node in given rack.
	// +optional
	NodeService *RackNodeServiceTemplate `json:"nodeService,omitempty"`
}

// RackNodeServiceTemplate hold options related to properties of rack Service.
type RackNodeServiceTemplate struct {
	ObjectTemplateMetadata `json:",inline"`
}

// ExposeOptions hold options related to exposing ScyllaDBDatacenter backends.
type ExposeOptions struct {
	// cql specifies expose options for CQL SSL backend.
	// +optional
	CQL *CQLExposeOptions `json:"cql,omitempty"`

	// nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node.
	// +kubebuilder:default:={type:"ClusterIP"}
	NodeService *NodeServiceTemplate `json:"nodeService,omitempty"`

	// BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.
	BroadcastOptions *NodeBroadcastOptions `json:"broadcastOptions,omitempty"`
}

// CQLExposeOptions hold options related to exposing CQL backend.
type CQLExposeOptions struct {
	// ingress specifies an Ingress configuration options.
	// If provided and enabled, Ingress objects routing to CQL SSL port are generated for each ScyllaDB node
	// with the following options.
	Ingress *CQLExposeIngressOptions `json:"ingress,omitempty"`
}

// CQLExposeIngressOptions defines configuration options for Ingress objects associated with cluster nodes.
type CQLExposeIngressOptions struct {
	ObjectTemplateMetadata `json:",inline"`

	// ingressClassName specifies Ingress class name.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`
}

// Placement holds configuration options related to scheduling.
type Placement struct {
	// nodeAffinity describes node affinity scheduling rules for the Pod.
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// podAffinity describes Pod affinity scheduling rules.
	// +optional
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`

	// podAntiAffinity describes Pod anti-affinity scheduling rules.
	// +optional
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// tolerations describe Pod toleration rules.
	// This allows the Pod to tolerate any taint that matches the triple <key,value,effect>
	// using the matching operator.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// RackStatus is the status of a ScyllaDB Rack
type RackStatus struct {
	// name specifies the name of datacenter this status describes.
	Name string `json:"name,omitempty"`

	// version specifies the current version of ScyllaDB in use.
	CurrentVersion string `json:"currentVersion"`

	// updatedVersion specifies the updated version of ScyllaDB.
	UpdatedVersion string `json:"updatedVersion"`

	// nodes specify the total number of nodes requested in rack.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// currentNodes specify the total number of nodes created in rack.
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes specify the number of nodes matching the current spec in rack.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes specify the total number of ready nodes in rack.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// availableNodes specify the total number of available nodes in rack.
	// +optional
	AvailableNodes *int32 `json:"availableNodes,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`
}

// ScyllaDBDatacenterStatus defines the observed state of ScyllaDBDatacenter.
type ScyllaDBDatacenterStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBDatacenter. It corresponds to the
	// ScyllaDBDatacenter's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBDatacenter state.
	// To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// version specifies the current version of ScyllaDB in use.
	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`

	// updatedVersion specifies the updated version of ScyllaDB.
	// +optional
	UpdatedVersion string `json:"updatedVersion,omitempty"`

	// nodes specify the total number of nodes requested in datacenter.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// currentNodes specify the total number of nodes created in datacenter.
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes specify the number of nodes matching the current spec in datacenter.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes specify the total number of ready nodes in datacenter.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// availableNodes specify the total number of available nodes in datacenter.
	// +optional
	AvailableNodes *int32 `json:"availableNodes,omitempty"`

	// racks reflect the status of datacenter racks.
	Racks []RackStatus `json:"racks"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AVAILABLE",type=string,JSONPath=".status.conditions[?(@.type=='Available')].status"
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ScyllaDBDatacenter defines a monitoring instance for ScyllaDB clusters.
type ScyllaDBDatacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this ScyllaDBDatacenter.
	Spec ScyllaDBDatacenterSpec `json:"spec,omitempty"`

	// status specifies the current status of this ScyllaDBDatacenter.
	Status ScyllaDBDatacenterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBDatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBDatacenter `json:"items"`
}

// GetIPFamily returns the IP family for this datacenter.
// It defaults to IPv4 if not specified.
func (s *ScyllaDBDatacenterSpec) GetIPFamily() corev1.IPFamily {
	if s.IPFamily != nil {
		return *s.IPFamily
	}
	return corev1.IPv4Protocol
}
