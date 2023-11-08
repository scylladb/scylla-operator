// Copyright (C) 2021 ScyllaDB

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ScyllaOperatorConfigSpec struct {
	// scyllaUtilsImage is a Scylla image used for running scylla utilities.
	// +kubebuilder:validation:MinLength=1
	ScyllaUtilsImage string `json:"scyllaUtilsImage"`
}

type ScyllaOperatorConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=scyllaoperatorconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaOperatorConfig describes the Scylla Operator configuration.
type ScyllaOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of the operator.
	Spec ScyllaOperatorConfigSpec `json:"spec,omitempty"`

	// status defines the observed state of the operator.
	Status ScyllaOperatorConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaOperatorConfig `json:"items"`
}
