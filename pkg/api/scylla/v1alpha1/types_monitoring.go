package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.
	// +optional
	VolumeClaimTemplate corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
}

// PrometheusMode describes the mode of the Prometheus instance.
// +kubebuilder:validation:Enum="Managed";"External"
type PrometheusMode string

const (
	// PrometheusModeManaged defines a mode where a `Prometheus` object is created as a child of a `ScyllaDBMonitoring`
	// object. `ServiceMonitor` and `PrometheusRule` resources are also created to configure scraping and alerting.
	// This mode requires a Prometheus Operator to be installed in the cluster.
	//
	// Deprecated: This mode is deprecated and will be removed in future versions. Use `External` mode instead.
	//
	PrometheusModeManaged PrometheusMode = "Managed"

	// PrometheusModeExternal defines a mode where no `Prometheus` child object is created, but `ServiceMonitor` and
	// `PrometheusRule` objects are still created to configure scraping and alerting.
	// This mode requires a Prometheus Operator to be installed in the cluster, along with a `Prometheus` instance
	// configured to reconcile `ServiceMonitor` and `PrometheusRule` resources.
	PrometheusModeExternal PrometheusMode = "External"
)

// PrometheusSpec holds the spec prometheus options.
type PrometheusSpec struct {
	// mode defines the mode of the Prometheus instance.
	// +kubebuilder:default:="Managed"
	// +optional
	Mode PrometheusMode `json:"mode,omitempty"`

	// placement describes restrictions for the nodes Prometheus is scheduled on.
	// +optional
	Placement *PlacementSpec `json:"placement,omitempty"`

	// resources the Prometheus container will use.
	Resources corev1.ResourceRequirements `json:"resources"`

	// exposeOptions specifies options for exposing Prometheus UI.
	//
	// Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future
	// versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Prometheus if needed.
	//
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
	//
	// Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future
	// versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Grafana if needed.
	//
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

	// datasources is a list of Grafana datasources to configure.
	// It's expected to be set when using Prometheus component in `External` mode.
	// At most one datasource is allowed for now (only Prometheus is supported).
	// +kubebuilder:validation:MaxItems=1
	// +optional
	Datasources []GrafanaDatasourceSpec `json:"datasources,omitempty"`
}

// GrafanaDatasourceType defines the type of Grafana datasource.
// +kubebuilder:validation:Enum="Prometheus"
type GrafanaDatasourceType string

const (
	// GrafanaDatasourceTypePrometheus is the Prometheus datasource type.
	GrafanaDatasourceTypePrometheus GrafanaDatasourceType = "Prometheus"
)

type GrafanaDatasourceSpec struct {
	// name is the name of the datasource as it will appear in Grafana.
	// Only "prometheus" is supported as that's the datasource name expected by the ScyllaDB monitoring stack dashboards.
	// +kubebuilder:validation:Enum="prometheus"
	// +kubebuilder:default:="prometheus"
	Name string `json:"name,omitempty"`

	// type is the type of the datasource. Only "prometheus" is supported.
	// +kubebuilder:validation:Enum="Prometheus"
	// +kubebuilder:default:="Prometheus"
	// +optional
	Type GrafanaDatasourceType `json:"type,omitempty"`

	// url is the URL of the datasource.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// prometheusOptions defines Prometheus-specific options.
	// +optional
	PrometheusOptions *GrafanaPrometheusDatasourceOptions `json:"prometheusOptions,omitempty"`
}

type GrafanaPrometheusDatasourceOptions struct {
	// tls holds TLS configuration for connecting to Prometheus over HTTPS.
	// +optional
	TLS *GrafanaDatasourceTLSSpec `json:"tls,omitempty"`

	// auth holds authentication options for connecting to Prometheus.
	// +optional
	Auth *GrafanaPrometheusDatasourceAuthSpec `json:"auth,omitempty"`
}

// GrafanaPrometheusDatasourceAuthType defines the type of authentication to use when connecting to Prometheus.
type GrafanaPrometheusDatasourceAuthType string

const (
	// GrafanaPrometheusDatasourceAuthTypeNoAuthentication means no authentication.
	GrafanaPrometheusDatasourceAuthTypeNoAuthentication GrafanaPrometheusDatasourceAuthType = "NoAuthentication"

	// GrafanaPrometheusDatasourceAuthTypeBearerToken means Bearer token authentication.
	GrafanaPrometheusDatasourceAuthTypeBearerToken GrafanaPrometheusDatasourceAuthType = "BearerToken"
)

type GrafanaPrometheusDatasourceAuthSpec struct {
	// type is the type of authentication to use.
	// +kubebuilder:default:="NoAuthentication"
	// +optional
	Type GrafanaPrometheusDatasourceAuthType `json:"type,omitempty"`

	// bearerToken holds options for Bearer token authentication.
	// +optional
	BearerTokenOptions *GrafanaPrometheusDatasourceBearerTokenAuthOptions `json:"bearerTokenOptions,omitempty"`
}

type GrafanaPrometheusDatasourceBearerTokenAuthOptions struct {
	// secretRef is a reference to a key in a Secret holding a Bearer token to use to authenticate with Prometheus.
	// +optional
	SecretRef *LocalObjectKeySelector `json:"secretRef,omitempty"`
}

type GrafanaDatasourceTLSSpec struct {
	// caCert is a reference to a key within the CA bundle ConfigMap. The key should hold the CA cert in PEM format.
	// When not specified, system CAs are used.
	// +optional
	CACertConfigMapRef *LocalObjectKeySelector `json:"caCertConfigMapRef,omitempty"`

	// insecureSkipVerify controls whether to skip server certificate verification.
	// +kubebuilder:default:=false
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// clientTLSKeyPairSecretRef is a reference to a Secret holding client TLS certificate and key for mTLS authentication.
	// It's expected to be a standard Kubernetes TLS Secret with `tls.crt` and `tls.key` keys.
	// +optional
	ClientTLSKeyPairSecretRef *LocalObjectReference `json:"clientTLSKeyPairSecretRef,omitempty"`
}

// LocalObjectKeySelector selects a key of a ConfigMap or Secret in the same namespace.
type LocalObjectKeySelector struct {
	// name of the selected object.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// key within the selected object.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// LocalObjectReference contains a reference to an object in the same namespace.
// It can be used to reference a Secret, ConfigMap, or any other namespaced resource.
type LocalObjectReference struct {
	// Name of the referent.
	Name string `json:"name"`
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

func (smc *ScyllaDBMonitoringSpec) GetType() ScyllaDBMonitoringType {
	if smc.Type == nil {
		return ScyllaDBMonitoringTypeSAAS
	}

	return *smc.Type
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
// +kubebuilder:printcolumn:name="AVAILABLE",type=string,JSONPath=".status.conditions[?(@.type=='Available')].status"
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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
