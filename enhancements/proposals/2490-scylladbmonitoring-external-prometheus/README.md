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
- Keep creating scraping configuration for an external Prometheus instance via `ServiceMonitor` and `PrometheusRule`
  resources.
- Maintain backwards compatibility: default behavior remains unchanged (managed Prometheus enabled, Grafana pointing at
  it).

### Non-Goals

- Support Prometheus instances installed other way than by the Prometheus Operator (to keep integration with Prometheus
  via `ServiceMonitor` and `PrometheusRule` resources).
- Add support for non-Prometheus Grafana data sources.
- Support for disabling the managed Grafana instance.

## Proposal

I propose to extend the `v1alpha1.ScyllaDBMonitoring` CRD with new fields that will allow users to disable the managed
Prometheus instance and configure the managed Grafana instance to use an external Prometheus instance as a data source.

Grafana data source provisioning configuration will be provided inline in the CR spec. It will be expected to contain a
YAML configuration compliant with
the [Grafana provisioning API](https://grafana.com/docs/grafana/latest/administration/provisioning/#datasources).

To support TLS/bearer auth to external Prometheus, users can mount Secrets/ConfigMaps into Grafana and reference them
from provisioning data source config via `$__file{}` directive.

### User Stories

#### Story 1

As a cluster admin on OpenShift, I want to monitor Scylla Operator-managed ScyllaDB cluster, and I already have a
Prometheus instance running in my cluster. I want the Operator to configure its managed Grafana instance to use my
existing Prometheus instance as a data source.

### Notes

- We assume that the "external Prometheus" means a Prometheus instance that is managed by
  a [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator). This is because the ScyllaDB
  Operator uses `ServiceMonitor` and `PrometheusRule` resources to define scraping and alerting rules, which are
  reconciled by the Prometheus Operator.

### Risks and Mitigations

1. **Risk**: Allowing arbitrary `Volume`s and `VolumeMount`s in the Grafana pod may lead to unexpected behavior if users
   will mount volumes under the same paths as Grafana uses for its configuration/provisioning, etc.
   **Mitigation**: We should document that users should not mount volumes under Grafana's configuration paths. We should
   also validate that the mounted volumes do not conflict with
   Grafana's configuration paths in the admission webhook.

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
   modes.
   Implementation of switching modes would require additional work to ensure the managed resources are
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
        PrometheusDatasource *apiextensiosnv1.JSON `json:"prometheusDatasource,omitempty"`

        // volumes is a list of volumes to be added to the Grafana pod.
        // This is useful for providing additional configuration files or data to Grafana.
        Volumes []corev1.Volume `json:"volumes,omitempty"`

        // volumeMounts is a list of volume mounts to be added to the Grafana container.
        // This is useful for mounting additional configuration files or data into Grafana.
        VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
    ```

### Example

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

### Validation

#### Admission Webhook

The Operator will validate in its admission webhook the `ScyllaDBMonitoring` objects to comply with the following rules:

- If `spec.components.prometheus.mode` is set to `External`, `spec.components.grafana.prometheusDatasource` must be
  specified.
- Immutability of `spec.components.prometheus.mode` field.
- Prevent duplicate volume names in `spec.components.grafana.volumes`.
- Do not allow mounting volumes under Grafana's configuration paths.

### Test Plan

- Validation rules will be covered in unit tests.
- Functions generating `ScyllaDBMonitoring` child resources will be covered in unit tests.
- A regular E2E test will be added to verify that it's possible to use `ScyllaDBMonitoring` with `External` Prometheus
  mode
  and configure the managed Grafana instance to use an external Prometheus instance as a data source.
- An OpenShift E2E test will be added that will verify that the `ScyllaDBMonitoring` CRD can be used to integrate
  with a Prometheus instance managed by OpenShift's user-workload monitoring stack.

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

- Add a guide that will walk users through the process of configuring the `ScyllaDBMonitoring` CRD to disable the
  managed Prometheus instance and configure the managed Grafana instance to use an external Prometheus instance as a
  data source on the OpenShift platform.
- Update the `ScyllaDBMonitoring` CRD documentation to reflect the new fields and their usage.

## Implementation History

- 2025-08-21: Enhancement proposal introduced.

## Alternatives

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

### Richer Grafana data source API

This option involves extend the `ScyllaDBMonitoring` CRD with a richer strongly-typed API for configuring Grafana data
sources (similar to the `GrafanaDatasource` CRD from
the [Grafana Operator](https://grafana.github.io/grafana-operator/docs/datasources/)). I decided to not pursue this
option because it would require a significant amount of work to implement and maintain the API we have no control over (
schema is defined by Grafana).
