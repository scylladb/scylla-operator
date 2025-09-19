# Extending ScyllaDBMonitoring with external Prometheus support

## Summary

This proposal aims to extend the `ScyllaDBMonitoring` CRD to support external Prometheus instances. Currently, there's
no possibility to configure the monitoring stack using this CRD
other way than letting it create managed Prometheus and Grafana instances. The goal is to allow users to configure
`ScyllaDBMonitoring` to _not_ deploy managed Prometheus, and to allow configuring managed Grafana instance with an
external Prometheus as a data source.

## Motivation

It is common for users to already operate a Prometheus instance (e.g.,
OpenShift [user-workload monitoring](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/monitoring/configuring-user-workload-monitoring),
a global corporate Prometheus stack, etc.). Running a second Prometheus solely for ScyllaDB introduces unnecessary
duplication, resource overhead, and management
complexity.

By providing knobs to skip deploying Prometheus and to configure Grafana with an external Prometheus as a data source,
we give users the flexibility to integrate with their existing monitoring stack.

### Goals

- Allow users to disable the managed Prometheus component.
- Allow users to configure the managed Grafana instance to use an external Prometheus instance as a data source.
- Maintain backwards compatibility: default behavior remains unchanged (managed Prometheus enabled, Grafana pointing at
  it).
- Allow secure connection to the external Prometheus instance using TLS (CA cert, client cert/key) and bearer token
  authentication.

### Non-Goals

- Support Prometheus instances installed other way than by the Prometheus Operator (to keep integration with Prometheus
  via `ServiceMonitor` and `PrometheusRule` resources).
- Add support for non-Prometheus Grafana data sources.
- Support for disabling the managed Grafana instance.
- Support all possible authentication methods.

## Proposal

I propose to extend the `v1alpha1.ScyllaDBMonitoring` CRD with new fields that will allow users to disable the managed
Prometheus instance and configure the managed Grafana instance to use an external Prometheus instance as a data source.

The operator will keep creating scraping configuration for an external Prometheus instance via `ServiceMonitor` and
`PrometheusRule`
resources as it does now.

Grafana data source provisioning configuration will be provided inline in the CR spec, using a strongly-typed API.
It will support only a single Prometheus data source, as ScyllaDB monitoring stack dashboards are expecting the
Prometheus data source to be named `prometheus`.

It will allow configuring connection to the external Prometheus instance using TLS (CA cert, client cert/key) and bearer
token auth.

### User Stories

#### Story 1

As a cluster admin on OpenShift, I want to monitor Scylla Operator-managed ScyllaDB cluster, and I already have a
Prometheus instance running in my cluster. I want the Operator to configure its managed Grafana instance to use my
existing Prometheus instance as a data source.

#### Story 2

As a cluster admin on Kubernetes, I want to monitor Scylla Operator-managed ScyllaDB cluster, and I already have a
Prometheus instance running in my cluster (deployed with Prometheus Operator). I want the Operator to configure its
managed Grafana instance to use my existing Prometheus instance as a data source.

### Notes

- We assume that the "external Prometheus" means a Prometheus instance that is managed by
  a [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator). This is because the ScyllaDB
  Operator uses `ServiceMonitor` and `PrometheusRule` resources to define scraping and alerting rules, which are
  reconciled by the Prometheus Operator.

### Risks and Mitigations

1. Strongly-typed Grafana datasource specification will not include all possible fields supported by Grafana.

   We consciously decided to use a strongly-typed API to make sure all supported configuration options are validated
   and tested on our side. If an extension is needed in the future, we can add more fields as necessary based on user
   feedback.

## Design Details

### Proposed API changes

`ScyllaDBMonitoring` CRD will be extended with the following changes:

1. Add a new `spec.components.prometheus.mode` field to allow users to choose the mode of the Prometheus component.
   The field will be an enum, defaulting to `Managed` (i.e., Operator-managed Prometheus is enabled by default).
   When set to `External`, the Operator will not deploy a managed Prometheus instance. Only the scraping and alerting
   resources will be created (`ServiceMonitor`s and `PrometheusRule`s) to allow the external Prometheus instance to
   scrape metrics and alerts from the ScyllaDB cluster.

    ```go
    // PrometheusMode describes the mode of the Prometheus instance.
    // +kubebuilder:validation:Enum="Managed";"External"
    type PrometheusMode string

    const (
        // PrometheusModeManaged defines a mode where a `Prometheus` object is created as a child of a `ScyllaDBMonitoring`
        // object. `ServiceMonitor` and `PrometheusRule` resources are also created to configure scraping and alerting.
		// This mode requires a Prometheus Operator to be installed in the cluster.
        PrometheusModeManaged PrometheusMode = "Managed"
   
        // PrometheusModeExternal defines a mode where no `Prometheus` child object is created, but `ServiceMonitor` and
        // `PrometheusRule` objects are still created to configure scraping and alerting.
        // This mode requires a Prometheus Operator to be installed in the cluster, along with a `Prometheus` instance
        // configured to reconcile `ServiceMonitor` and `PrometheusRule` resources.
        PrometheusModeExternal PrometheusMode = "External"
    )
    
    type PrometheusSpec struct {
        // mode defines the mode of the Prometheus instance. It's an immutable field.
        // +kubebuilder:default:="Managed"
        // +optional
        Mode PrometheusMode `json:"mode,omitempty"`
        ...
    }
    ```

   Externally managed Prometheus instances can be configured to reconcile `ServiceMonitor` and `PrometheusRule`
   resources by setting `ruleSelector` and `serviceMonitorSelector` fields in
   the [Prometheus](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.Prometheus) spec.
   We will document that these fields have to match the
   `scylla-operator.scylladb.com/scylladbmonitoring-name: "{{ .scyllaDBMonitoringName }}"`
   label in order to work, as that's the label the Operator adds to the `ServiceMonitor` and `PrometheusRule` resources
   it creates.

   The added `mode` field will be immutable to prevent users from switching between managed and external Prometheus
   modes. Users will be asked to delete and recreate the `ScyllaDBMonitoring` resource if they want to switch modes.

2. Add new `datasources` field to `spec.components.grafana` to allow users to configure the managed Grafana instance to
   use an external Prometheus instance as a data source.

   Because ScyllaDB monitoring stack dashboards are expecting the Prometheus data source to be named `prometheus`, we
   will support only 1 data source in this field for now, but we will make it a list to allow adding more data sources
   in the future if needed.

   ```go
   type GrafanaSpec struct {
        ...
       // datasources is a list of Grafana datasources to configure.
       // It's expected to be set when using Prometheus component in `External` mode.
       // At most one datasource is allowed for now (only Prometheus is supported).
       // +kubebuilder:validation:MaxItems=1
       // +optional
       Datasources []GrafanaDatasourceSpec `json:"datasources,omitempty"`
   }
   ```

   We will set the datasource name to `prometheus` by default and the type to `Prometheus`, so users
   don't have to specify them explicitly.

   To securely provide sensitive information, all sensitive fields in the Grafana data source configuration (like tokens
   or TLS certificates) will be expected to be provided via `Secret` references.

   ```go
   type GrafanaDatasourceSpec struct {
       // name is the name of the datasource as it will appear in Grafana.
       // Only "prometheus" is supported for now as that's the datasource name expected by the ScyllaDB monitoring stack dashboards.
       // +kubebuilder:validation:Enum="prometheus"
       // +kubebuilder:default:="prometheus"
       Name string `json:"name,omitempty"`

       // type is the type of the datasource. Only "prometheus" is supported.
       // +kubebuilder:default:="Prometheus"
       // +optional
       Type GrafanaDatasourceType `json:"type,omitempty"`

       // url is the URL of the datasource.
       // +kubebuilder:validation:MinLength=1
       URL string `json:"url"`

       // prometheus defines Prometheus-specific options.
       // +optional
       PrometheusOptions *GrafanaPrometheusDatasourceOptions `json:"prometheusOptions,omitempty"`
   }
   ```

   Strongly-typed enum will be used for the datasource type:

   ```go
   // GrafanaDatasourceType defines the type of Grafana datasource.
   // +kubebuilder:validation:Enum="Prometheus"
   type GrafanaDatasourceType string

   const (
       // GrafanaDatasourceTypePrometheus is the Prometheus datasource type.
       GrafanaDatasourceTypePrometheus GrafanaDatasourceType = "Prometheus"
   )
   ```

   `GrafanaPrometheusDatasourceSpec` will define Prometheus-specific options, including TLS and authentication.

   ```go
   type GrafanaPrometheusDatasourceOptions struct {
      // tls holds TLS configuration for connecting to Prometheus over HTTPS.
      // +optional
      TLS *GrafanaDatasourceTLSSpec `json:"tls,omitempty"`

      // auth holds authentication options for connecting to Prometheus.
      // +optional
      Auth *GrafanaPrometheusDatasourceAuthSpec `json:"auth,omitempty"`
   }
   ```

   `GrafanaDatasourceTLSSpec` will define TLS configuration options for server certificate verification and client
   authentication.

   ```go
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
   ```

   `GrafanaPrometheusDatasourceAuthSpec` will define authentication options, starting with bearer token auth. It will
   be a separate struct to allow adding more auth methods in the future if needed (e.g., basic auth).

   ```go
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
   ```

On every change to the `datasources` field, the Grafana deployment will be rerolled to apply the new
configuration. It will be achieved by leveraging the existing `scylla-operator.scylladb.com/inputs-hash` annotation on 
the Grafana deployment's Pod template. Among other already used objects and fields, hash will be extended to take into account
content of the referenced `Secret`/`ConfigMap`s as well.

`Secret` and `ConfigMap` references in the data source configuration will be tracked, and changes to them will also
trigger a `ScyllaDBMonitoring` reconciliation. `ScyllaDBMonitoring` controller will watch `Secret` and `ConfigMap` resources, and
will trigger a reconciliation of `ScyllaDBMonitoring` resources that reference them in their Grafana data source
configuration. Whether a `Secret` or `ConfigMap` is referenced will be determined by querying an index created by the
controller. The index will be created by using `ScyllaDBMonitoring`'s informer's `cache.SharedIndexInformer.AddIndexers`
method. Queries to the index will be performed using `cache.Indexer.ByIndex` method.

### Example (verbose)

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBMonitoring
metadata:
  name: example
spec:
  type: Platform
  endpointsSelector:
    matchLabels:
      app.kubernetes.io/name: scylla
      scylla-operator.scylladb.com/scylla-service-type: member
      scylla/cluster: replace-with-your-scyllacluster-name
  components:
    prometheus:
      mode: External
    grafana:
      datasources:
        - name: prometheus
          url: "https://external-prometheus-address:9090"
          type: Prometheus
          prometheusOptions:
            auth:
              type: BearerToken
              bearerTokenOptions:
                secretRef:
                  name: prometheus-bearer-token
                  key: token
            tls:
              caCertConfigMapRef:
                name: prometheus-serving-ca
                key: ca.crt
              insecureSkipVerify: false
              clientTLSKeyPairSecretRef:
                name: prometheus-client-certs
```

### Handling error conditions

If the `ScyllaDBMonitoring` resource is misconfigured (e.g., missing required fields, invalid references to
`Secret`/`ConfigMap`s, etc.), the Operator will set proper conditions on the `ScyllaDBMonitoring` resource to inform the user
about the issue.

When the configured external Prometheus instance is not reachable or returns errors, the managed Grafana instance will
show errors when trying to use the Prometheus data source. The Operator will not try to validate the connectivity to the
external Prometheus instance, as it would introduce unnecessary complexity and potential issues (e.g., network policies,
firewalls, etc. preventing the Operator from reaching the Prometheus instance).

### Validation

#### Admission Webhook

The Operator will validate in its admission webhook the `ScyllaDBMonitoring` objects to comply with the following rules:

- If `spec.components.prometheus.mode` is set to `External`, `spec.components.grafana.datasources` must be set
- If `spec.components.grafana.datasources` is set, it must contain exactly one datasource of type `Prometheus`.
- Immutability of `spec.components.prometheus.mode` field.

To achieve this:

- The webhook server will be extended with a new validator for `ScyllaDBMonitoring` resources.
- `ValidatingWebhookConfiguration` definition will be extended to track `ScyllaDBMonitoring` creations and updates.
- `ClusterRole` used by the webhook server will be extended to allow `get` and `list` operations on
  `ScyllaDBMonitoring` resources.

### Test Plan

- Validation rules will be covered in unit tests.
- Functions generating `ScyllaDBMonitoring` child resources will be covered in unit tests.
- A regular E2E test will be added to verify that it's possible to use `ScyllaDBMonitoring` with `External`
  Prometheus mode and configure the managed Grafana instance to use an external Prometheus instance as a data source
  (with mTLS).
- An OpenShift E2E test will be added that will verify that the `ScyllaDBMonitoring` CRD can be used to integrate
  with a Prometheus instance managed by OpenShift's user-workload monitoring stack (with bearer token).
- A E2E test will be added to verify that create and update operations to `ScyllaDBMonitoring` resources trigger
  validating
  webhook and that invalid resources are rejected.

### Upgrade / Downgrade Strategy

No specific upgrade or downgrade strategy is required.

### Version Skew Strategy

To create `ServiceMonitor` and `PrometheusRule` resources, Operator uses
`github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring` Go module for their schema. In an OpenShift
cluster, mentioned CRDs are already installed (along with the Prometheus Operator) therefore we can only support
OpenShift clusters with the Prometheus Operator supporting the version of the `ServiceMonitor` and `PrometheusRule`
resources we use in the Operator. Their API is stable enough to not expect any breaking changes in the foreseeable
future, but it's worth noting that in our documentation (i.e., in
the [Support matrix](https://operator.docs.scylladb.com/stable/support/releases.html#support-matrix)).

### Documentation plan

- Add a guide that will walk users through the process of configuring the `ScyllaDBMonitoring` CRD. It should contain
  the "standard" example with Scylla Operator-managed Prometheus, and a detailed example with an externally managed
  Prometheus instance.
- Add a guide that will explain how to configure `ScyllaDBMonitoring` to work with OpenShift's user-workload
  monitoring stack (how to bind the right `ClusterRole` to the Grafana service account, how to get the bearer token, etc.).
- Update the `ScyllaDBMonitoring` CRD documentation to reflect the new fields and their usage.
- (Option) Write a blog post that will show how to use the externally managed Prometheus mode in a real-world
  scenario on OpenShift. Rationale for making it a blog post is that it will not fit well in our official documentation
  and could get outdated quickly due to OpenShift changes, but it could be useful for users.

## Implementation History

- 2025-09-12: Initial implementation completed.
- 2025-08-29: Proposal accepted.
- 2025-08-26: First review iteration.
- 2025-08-21: Enhancement proposal introduced.

## Alternatives

### Inline `apiextensionsv1.JSON` for datasource configuration

_Note: This was the initially proposed approach. It was rejected due to being too low level in terms of API. More
user-friendly
approach was desired._

Grafana data source provisioning configuration will be provided inline in the CR spec. It will be expected to contain a
YAML configuration compliant with
the [Grafana provisioning API](https://grafana.com/docs/grafana/latest/administration/provisioning/#datasources).

To support TLS/bearer auth to external Prometheus, users can mount Secrets/ConfigMaps into Grafana and reference them
from provisioning data source config via `$__file{}` directive.

`ScyllaDBMonitoring` CRD will be extended with the following changes:

1. Add a new `spec.components.prometheus.mode` field to allow users to choose the mode of the Prometheus component.
   The field will be an enum, defaulting to `Managed` (i.e., Operator-managed Prometheus is enabled by default).
   When set to `External`, the Operator will not deploy a managed Prometheus instance. Only the scraping and alerting
   resources will be created (`ServiceMonitor`s and `PrometheusRule`s) to allow the external Prometheus instance to
   scrape metrics and alerts from the ScyllaDB cluster.

    ```go
    // PrometheusMode describes the mode of the Prometheus instance.
    // +kubebuilder:validation:Enum="Managed";"External"
    type PrometheusMode string

    const (
        // PrometheusModeManaged defines a Prometheus instance that is managed by the operator.
        PrometheusModeManaged PrometheusMode = "Managed"
        // PrometheusModeExternal defines a Prometheus instance that is managed externally by a Prometheus Operator.
        PrometheusModeExternal PrometheusMode = "External"
    )

    type PrometheusSpec struct {
        // mode defines the mode of the Prometheus instance. It's an immutable field.
        // +kubebuilder:default:="Managed"
        // +optional
        Mode PrometheusMode `json:"mode,omitempty"`
        ...
    }
    ```

   Externally managed Prometheus instances can be configured to reconcile `ServiceMonitor` and `PrometheusRule`
   resources by setting `ruleSelector` and `serviceMonitorSelector` fields in
   the [Prometheus](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.Prometheus) spec.
   We will document that these fields have to match the
   `scylla-operator.scylladb.com/scylladbmonitoring-name: "{{ .scyllaDBMonitoringName }}"`
   label in order to work, as that's the label the Operator adds to the `ServiceMonitor` and `PrometheusRule` resources
   it creates.

   The added `mode` field will be immutable to prevent users from switching between managed and external Prometheus
   modes. Implementation of switching modes would require additional work to ensure the managed resources are
   properly cleaned up, and it's not worth the effort at this point.


2. Add new fields to `spec.components.grafana` to allow users to configure the managed Grafana instance to use an
   external Prometheus instance as a data source.

   `prometheusDatasource` field will be added to the `GrafanaSpec`. It will use an `apiextensionsv1.JSON` type, allowing
   users to
   specify the Grafana data source configuration inline. Thanks to allowing arbitrary fields, it's open to Grafana
   schema changes. Operator will create a `ConfigMap` automatically and mount it in the Grafana pod under the expected
   directory.

   Because ScyllaDB monitoring stack dashboards are expecting the Prometheus data source to be named `prometheus`, we
   will set the datasource name to `prometheus` by default and will ignore the name provided by the user in the
   `prometheusDatasource` field. We will also set the type to `prometheus` by default, so users don't have to specify
   it explicitly.

   To securely provide sensitive information (like tokens or TLS certificates) in the Grafana data source configuration,
   we need to allow users to mount volumes in the Grafana pod. This will be made possible by adding `Volumes` and
   `VolumeMounts` fields. Users will be able to refer to these volumes in the `prometheusDatasource` via Grafana's
   `$__file{}` directive.

   On every change to the `prometheusDatasource` field, the Grafana deployment will be rerolled to apply the new
   configuration.

    ```go
        // prometheusDatasource is the Prometheus data source configuration. It should be a valid Grafana data source 
        // configuration, compliant with the Grafana provisioning API. `name` and `type` fields will always be overridden
        // by the Operator to ensure that the Prometheus data source is named "prometheus" and is of type "prometheus".
        PrometheusDatasource *apiextensionsv1.JSON `json:"prometheusDatasource,omitempty"`

        // volumes is a list of volumes to be added to the Grafana pod.
        // This is useful for providing additional configuration files or data to Grafana.
        Volumes []corev1.Volume `json:"volumes,omitempty"`

        // volumeMounts is a list of volume mounts to be added to the Grafana container.
        // This is useful for mounting additional configuration files or data into Grafana.
        VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
    ```

#### Example

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBMonitoring
metadata:
  name: example
spec:
  type: Platform
  endpointsSelector:
    matchLabels:
      app.kubernetes.io/name: scylla
      scylla-operator.scylladb.com/scylla-service-type: member
      scylla/cluster: replace-with-your-scyllacluster-name
  components:
    prometheus:
      mode: External
    grafana:
      prometheusDatasource:
        # name: prometheus # This is the default name the Operator will set.
        # type: prometheus # This is the default type the Operator will set.
        access: proxy
        url: "https://external-prometheus-address:9090"
        isDefault: true
        version: 1
        editable: false
        jsonData:
          timeInterval: "5s"
          tlsAuthWithCACert: true
        secureJsonData:
          tlsCACert: "$__file{/var/run/configmaps/prometheus-serving-ca/ca-bundle.crt}"
          tlsClientCert: "$__file{/var/run/secrets/prometheus-client-certs/tls.crt}"
          tlsClientKey: "$__file{/var/run/secrets/prometheus-client-certs/tls.key}"
      volumes:
        - name: client-tls-certs
          secret:
            secretName: prometheus-client-certs
        - name: prometheus-serving-ca
          configMap:
            name: prometheus-serving-ca
      volumeMounts:
        - name: client-tls-certs
          mountPath: /var/run/secrets/prometheus-client-certs
          readOnly: true
        - name: prometheus-serving-ca
          mountPath: /var/run/configmaps/prometheus-serving-ca
      exposeOptions:
        webInterface:
          ingress:
            ingressClassName: haproxy
            dnsDomains:
              - example-grafana.test.svc.cluster.local
            annotations:
              haproxy-ingress.github.io/ssl-passthrough: "true"
```

### Using `ConfigMap` reference for data source API

Instead of using `apiextensionsv1.JSON` field to provide Grafana data sources configuration, we could use a `ConfigMap`
reference. This approach requires creating a separate `ConfigMap` that contains the Grafana data sources configuration
in YAML format. The `ScyllaDBMonitoring` CRD would then reference this `ConfigMap` in the
`spec.components.grafana.datasourcesConfigMapRef` field.
This approach is more verbose and requires managing an additional resource, but it allows users to keep the
data sources configuration in a separate file, which can be useful for larger configurations or when the configuration
needs to be shared across multiple `ScyllaDBMonitoring` instances.

#### Example

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBMonitoring
metadata:
  name: example
spec:
  type: Platform
  endpointsSelector:
    matchLabels:
      app.kubernetes.io/name: scylla
      scylla-operator.scylladb.com/scylla-service-type: member
      scylla/cluster: replace-with-your-scyllacluster-name
  components:
    prometheus:
      enabled: false
    grafana:
      datasourceConfigMapRef:
        name: example-grafana-datasources
      volumes:
        - name: client-tls-certs
          secret:
            secretName: prometheus-client-certs
        - name: prometheus-serving-ca
          configMap:
            name: prometheus-serving-ca
      volumeMounts:
        - name: client-tls-certs
          mountPath: /var/run/secrets/prometheus-client-certs
          readOnly: true
        - name: prometheus-serving-ca
          mountPath: /var/run/configmaps/prometheus-serving-ca
      exposeOptions:
        webInterface:
          ingress:
            ingressClassName: haproxy
            dnsDomains:
              - example-grafana.test.svc.cluster.local
            annotations:
              haproxy-ingress.github.io/ssl-passthrough: "true"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-grafana-datasources
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
    - name: prometheus
      type: prometheus
      access: proxy
      url: "https://external-prometheus-address:9090"
      isDefault: true
      version: 1
      editable: false
      jsonData:
        timeInterval: "5s"
        tlsAuthWithCACert: true
      secureJsonData:
        tlsCACert: "$__file{/var/run/configmaps/prometheus-serving-ca/ca-bundle.crt}"
        tlsClientCert: "$__file{/var/run/secrets/prometheus-client-certs/tls.crt}"
        tlsClientKey: "$__file{/var/run/secrets/prometheus-client-certs/tls.key}"
```
