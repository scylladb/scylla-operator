// Copyright (C) 2025 ScyllaDB

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type NodeStatus string

const (
	NodeStatusUp   NodeStatus = "UP"
	NodeStatusDown NodeStatus = "DOWN"
)

type ObservedNodeStatus struct {
	//  HostID is the ScyllaDB node's host ID.
	HostID string `json:"hostID"`

	// Status is the status of the node.
	Status NodeStatus `json:"status"`
}

type NodeStatusReport struct {
	// Ordinal is the ordinal of the reporter node within its rack.
	Ordinal int `json:"ordinal"`

	// HostID is the ScyllaDB reporter node's host ID.
	// +optional
	HostID *string `json:"hostID,omitempty"`

	// ObservedNodes holds the list of node statuses as observed by the reporter node.
	// +optional
	ObservedNodes []ObservedNodeStatus `json:"observedNodes,omitempty"`
}

type RackNodesStatusReport struct {
	// Name is the name of the rack.
	Name string `json:"name"`

	// Nodes holds the list of node status reports collected from nodes from this rack.
	Nodes []NodeStatusReport `json:"nodes"`
}

// ScyllaDBDatacenterNodesStatusReport serves as an internal interface for propagating and aggregating node statuses observed by nodes in a ScyllaDB datacenter.
// It is used by ScyllaDB Operator for coordinating operations such as topology changes and is not intended for direct user interaction.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScyllaDBDatacenterNodesStatusReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DatacenterName is the name of the reported ScyllaDB datacenter.
	DatacenterName string `json:"datacenterName"`

	// Racks holds the list of rack status reports.
	Racks []RackNodesStatusReport `json:"racks"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBDatacenterNodesStatusReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ScyllaDBDatacenterNodesStatusReport `json:"items"`
}
