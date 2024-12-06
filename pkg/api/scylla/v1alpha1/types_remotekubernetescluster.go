package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClientHealthcheckProbes struct {
	// periodSeconds specifies the period of client healthcheck probes.
	PeriodSeconds int32 `json:"periodSeconds"`
}

type RemoteKubernetesClusterSpec struct {
	// kubeconfigSecretRef is a reference to a secret keeping kubeconfig allowing to connect to remote Kubernetes cluster.
	KubeconfigSecretRef corev1.SecretReference `json:"kubeconfigSecretRef"`

	// healthcheckProbes hold client healthcheck probes settings.
	// +kubebuilder:default:={"periodSeconds": 60}
	// +optional
	ClientHealthcheckProbes *ClientHealthcheckProbes `json:"clientHealthcheckProbes"`
}

type RemoteKubernetesClusterStatus struct {
	// observedGeneration is the most recent generation observed for this RemoteKubernetesCluster. It corresponds to the
	// RemoteKubernetesCluster's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing RemoteKubernetesCluster state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=remotekubernetesclusters,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AVAILABLE",type=string,JSONPath=".status.conditions[?(@.type=='Available')].status"
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type RemoteKubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of the RemoteKubernetesCluster.
	Spec RemoteKubernetesClusterSpec `json:"spec,omitempty"`

	// status defines the observed state of the RemoteKubernetesCluster.
	Status RemoteKubernetesClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RemoteKubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteKubernetesCluster `json:"items"`
}
