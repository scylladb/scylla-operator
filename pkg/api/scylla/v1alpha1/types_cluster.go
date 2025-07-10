// Copyright (c) 2024 ScyllaDB.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ScyllaDBClusterDefaultNodeServiceType             = NodeServiceTypeHeadless
	ScyllaDBClusterDefaultClientsBroadcastAddressType = BroadcastAddressTypePodIP
	ScyllaDBClusterDefaultNodesBroadcastAddressType   = BroadcastAddressTypePodIP
)

// ScyllaDBClusterSpec defines the desired state of ScyllaDBCluster.
type ScyllaDBClusterSpec struct {
	// metadata controls shared metadata for all resources created based on this spec.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// clusterName specifies the name of the ScyllaDB cluster.
	// When joining two DCs, their cluster name must match.
	// If empty, it's taken from the 'scylladbcluster.metadata.name'.
	// This field is immutable.
	// +optional
	ClusterName *string `json:"clusterName,omitempty"`

	// scyllaDB holds a specification of ScyllaDB.
	ScyllaDB ScyllaDB `json:"scyllaDB"`

	// scyllaDBManagerAgent holds a specification of ScyllaDB Manager Agent.
	// +optional
	ScyllaDBManagerAgent *ScyllaDBManagerAgent `json:"scyllaDBManagerAgent,omitempty"`

	// forceRedeploymentReason can be used to force a rolling restart of all racks in this DC by providing a unique string.
	// +optional
	ForceRedeploymentReason *string `json:"forceRedeploymentReason,omitempty"`

	// exposeOptions specifies parameters related to exposing ScyllaDBCluster backends.
	// +optional
	ExposeOptions *ScyllaDBClusterExposeOptions `json:"exposeOptions,omitempty"`

	// datacenterTemplate provides a template for every datacenter.
	// Every datacenter inherits properties specified in the template, unless the same field is specified on the datacenter level.
	// Depending on the type of field, values are either merged, appended or overwritten. Struct fields are merged following the same principles.
	// Map fields are merged - on collision most specific one wins. Slices are appended. Primitive types are overwritten.
	// +optional
	DatacenterTemplate *ScyllaDBClusterDatacenterTemplate `json:"datacenterTemplate,omitempty"`

	// datacenters specify the datacenters in the cluster.
	Datacenters []ScyllaDBClusterDatacenter `json:"datacenters"`

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

type ScyllaDBClusterDatacenter struct {
	ScyllaDBClusterDatacenterTemplate `json:",inline"`

	// name is the name of the ScyllaDB datacenter. Used as datacenter name in GossipingPropertyFileSnitch.
	Name string `json:"name"`

	// remoteKubernetesClusterName is a reference to RemoteKubernetesCluster where this datacenter should be deployed.
	RemoteKubernetesClusterName string `json:"remoteKubernetesClusterName"`

	// forceRedeploymentReason can be used to force a rolling restart of all racks in this DC by providing a unique string.
	// +optional
	ForceRedeploymentReason *string `json:"forceRedeploymentReason,omitempty"`
}

type ScyllaDBClusterDatacenterTemplate struct {
	// metadata controls shared metadata for all pods created based on this datacenter.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// placement describes restrictions for the nodes ScyllaDB is scheduled on.
	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// topologyLabelSelector specifies a label selector which will be used to target nodes at specified topology constraints.
	// Datacenter topologyLabelSelector is merged with rack topologyLabelSelector and then converted into nodeAffinity
	// targeting nodes having specified topology.
	// +optional
	TopologyLabelSelector map[string]string `json:"topologyLabelSelector,omitempty"`

	// scyllaDB defines ScyllaDB properties for this datacenter.
	// These override the settings set on cluster level.
	// +optional
	ScyllaDB *ScyllaDBTemplate `json:"scyllaDB,omitempty"`

	// scyllaDBManagerAgent specifies ScyllaDB Manager Agent properties for this datacenter.
	// These override the settings set on cluster level.
	// +optional
	ScyllaDBManagerAgent *ScyllaDBManagerAgentTemplate `json:"scyllaDBManagerAgent,omitempty"`

	// rackTemplate provides a template for every rack.
	// Every rack inherits properties specified in the template, unless it's overwritten on the rack level.
	// +optional
	RackTemplate *RackTemplate `json:"rackTemplate,omitempty"`

	// racks specify the racks in the datacenter.
	// +optional
	Racks []RackSpec `json:"racks,omitempty"`
}

// ScyllaDBClusterNodeBroadcastOptions hold options related to addresses broadcasted by ScyllaDB node.
type ScyllaDBClusterNodeBroadcastOptions struct {
	// nodes specifies options related to the address that is broadcasted for communication with other nodes.
	// This field controls the `broadcast_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"PodIP"}
	Nodes BroadcastOptions `json:"nodes"`

	// clients specifies options related to the address that is broadcasted for communication with clients.
	// This field controls the `broadcast_rpc_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"PodIP"}
	Clients BroadcastOptions `json:"clients"`
}

// ScyllaDBClusterExposeOptions hold options related to exposing ScyllaDBCluster backends.
type ScyllaDBClusterExposeOptions struct {
	// nodeService controls properties of Service dedicated for each ScyllaDBCluster node.
	// +optional
	// +kubebuilder:default:={type:"Headless"}
	NodeService *NodeServiceTemplate `json:"nodeService,omitempty"`

	// BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.
	// +optional
	BroadcastOptions *ScyllaDBClusterNodeBroadcastOptions `json:"broadcastOptions,omitempty"`
}

// ScyllaDBClusterRackStatus is the status of a ScyllaDB rack
type ScyllaDBClusterRackStatus struct {
	// name is the name of rack this status describes.
	Name string `json:"name"`

	// version is the current version of ScyllaDB in use.
	// +optional
	CurrentVersion *string `json:"currentVersion,omitempty"`

	// updatedVersion is the updated version of ScyllaDB.
	// +optional
	UpdatedVersion *string `json:"updatedVersion,omitempty"`

	// nodes is the total number of nodes requested in rack.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// nodes is the total number of nodes created in rack.
	// +optional
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec in rack.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes is the total number of ready nodes in rack.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// availableNodes is the total number of available nodes in rack.
	// +optional
	AvailableNodes *int32 `json:"availableNodes,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`
}

// ScyllaDBClusterDatacenterStatus is the status of a ScyllaDB Datacenter
type ScyllaDBClusterDatacenterStatus struct {
	// name is the name of datacenter this status describes.
	Name string `json:"name"`

	// remoteNamespaceName is the name of the corev1.Namespace where dependant objects are reconciled in remote Kubernetes cluster.
	RemoteNamespaceName *string `json:"remoteNamespaceName,omitempty"`

	// version is the current version of ScyllaDB in use.
	// +optional
	CurrentVersion *string `json:"currentVersion,omitempty"`

	// updatedVersion is the updated version of ScyllaDB.
	// +optional
	UpdatedVersion *string `json:"updatedVersion,omitempty"`

	// nodes is the total number of nodes requested in datacenter.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// nodes is the total number of nodes created in datacenter.
	// +optional
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec in datacenter.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes is the total number of ready nodes in datacenter.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// availableNodes is the total number of available nodes in datacenter.
	// +optional
	AvailableNodes *int32 `json:"availableNodes,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`

	// racks contains rack statuses.
	// +optional
	Racks []ScyllaDBClusterRackStatus `json:"racks,omitempty"`
}

// ScyllaDBClusterStatus defines the observed state of ScyllaDBCluster.
type ScyllaDBClusterStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBCluster. It corresponds to the
	// ScyllaDBCluster's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBCluster state.
	// To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// version is the current version of ScyllaDB in use.
	// +optional
	CurrentVersion *string `json:"currentVersion,omitempty"`

	// updatedVersion is the updated version of ScyllaDB.
	// +optional
	UpdatedVersion *string `json:"updatedVersion,omitempty"`

	// nodes is the total number of nodes requested in cluster.
	// +optional
	Nodes *int32 `json:"nodes,omitempty"`

	// nodes is the total number of nodes created in cluster.
	// +optional
	CurrentNodes *int32 `json:"currentNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec in cluster.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// readyNodes is the total number of ready nodes in cluster.
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// availableNodes is the total number of available nodes in cluster.
	// +optional
	AvailableNodes *int32 `json:"availableNodes,omitempty"`

	// Datacenters reflect the status of datacenters.
	// +optional
	Datacenters []ScyllaDBClusterDatacenterStatus `json:"datacenters,omitempty"`
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

// ScyllaDBCluster defines a monitoring instance for ScyllaDB clusters.
type ScyllaDBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this ScyllaDBCluster.
	Spec ScyllaDBClusterSpec `json:"spec,omitempty"`

	// status is the current status of this ScyllaDBCluster.
	Status ScyllaDBClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDBClusterList holds a list of ScyllaDBCluster.
type ScyllaDBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBCluster `json:"items"`
}
