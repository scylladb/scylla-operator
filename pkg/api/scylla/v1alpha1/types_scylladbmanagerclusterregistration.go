// Copyright (C) 2025 ScyllaDB

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type LocalScyllaDBReference struct {
	// kind specifies the type of the resource.
	Kind string `json:"kind"`
	// name specifies the name of the resource in the same namespace.
	Name string `json:"name"`
}

type ScyllaDBManagerClusterRegistrationSpec struct {
	// scyllaDBClusterRef specifies the typed reference to the local ScyllaDB cluster.
	// Supported kind is ScyllaDBDatacenter in scylla.scylladb.com group.
	ScyllaDBClusterRef LocalScyllaDBReference `json:"scyllaDBClusterRef"`
}

type ScyllaDBManagerClusterRegistrationStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBManagerClusterRegistration. It corresponds to the
	// ScyllaDBManagerClusterRegistration's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBManagerClusterRegistration state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// clusterID reflects the internal identification number of the cluster in ScyllaDB Manager state.
	// +optional
	ClusterID *string `json:"clusterID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type ScyllaDBManagerClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of ScyllaDBManagerClusterRegistration.
	Spec ScyllaDBManagerClusterRegistrationSpec `json:"spec,omitempty"`

	// status reflects the observed state of ScyllaDBManagerClusterRegistration.
	Status ScyllaDBManagerClusterRegistrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBManagerClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBManagerClusterRegistration `json:"items"`
}
