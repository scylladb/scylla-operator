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
	// Ordinal is the ordinal of the node within its rack.
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

	// Nodes holds the list of node status reports for this rack.
	Nodes []NodeStatusReport `json:"nodes"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScyllaDBDatacenterStatusReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Name is the name of the reported ScyllaDB datacenter.
	Name string `json:"name"`

	// Racks holds the list of rack status reports.
	Racks []RackStatusReport `json:"racks"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBDatacenterStatusReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ScyllaDBDatacenterStatusReport `json:"items"`
}
