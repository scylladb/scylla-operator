// Copyright (c) 2022 ScyllaDB.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScyllaDatacenterSpec defines the desired state of Datacenter.
type ScyllaDatacenterSpec struct {
	// scylla holds a specification of Scylla.
	Scylla Scylla `json:"scylla"`

	// scyllaManagerAgent holds a specification of Scylla Manager Agent.
	// +optional
	ScyllaManagerAgent *ScyllaManagerAgent `json:"scyllaManagerAgent,omitempty"`

	// forceRedeploymentReason can be used to force a rolling update of all datacenters by providing a unique string.
	// +optional
	ForceRedeploymentReason string `json:"forceRedeploymentReason,omitempty"`

	// imagePullSecrets is an optional list of references to secrets in the same namespace
	// used for pulling any images used by this spec.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// network holds network configuration.
	// +optional
	Network *Network `json:"network,omitempty"`

	// name is the name of the scylla datacenter. Used in the cassandra-rackdc.properties file.
	DatacenterName string `json:"datacenterName"`

	// dnsDomains specifies a list of DNS domains this cluster is reachable by.
	// These domains are used when setting up the infrastructure, like certificates.
	// +optional
	DNSDomains []string `json:"dnsDomains,omitempty"`

	// exposeOptions specifies parameters related to exposing ScyllaCluster backends.
	// +optional
	ExposeOptions *ExposeOptions `json:"exposeOptions,omitempty"`

	// nodesPerRack specifies how many nodes are deployed in each rack.
	// +optional
	NodesPerRack *int32 `json:"nodesPerRack,omitempty"`

	// racks specify the racks in the datacenter.
	// +optional
	Racks []RackSpec `json:"racks"`

	// placement describes restrictions for the nodes Scylla is scheduled on.
	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// metadata defines custom metadata values added to all child resources of ScyllaCluster.
	// +optional
	Metadata *Metadata `json:"metadata,omitempty"`
}

// RackSpec is the desired state for a Scylla Rack.
type RackSpec struct {
	// name is the name of the Scylla Rack. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`

	// nodes is the number of Scylla instances in this rack.
	// It overrides Datacenter level settings.
	// +optional
	Nodes *int32 `json:"nodes"`

	// +optional
	Scylla *ScyllaOverrides `json:"scylla,omitempty"`

	// +optional
	ScyllaManagerAgent *ScyllaManagerAgentOverrides `json:"scyllaManagerAgent,omitempty"`

	// placement describes restrictions for the nodes Scylla is scheduled on.
	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// +optional
	UnsupportedVolumes []corev1.Volume `json:"unsupportedVolumes,omitempty"`
}

type ScyllaOverrides struct {
	// resources requirements for the Scylla container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// storage requirements for the containers
	// +optional
	Storage *Storage `json:"storage,omitempty"`

	// customConfigMapRef points to custom Scylla configuration stored as ConfigMap.
	// Overrides upper level settings.
	// +optional
	CustomConfigMapRef *corev1.LocalObjectReference `json:"customConfigMapRef,omitempty"`

	// unsupportedVolumeMounts are volume mounts appended to Scylla container.
	// Usage of this field is unsupported and may lead to unexpected behavior.
	// +optional
	UnsupportedVolumeMounts []corev1.VolumeMount `json:"unsupportedVolumeMounts,omitempty"`
}

type ScyllaManagerAgentOverrides struct {
	// resources are requirements for the Scylla Manager Agent container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// customConfigSecretRef points to custom Scylla Manager Agent configuration stored as Secret.
	// +optional
	CustomConfigSecretRef *corev1.LocalObjectReference `json:"customConfigSecretRef,omitempty"`

	// unsupportedVolumeMounts are volume mounts appended to Scylla Manager Agent container.
	// Usage of this field is unsupported and may lead to unexpected behavior.
	// +optional
	UnsupportedVolumeMounts []corev1.VolumeMount `json:"unsupportedVolumeMounts,omitempty"`
}

// Scylla holds configuration options related to ScyllaDB.
type Scylla struct {
	// image holds a reference to the Scylla container image.
	Image string `json:"image"`

	// alternator designates this cluster an Alternator cluster.
	// +optional
	AlternatorOptions *AlternatorOptions `json:"alternatorOptions,omitempty"`

	// scyllaArgs will be appended to the Scylla binary during startup.
	// +optional
	UnsupportedScyllaArgsOverrides []string `json:"unsupportedScyllaArgsOverrides,omitempty"`

	// developerMode determines if the cluster runs in developer-mode.
	// +optional
	EnableDeveloperMode *bool `json:"enableDeveloperMode,omitempty"`
}

// Storage describes options of storage.
type Storage struct {
	// resources represents the minimum resources the data volume should have.
	Resources corev1.ResourceRequirements `json:"resources"`

	// storageClassName is the name of a storageClass to request.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// Metadata defined custom metadata added to child objects.
type Metadata struct {
	// labels is a map of string keys and values that can be used to organize and categorize objects.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations is a string key value map stored with a resource that may be used to store and retrieve arbitrary metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AlternatorOptions holds Alternator settings.
type AlternatorOptions struct {
	// enabled controls if Alternator is enabled.
	// +optional
	Enabled *bool `json:"enabled"`

	// writeIsolation indicates the isolation level.
	WriteIsolation string `json:"writeIsolation,omitempty"`
}

// ScyllaManagerAgent holds configuration options related to Scylla Manager Agent.
type ScyllaManagerAgent struct {
	// image holds a reference to the Scylla Manager Agent container image.
	Image string `json:"image"`
}

// Network holds configuration options related to networking.
type Network struct {
	// dnsPolicy defines how a pod's DNS will be configured.
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
}

// ExposeOptions hold options related to exposing ScyllaCluster backends.
type ExposeOptions struct {
	// cql specifies expose options for CQL SSL backend.
	// +optional
	CQL *CQLExposeOptions `json:"cql,omitempty"`
}

type CQLExposeOptions struct {
	// ingress is an Ingress configuration option.
	// If provided, Ingress objects routing to CQL SSL port are generated for each Scylla node
	// with the following options.
	Ingress *IngressOptions `json:"ingress,omitempty"`
}

// IngressOptions defines configuration options for Ingress objects associated with cluster nodes.
type IngressOptions struct {
	// disabled controls if Ingress object creation is disabled.
	// Unless disabled, there is an Ingress objects created for every Scylla node.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// ingressClassName specifies Ingress class name.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`

	// annotations specify custom annotations added to every Ingress object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
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

// ScyllaDatacenterStatus defines the observed state of ScyllaDatacenter.
type ScyllaDatacenterStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDatacenter. It corresponds to the
	// ScyllaDatacenter's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// racks reflect status of cluster racks.
	Racks map[string]RackStatus `json:"racks,omitempty"`

	// upgrade reflects state of ongoing upgrade procedure.
	Upgrade *UpgradeStatus `json:"upgrade,omitempty"`

	// conditions hold conditions describing ScyllaCluster state.
	// To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	AvailableCondition   = "Available"
	ProgressingCondition = "Progressing"
	DegradedCondition    = "Degraded"
)

// UpgradeStatus contains the internal state of an ongoing upgrade procedure.
// Do not rely on these internal values externally. They are meant for keeping an internal state
// and their values are subject to change within the limits of API compatibility.
type UpgradeStatus struct {
	// state reflects current upgrade state.
	State string `json:"state"`

	// fromVersion reflects from which version ScyllaDatacenter is being upgraded.
	FromVersion string `json:"fromVersion"`

	// toVersion reflects to which version ScyllaDatacenter is being upgraded.
	ToVersion string `json:"toVersion"`

	// systemSnapshotTag is the snapshot tag of system keyspaces.
	SystemSnapshotTag string `json:"systemSnapshotTag,omitempty"`

	// dataSnapshotTag is the snapshot tag of data keyspaces.
	DataSnapshotTag string `json:"dataSnapshotTag,omitempty"`
}

// RackStatus is the status of a Scylla Rack
type RackStatus struct {
	// Image is the current image of Scylla in use.
	Image string `json:"image"`

	// nodes is the current number of nodes requested in the specific Rack
	Nodes *int32 `json:"nodes,omitempty"`

	// readyNodes is the number of ready nodes in the specific Rack
	ReadyNodes *int32 `json:"readyNodes,omitempty"`

	// updatedNodes is the number of nodes matching the current spec.
	// +optional
	UpdatedNodes *int32 `json:"updatedNodes,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`

	// conditions are the latest available observations of a rack's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// replaceAddressFirstBoot holds addresses which should be replaced by new nodes.
	ReplaceAddressFirstBoot map[string]string `json:"replaceAddressFirstBoot,omitempty"`
}

const (
	RackConditionTypeNodeLeaving         = "NodeLeaving"
	RackConditionTypeUpgrading           = "RackUpgrading"
	RackConditionTypeNodeReplacing       = "NodeReplacing"
	RackConditionTypeNodeDecommissioning = "NodeDecommissioning"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDatacenter defines a single datacenter of Scylla cluster.
type ScyllaDatacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this scylla cluster.
	Spec ScyllaDatacenterSpec `json:"spec,omitempty"`

	// status is the current status of this scylla cluster.
	Status ScyllaDatacenterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDatacenterList holds a list of ScyllaDatacenters.
type ScyllaDatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDatacenter `json:"items"`
}
