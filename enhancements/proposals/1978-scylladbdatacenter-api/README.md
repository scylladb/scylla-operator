# Gradually replace ScyllaCluster API with ScyllaDBDatacenter API

## Summary

This proposal aims to introduce a new API called `v1alpha1.ScyllaDBDatacenter` that will replace the existing `v1.ScyllaCluster`.
The new API is going to provide the same set of features and will contain fixes for mistakes we made in the past, 
as well as several UX improvements.

## Motivation

`v1.ScyllaCluster` has several drawbacks, some of which could be resolved within the spec,
but naming issues of the object itself cannot.
By introducing a new API, we can solve issues with object format and naming at once.

### Goals

- List all issues with the existing `v1.ScyllaCluster` API.
- Define the new `v1alpha1.ScyllaDBDatacenter` API fixing the above list.
- Explain the migration to the new API.

### Non-Goals

- Introduce new features alongside new API.

## Proposal

I propose to add a new `v1alpha1.ScyllaDBDatacenter` to the existing `scylla.scylladb.com` API group.
Existing controllers reconciling `v1.ScyllaCluster` are going to switch to reconcile the new API,
and we'll add a controller that creates a backing `v1alpha1.ScyllaDBDatacneter` for every existing `v1.ScyllaCluster`.
This will allow existing users to continue using the API they are used to.
For the alpha phase the docs will keep using `v1.ScyllaCluster` API,
and we'll add a dedicated page with an example of `v1alpha1.ScyllaDBDatacenter` with a disclaimer about the features available,
along with the generated API reference.

### User Stories

#### Existing users of `v1.ScyllaCluster`

As a user of `v1.ScyllaCluster` I want to continue using it. This change must not interrupt anyhow.

#### New users

As a new user, I should learn about `v1alpha1.ScyllaDBDatacenter` from additional examples and documentation 
and have support for all existing features from the `v1.ScyllaCluster` API.

### Risks and Mitigations

Not known.

## Design Details

### Minor changes
The following list of minor issues will be fixed in the new API. Major ones are described in separate paragraphs.
- `v1.ScyllaCluster` provides a single datacenter spec, while ScyllaDB can be deployed in multiple datacenters.
  Having `Cluster` in the name is wrong, as this resource represents only a single DC.
- Some time ago, the name of a database we are managing was changed to ScyllaDB. Resources should reflect that.
  Similarly, references to Scylla Manager Agent are renamed to ScyllaDB Manager Agent, even though the naming change isn't completed.
- Instead of having a single field specifying the container image of used components,
  there are two fields, one for the repository and the second for the version, which prevents from using 
  full format of container images [#821](https://github.com/scylladb/scylla-operator/issues/821).
  This unnecessary split is error-prone.
- Kubernetes API Convention suggests using arrays instead of maps. The status of racks doesn't follow that rule.
- `cpuSet` is confusing, as it doesn't change anything related to cpuset, it only controls stricter validation of allocated resources.
  This is unnecessary when there is documentation about performance tuning containing these recommendations.
  This field will be removed and validation will be relaxed. [#508](https://github.com/scylladb/scylla-operator/issues/508)
- We initially thought that `hostNetworking` helps with performance, but later we discovered we were wrong.
  As we only see the downsides of using it, `hostNetworking` support will be discontinued, and setting it becomes deprecated
  and discouraged by not having a field to set it in API. [#1378](https://github.com/scylladb/scylla-operator/issues/1378)
  To preserve backward compatibility controllers will support enabling it by setting special 
  annotation `internal.scylla-operator.scylladb.com/enable-host-networking` to `true` on `v1alpha1.ScyllaDBDatacenter`.
  The controller migrating from `v1.ScyllaCluster` to `v1alpha1.ScyllaDBDatacenter` will set this annotation when required.
- `sysctls` doesn't belong to ScyllaDB cluster nor datacenter, these are configuration options for nodes. 
  A better place to have them is NodeConfig. Until it's available in NodeConfig, we will support setting them 
  via `internal.scylla-operator.scylladb.com/set-sysctls` annotation, in a JSON encoded array of `"key": "value"` format. [#749](https://github.com/scylladb/scylla-operator/issues/749)

### Helm chart

The existing `scylla` helm chart will use the existing `v1.ScyllaCluster`. It has several differences between what's in CRD
and what's in the chart templates, hence it cannot be simply mapped one-to-one.
A new helm chart called `scylladb-datacenter` will simply copy `spec` from `values` to a new CRD solving this issue and making it
easily extensible, while keeping the schema of CRD and helm schema in sync.

### Scylla Manager integration

Scylla Manager tasks do not fit into a single datacenter, as Manager tasks work on the entire cluster.
These are not yet transferred from the spec and status of the old API until a better alternative for these is available.
The existing controller reconciling these will continue using `v1.ScyllaCluster`, users wanting to use these,
won't be able to transition to the new API until an extension is available.
Extending the manager integration may be addressed in a future proposal that rethinks the overall approach in more detail.

### Topology placement

Currently, to provide placement rules for a rack, users have to provide a very lengthy `nodeAffinity`.
Even the simplest rules require providing 10'ish lines with 4'ish indentation levels.
This makes the entire resource huge and UX isn't great.
To improve it, the new field `topologyLabelSelector` added on the datacenter and rack level,
provides a label selector which will be translated into placement nodeAffinity rules selecting nodes matching these labels.
Label selectors from the datacenter and rack level will be merged into the final node affinity. 
In case of a collision, rack ones take precedence.

Given following spec:
```yaml
spec:
  rackTemplate:
    topologyLabelSelector:
      scylla.scylladb.com/node-type: scylla
  racks:
  - name: a
    topologyLabelSelector:
      topology.kubernetes.io/zone: us-east1a
```

Results in rack `a` having `nodeAffinity`:
```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: scylla.scylladb.com/node-type
        operator: In
        values:
        - scylla
      - key: topology.kubernetes.io/zone
        operator: In
        values:
        - us-east1a
```

### Uniform datacenter

ScyllaDB recommendation is to have uniform racks. 
I propose to extend the API so that the racks can be defined uniformly in a simplified manner, 
while preserving the existing flexibility of having non-uniform racks by overriding it with dedicated specifications, if needed.

#### Rack template

New field `rackTemplate` on datacenter level, allows defining shared properties across all racks.
Racks inherit these properties unless the same is specified on rack level, then overwritten value takes precedence. 

An example of nine node cluster having three racks with uniform sizes and resources.
```yaml
spec:
  rackTemplate:
    nodes: 3
    topologyLabelSelector:
      topology.kubernetes.io/region: us-east1
    scylladb:
      image: docker.io/scylladb/scylla:6.0.1
      resources:
        requests:
          cpu: 1
          memory: 4Gi
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
    scyllaDBManagerAgent:
      image: docker.io/scylladb/scylla-manager-agent:3.3.0
      resources:
        requests:
          cpu: 100m
          memory: 100Mi
  racks:
  - name: a
    topologyLabelSelector:
      topology.kubernetes.io/zone: us-east1a
  - name: b
    topologyLabelSelector:
      topology.kubernetes.io/zone: us-east1b
  - name: c
    topologyLabelSelector:
      topology.kubernetes.io/zone: us-east1c

```

### API Spec

Proposed API spec:
```go
const (
	AvailableCondition   = "Available"
	ProgressingCondition = "Progressing"
	DegradedCondition    = "Degraded"
)

// ScyllaDBDatacenterSpec defines the desired state of ScyllaDBDatacenter.
type ScyllaDBDatacenterSpec struct {
	// metadata controls shared metadata for all pods created based on this spec.
	// +optional
	Metadata ObjectTemplateMetadata `json:"metadata,omitempty"`

	// clusterName specifies the name of the scylla cluster.
	// When joining two DCs, their cluster name must match.
	// This field is immutable.
	ClusterName string `json:"clusterName"`

	// datacenterName specifies the name of the scylla datacenter. Used as datacenter name in GossipingPropertyFileSnitch.
	// If empty, it's taken from the 'scylladbdatacenter.metadata.name'.
	// +optional
	DatacenterName string `json:"datacenterName,omitempty"`

	// scyllaDB holds a specification of ScyllaDB.
	ScyllaDB ScyllaDB `json:"scyllaDB"`

	// scyllaManagerAgent holds a specification of ScyllaDB Manager Agent.
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

	// forceRedeploymentReason can be used to force a rolling restart of all racks in this DC by providing a unique string.
	// +optional
	ForceRedeploymentReason string `json:"forceRedeploymentReason,omitempty"`

	// exposeOptions specifies parameters related to exposing ScyllaCluster backends.
	// +optional
	ExposeOptions *ExposeOptions `json:"exposeOptions,omitempty"`

	// rackTemplate provides a template for every rack.
	// Every rack inherits properties specified in the template, unless it's overwritten on the rack level.
	// +optional
	RackTemplate *RackTemplate `json:"rackTemplate,omitempty"`

	// racks specify the racks in the datacenter.
	Racks []RackSpec `json:"racks"`

	// disableAutomaticOrphanedNodeReplacement controls if automatic orphan node replacement should be disabled.
	DisableAutomaticOrphanedNodeReplacement bool `json:"disableAutomaticOrphanedNodeReplacement,omitempty"`

	// minTerminationGracePeriodSeconds specifies minimum duration in seconds to wait before every drained node is
	// terminated. This gives time to potential load balancer in front of a node to notice that node is not ready anymore
	// and stop forwarding new requests.
	// This applies only when node is terminated gracefully.
	// If not provided, Operator will determine this value.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	MinTerminationGracePeriodSeconds *int32 `json:"minTerminationGracePeriodSeconds,omitempty"`

	// minReadySeconds is the minimum number of seconds for which a newly created ScyllaDB node should be ready
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
	// labels is a custom key value map that gets merged with managed object labels.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations is a custom key value map that gets merged with managed object annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type RackTemplate struct {
	// nodes specifies desired number of nodes in rack.
	// +optional
	Nodes *int32 `json:"nodes"`

	// placement describes restrictions for the nodes ScyllaDB is scheduled on.
	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// topologyLabelSelector specifies a label selector which will be used to target nodes at specified topology constraints.
	// Datacenter topologyLabelSelector is merged with rack topologyLabelSelector and then converted into nodeAffinity
	// targeting nodes having specified topology.
	TopologyLabelSelector *metav1.LabelSelector `json:"topologyLabelSelector,omitempty"`

	// scyllaDB defined ScyllaDB properties for this rack.
	// These override the settings set on Datacenter level.
	ScyllaDB *ScyllaDBTemplate `json:"scyllaDB"`

	// scyllaDBManagerAgent specifies ScyllaDB Manager Agent properties for this rack.
	// These override the settings set on Datacenter level.
	ScyllaDBManagerAgent *ScyllaDBManagerAgentTemplate `json:"scyllaDBManagerAgent"`
}

// ScyllaDBTemplate allows overriding a subset of ScyllaDB settings.
type ScyllaDBTemplate struct {
	// resources requirements for the ScyllaDB container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// storage requirements for the containers
	// +optional
	Storage *Storage `json:"storage,omitempty"`

	// customConfigMapRef points to custom ScyllaDB configuration stored as ConfigMap.
	// Overrides upper level settings.
	// +optional
	CustomConfigMapRef *string `json:"customConfigMapRef,omitempty"`

	// volumes added to Scylla Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// volumeMounts to be added to ScyllaDB container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// ScyllaDBManagerAgentTemplate allows to override a subset of ScyllaManagerAgent settings.
type ScyllaDBManagerAgentTemplate struct {
	// resources are requirements for the ScyllaDB Manager Agent container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// customConfigSecretRef points to custom ScyllaDB Manager Agent configuration stored as Secret.
	// +optional
	CustomConfigSecretRef *string `json:"customConfigSecretRef,omitempty"`

	// volumes added to Scylla Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// volumeMounts to be added to Agent container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// RackSpec is the desired state for a ScyllaDB Rack.
type RackSpec struct {
	RackTemplate

	// name is the name of the ScyllaDB Rack. Used as rack name in GossipingPropertyFileSnitch.
	// This field is immutable.
	Name string `json:"name"`
}

// ScyllaDB holds configuration options related to ScyllaDB.
type ScyllaDB struct {
	ScyllaDBTemplate

	// image holds a reference to the ScyllaDB container image.
	Image string `json:"image"`

	// externalSeeds specifies the external seeds to propagate to ScyllaDB binary on startup as "seeds" parameter of seed-provider.
	ExternalSeeds []string `json:"externalSeeds,omitempty"`

	// alternatorOptions designates this cluster an Alternator cluster.
	// +optional
	AlternatorOptions *AlternatorOptions `json:"alternatorOptions,omitempty"`

	// additionalScyllaDBArguments will be appended to the ScyllaDB binary during startup.
	// When set, ScyllaDB may behave unexpectedly, and every such setup is considered unsupported.
	// +optional
	AdditionalScyllaDBArguments []string `json:"additionalScyllaDBArguments,omitempty"`

	// developerMode determines if the cluster runs in developer-mode.
	// +optional
	EnableDeveloperMode *bool `json:"enableDeveloperMode,omitempty"`
}

// Storage describes options of storage.
type Storage struct {
	// metadata controls shared metadata for the volume claim for this rack.
	// At this point, the values are applied only for the initial claim and are not reconciled during its lifetime.
	// Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.
	// +optional
	Metadata ObjectTemplateMetadata `json:"metadata,omitempty"`

	// capacity describes the requested size of each persistent volume.
	Capacity string `json:"capacity"`

	// storageClassName is the name of a storageClass to request.
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
	// +kubebuilder:validation:Enum="OperatorManaged";"UserManaged"
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
	// writeIsolation indicates the isolation level.
	WriteIsolation string `json:"writeIsolation,omitempty"`

	// servingCertificate references a TLS certificate for serving secure traffic.
	// +kubebuilder:default:={type:"OperatorManaged"}
	// +optional
	ServingCertificate *TLSCertificate `json:"servingCertificate,omitempty"`
}

// ScyllaDBManagerAgent holds configuration options related to Scylla Manager Agent.
type ScyllaDBManagerAgent struct {
	ScyllaDBManagerAgentTemplate

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

// BroadcastOptions hold options related to address broadcasted by ScyllaDB node.
type BroadcastOptions struct {
	// type of the address that is broadcasted.
	Type BroadcastAddressType `json:"type"`

	// podIP holds options related to Pod IP address.
	// +optional
	PodIP *PodIPAddressOptions `json:"podIP,omitempty"`
}

// NodeBroadcastOptions hold options related to addresses broadcasted by ScyllaDB node.
type NodeBroadcastOptions struct {
	// nodes specifies options related to the address that is broadcasted for communication with other nodes.
	// This field controls the `broadcast_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"PodIP"}
	Nodes BroadcastOptions `json:"nodes"`

	// clients specifies options related to the address that is broadcasted for communication with clients.
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

	// type is the Kubernetes Service type.
	// +kubebuilder:validation:Required
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

// ExposeOptions hold options related to exposing ScyllaCluster backends.
type ExposeOptions struct {
	// cql specifies expose options for CQL SSL backend.
	// +optional
	CQL *CQLExposeOptions `json:"cql,omitempty"`

	// nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node.
	// +kubebuilder:default:={type:"Headless"}
	NodeService *NodeServiceTemplate `json:"nodeService,omitempty"`

	// BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.
	BroadcastOptions *NodeBroadcastOptions `json:"broadcastOptions,omitempty"`
}

// CQLExposeOptions hold options related to exposing CQL backend.
type CQLExposeOptions struct {
	// ingress is an Ingress configuration options.
	// If provided and enabled, Ingress objects routing to CQL SSL port are generated for each ScyllaDB node
	// with the following options.
	Ingress *IngressOptions `json:"ingress,omitempty"`
}

// IngressOptions defines configuration options for Ingress objects associated with cluster nodes.
type IngressOptions struct {
	ObjectTemplateMetadata `json:",inline"`

	// ingressClassName specifies Ingress class name.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`
}

// Placement holds configuration options related to scheduling.
type Placement struct {
	// nodeAffinity describes node affinity scheduling rules for the pod.
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// podAffinity describes pod affinity scheduling rules.
	// +optional
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`

	// podAntiAffinity describes pod anti-affinity scheduling rules.
	// +optional
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect>
	// using the matching operator.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// RackStatus is the status of a Scylla Rack
type RackStatus struct {
	// name is the name of datacenter this status describes.
	Name string `json:"name,omitempty"`

	// version is the current version of Scylla in use.
	CurrentVersion string `json:"currentVersion"`

	// updatedVersion is the updated version of Scylla.
	UpdatedVersion string `json:"updatedVersion"`

	// nodes is the total number of nodes requested in rack.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// nodes is the total number of nodes created in rack.
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec in rack.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes is the total number of ready nodes in rack.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

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

	// version is the current version of Scylla in use.
	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`

	// updatedVersion is the updated version of Scylla.
	// +optional
	UpdatedVersion string `json:"updatedVersion,omitempty"`

	// nodes is the total number of nodes requested in datacenter.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// nodes is the total number of nodes created in datacenter.
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec in datacenter.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes is the total number of ready nodes in datacenter.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

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

	// status is the current status of this ScyllaDBDatacenter.
	Status ScyllaDBDatacenterStatus `json:"status,omitempty"`
}

```

### Test Plan

All existing E2E's will be changed to use `v1alpha1.ScyllaDBDatacenter`.
Conversion logic between two APIs is going to be covered by unit tests.
Additional E2E to validate converting controller.

### Upgrade / Downgrade Strategy

New CRD needs to be installed upon Scylla Operator update. Our documentation and every release notes cover that.
No specific actions are required for the downgrade.

### Version Skew Strategy

Each `v1.ScyllaCluster` is going to have a backing `v1alpha.ScyllaDBDatacenter` created by the new controller.
`v1alpha.ScyllaDBDatacenter` will be reconciled by the existing controllers. Users can use either of them. 
When rolled back, changes to `v1alpha1.ScyllaDBDatacenters` are not going to be reconciled, 
users have to go back to using `v1.ScyllaCluster`, if they started using `v1alpha1.ScyllaDBDatacenters` instead.

## Implementation History

26.06.2024 - Initial version

## Drawbacks

Such change is not something we couldn't live with, but it helps to reduce the ambiguity of naming mistakes,
as well as reduce the technical debt.
