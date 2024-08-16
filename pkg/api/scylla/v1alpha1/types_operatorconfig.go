// Copyright (C) 2021 ScyllaDB

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ScyllaOperatorConfigSpec struct {
	// scyllaUtilsImage is a ScyllaDB image used for running ScyllaDB utilities.
	ScyllaUtilsImage string `json:"scyllaUtilsImage"`

	// unsupportedBashToolsImageOverride allows to adjust a generic Bash image with extra tools used by the operator
	// for auxiliary purposes.
	// Setting this field renders your cluster unsupported. Use at your own risk.
	// +optional
	UnsupportedBashToolsImageOverride *string `json:"unsupportedBashToolsImageOverride,omitempty"`

	// unsupportedGrafanaImageOverride allows to adjust Grafana image used by the operator
	// for testing, dev or emergencies.
	// Setting this field renders your cluster unsupported. Use at your own risk.
	// +optional
	UnsupportedGrafanaImageOverride *string `json:"unsupportedGrafanaImageOverride,omitempty"`

	// unsupportedPrometheusVersionOverride allows to adjust Prometheus version used by the operator
	// for testing, dev or emergencies.
	// Setting this field renders your cluster unsupported. Use at your own risk.
	// +optional
	UnsupportedPrometheusVersionOverride *string `json:"unsupportedPrometheusVersionOverride,omitempty"`
}

type ScyllaOperatorConfigStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaOperatorConfig. It corresponds to the
	// ScyllaOperatorConfig's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// scyllaDBUtilsImage is the ScyllaDB image used for running ScyllaDB utilities.
	// +optional
	ScyllaDBUtilsImage *string `json:"scyllaDBUtilsImage"`

	// bashToolsImage is a generic Bash image with extra tools used by the operator for auxiliary purposes.
	// +optional
	BashToolsImage *string `json:"bashToolsImage,omitempty"`

	// grafanaImage is the image used by the operator to create a Grafana instance.
	// +optional
	GrafanaImage *string `json:"grafanaImage,omitempty"`

	// prometheusVersion is the Prometheus version used by the operator to create a Prometheus instance.
	// +optional
	PrometheusVersion *string `json:"prometheusVersion"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=scyllaoperatorconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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
