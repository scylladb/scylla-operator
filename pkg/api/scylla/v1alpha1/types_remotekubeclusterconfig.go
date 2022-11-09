// Copyright (C) 2021 ScyllaDB

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RemoteKubeClusterConfigSpec struct {
	// kubeConfigSecretRef is a reference to a secret keeping kube config allowing to connect to remote kube cluster.
	KubeConfigSecretRef *corev1.SecretReference `json:"kubeConfigSecretRef,omitempty"`
}

type RemoteKubeClusterConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=remotekubeclusterconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RemoteKubeClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of the operator.
	Spec RemoteKubeClusterConfigSpec `json:"spec,omitempty"`

	// status defines the observed state of the operator.
	Status RemoteKubeClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RemoteKubeClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteKubeClusterConfig `json:"items"`
}
