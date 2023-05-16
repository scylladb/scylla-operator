package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AvailableCondition   = "Available"
	ProgressingCondition = "Progressing"
	DegradedCondition    = "Degraded"
)

// GrafanaExposeOptions holds options related to exposing Grafana app.
type GrafanaExposeOptions struct {
	// webInterface specifies expose options for the user web interface.
	// +optional
	WebInterface *HTTPSExposeOptions `json:"webInterface,omitempty"`
}

// PrometheusExposeOptions holds options related to exposing Prometheus app.
type PrometheusExposeOptions struct {
	// webInterface specifies expose options for the user web interface.
	// +optional
	WebInterface *HTTPSExposeOptions `json:"webInterface,omitempty"`
}

// IngressOptions defines configuration options for Ingress objects.
type IngressOptions struct {
	// disabled controls if Ingress object creation is disabled.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// ingressClassName specifies Ingress class name.
	IngressClassName string `json:"ingressClassName,omitempty"`

	// annotations specifies custom annotations merged into every Ingress object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// dnsDomains is a list of DNS domains this ingress is reachable by.
	// +optional
	DNSDomains []string `json:"dnsDomains,omitempty"`
}

// HTTPSExposeOptions holds options related to exposing HTTPS backend.
type HTTPSExposeOptions struct {
	// ingress is an Ingress configuration options.
	// +optional
	Ingress *IngressOptions `json:"ingress,omitempty"`
}

// PlacementSpec defines pod placement.
// TODO: move this to corev1.Affinity in v1alpha2
type PlacementSpec struct {
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

// Storage holds the storage options.
type Storage struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	//
	// Deprecated: Use volumeClaimTemplate.metadata.labels on volumeClaimTemplate.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	//
	// Deprecated: Use volumeClaimTemplate.metadata.annotations.
	Annotations map[string]string `json:"annotations,omitempty"`

	// volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.
	// +optional
	VolumeClaimTemplate corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
}

// PrometheusSpec holds the spec prometheus options.
type PrometheusSpec struct {
	// placement describes restrictions for the nodes Prometheus is scheduled on.
	// +optional
	Placement *PlacementSpec `json:"placement,omitempty"`

	// resources the Prometheus container will use.
	Resources corev1.ResourceRequirements `json:"resources"`

	// exposeOptions specifies options for exposing Prometheus UI.
	// +optional
	ExposeOptions *PrometheusExposeOptions `json:"exposeOptions,omitempty"`

	// storage describes the underlying storage that Prometheus will consume.
	// +optional
	Storage *Storage `json:"storage"`
}

// GrafanaAuthentication holds the options to configure Grafana authentication.
type GrafanaAuthentication struct {
	// insecureEnableAnonymousAccess allows access to Grafana without authentication.
	// +optional
	InsecureEnableAnonymousAccess bool `json:"insecureEnableAnonymousAccess,omitempty"`
}

// GrafanaSpec holds the options to configure Grafana.
type GrafanaSpec struct {
	// placement describes restrictions for the nodes Grafana is scheduled on.
	// +optional
	Placement *PlacementSpec `json:"placement,omitempty"`

	// resources the Grafana container will use.
	Resources corev1.ResourceRequirements `json:"resources"`

	// exposeOptions specifies options for exposing Grafana UI.
	// +optional
	ExposeOptions *GrafanaExposeOptions `json:"exposeOptions,omitempty"`

	// servingCertSecretName is the name of the secret holding a serving cert-key pair.
	// If not specified, the operator will create a self-signed CA that creates
	// the default serving cert-key pair.
	// +optional
	ServingCertSecretName string `json:"servingCertSecretName,omitempty"`

	// authentication hold the authentication options for accessing Grafana.
	// +optional
	Authentication GrafanaAuthentication `json:"authentication,omitempty"`
}

// Components holds the options to configure individual applications.
type Components struct {
	// prometheus holds configuration for the prometheus instance, if any.
	// +optional
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`

	// grafana holds configuration for the grafana instance, if any.
	// +optional
	Grafana *GrafanaSpec `json:"grafana,omitempty"`
}

// ScyllaDBMonitoringType describes the platform type of the monitoring setup.
// +kubebuilder:validation:Enum="SaaS";"Platform"
type ScyllaDBMonitoringType string

const (
	// ScyllaDBMonitoringTypePlatform defines ScyllaDB monitoring setup that includes a view of the infrastructure.
	ScyllaDBMonitoringTypePlatform ScyllaDBMonitoringType = "Platform"

	// ScyllaDBMonitoringTypeSAAS defines ScyllaDB monitoring setup focused only on the ScyllaDB service.
	ScyllaDBMonitoringTypeSAAS ScyllaDBMonitoringType = "SaaS"
)

// ScyllaDBMonitoringSpec defines the desired state of ScyllaDBMonitoring.
type ScyllaDBMonitoringSpec struct {
	// endpointsSelector select which Endpoints should be scraped.
	// For local ScyllaDB clusters or datacenters, this is the same selector as if you were trying to select member Services.
	// For remote ScyllaDB clusters, this can select any endpoints that are created manually or for a Service without selectors.
	// +kubebuilder:validation:Required
	EndpointsSelector metav1.LabelSelector `json:"endpointsSelector"`

	// components hold additional config for the monitoring components in use.
	Components *Components `json:"components"`

	// type determines the platform type of the monitoring setup.
	// +kubebuilder:default:="SaaS"
	// +optional
	Type *ScyllaDBMonitoringType `json:"type,omitempty"`
}

// ScyllaDBMonitoringStatus defines the observed state of ScyllaDBMonitoring.
type ScyllaDBMonitoringStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBMonitoring. It corresponds to the
	// ScyllaDBMonitoring's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBMonitoring state.
	// To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// TODO: Add structured info about replicas for each components.
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDBMonitoring defines a monitoring instance for ScyllaDB clusters.
type ScyllaDBMonitoring struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this ScyllaDBMonitoring.
	Spec ScyllaDBMonitoringSpec `json:"spec,omitempty"`

	// status is the current status of this ScyllaDBMonitoring.
	Status ScyllaDBMonitoringStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDBMonitoringList holds a list of ScyllaDBMonitoring.
type ScyllaDBMonitoringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBMonitoring `json:"items"`
}
