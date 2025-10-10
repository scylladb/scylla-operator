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
	// +kubebuilder:validation:Enum="UP";"DOWN"
	Status NodeStatus `json:"status"`
}

// NodeStatusReport holds a report for a single node.
type NodeStatusReport struct {
	// Ordinal is the ordinal of the ScyllaDB node within its rack.
	Ordinal int `json:"ordinal"`

	// HostID is the ScyllaDB node's host ID.
	// +optional
	HostID *string `json:"hostID,omitempty"`

	// ObservedNodes holds the list of node statuses as observed by this node.
	// +optional
	ObservedNodes []ObservedNodeStatus `json:"observedNodes,omitempty"`
}

type RackStatusReport struct {
	// Name is the name of the rack.
	Name string `json:"name"`

	// Nodes holds the list of ScyllaDB node status reports from this rack.
	Nodes []NodeStatusReport `json:"nodes"`
}

// DatacenterStatusReport holds a report for a single datacenter.
type DatacenterStatusReport struct {
	// Name is the name of the datacenter.
	Name string `json:"name"`

	// Racks holds the list of rack status reports.
	Racks []RackStatusReport `json:"racks"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBStatusReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Datacenters holds the list of datacenter reports.
	// +optional
	Datacenters []DatacenterStatusReport `json:"datacenters,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBStatusReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ScyllaDBStatusReport `json:"items"`
}
