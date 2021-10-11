// Copyright (C) 2021 ScyllaDB

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ScyllaOperatorConfigSpec struct {
	// scyllaUtilsImage is an Scylla image used in containers requiring running scripts from Scylla image.
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

type ScyllaOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScyllaOperatorConfigSpec   `json:"spec,omitempty"`
	Status ScyllaOperatorConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScyllaOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaOperatorConfig `json:"items"`
}
