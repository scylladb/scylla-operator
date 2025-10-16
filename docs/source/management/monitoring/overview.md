# ScyllaDB Monitoring overview

## Architecture

ScyllaDB [exposes](https://monitoring.docs.scylladb.com/stable/reference/monitoring-apis.html) its metrics in Prometheus format. 
{{productName}} provides [ScyllaDBMonitoring](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource
that allows you to set up a complete monitoring stack for your ScyllaDB clusters based on the following components:

- Prometheus for metrics collection and alerting (along with scraping and alerting rules targeting ScyllaDB instances).
- Grafana for metrics visualization (with pre-configured dashboards for ScyllaDB).

:::{include} diagrams/monitoring-overview.mmd
:::

:::{note}
ScyllaDBMonitoring CRD is still in `v1alpha1` version, yet it is considered stable and ready for production use, with
the following caveats:
- `spec.components.grafana.exposeOptions` and `spec.components.prometheus.exposeOptions` are deprecated and will be removed in the next API version.
- **External** mode for Prometheus is likely to be the only supported mode in the next API version.
:::

## Prometheus

For deploying and/or configuring Prometheus, {{productName}} relies on the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator).
Depending on configuration, {{productName}} may deploy and manage a Prometheus instance for you, or it can be configured to use
an existing Prometheus instance (managed by the Prometheus Operator) in your cluster.

The following Prometheus Operator resources are created by {{productName}} when you deploy a ScyllaDBMonitoring resource:

- [Prometheus](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/prometheus_types.go#L1085) - the Prometheus instance itself (it may be omitted in External mode).
- [ServiceMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/servicemonitor_types.go#L41) - the resource that defines how to scrape metrics from ScyllaDB nodes.
- [PrometheusRule](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/prometheusrule_types.go#L37) - the resource that defines alerting rules for Prometheus.

Prometheus version used in the deployment is tied to the version of {{productName}}. You can find the exact version used in the
[config.yaml](https://github.com/scylladb/scylla-operator/blob/master/assets/config/config.yaml) file under `operator.prometheusVersion` key.

As of now, {{productName}} supports two modes of operation for ScyllaDBMonitoring regarding Prometheus deployment:
**Managed** and **External**. You can choose the mode that best fits your needs by setting the `spec.components.prometheus.mode` field in the ScyllaDBMonitoring resource.

### Managed

In the **Managed** mode, {{productName}} will deploy a Prometheus instance for you. This is the default mode.
What this means is that when you create a ScyllaDBMonitoring resource, {{productName}} will create a Prometheus custom 
resource (from the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)) in the same namespace as the ScyllaDBMonitoring resource. 
This Prometheus instance will be configured to scrape metrics from the ScyllaDB nodes in the cluster that ScyllaDBMonitoring is monitoring and
will also have alerting rules configured for ScyllaDB (using ServiceMonitor and PrometheusRule CRs).
This mode is suitable for most use cases, especially if you don't have an existing Prometheus instance in your cluster.

### External

In the **External** mode, {{productName}} will not deploy a Prometheus CR instance, but it will still create the ServiceMonitor and PrometheusRule resources
that are to be reconciled by an existing Prometheus Operator instance in your cluster. This mode is useful if you already have a Prometheus instance
deployed in your cluster, and you want to use it for monitoring your ScyllaDB clusters.

When using this mode, you need to ensure that the existing Prometheus instance is configured to discover and scrape the 
ServiceMonitor and PrometheusRule resources created by {{productName}}. This requires setting the appropriate
`serviceMonitorSelector` and `ruleSelector` in the Prometheus resource to match the labels used by {{productName}} (or
leaving them empty to select all resources).

Please note that in this mode, ScyllaDBMonitoring has to be configured so that Grafana can access the Prometheus instance.
You can configure Grafana datasources in the `spec.components.grafana.datasources` field of the ScyllaDBMonitoring resource.
Please refer to the [ScyllaDBMonitoring API reference](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) for details.

## Grafana

For deploying Grafana, {{productName}} doesn't use any third-party operator. Instead, it manages the Grafana deployment
directly. It preconfigures Grafana with dashboards from [scylla-monitoring](https://github.com/scylladb/scylla-monitoring/).

The Grafana image used in the deployment is tied to the version of {{productName}}. You can find the exact image used in the
[config.yaml](https://github.com/scylladb/scylla-operator/blob/master/assets/config/config.yaml) file under `operator.grafanaImage` key.

### Exposing Grafana

{{productName}} creates a `<scyllaDBMonitoringName>-grafana` `ClusterIP` Service for each ScyllaDBMonitoring.
You can access it outside the cluster using your preferred method:

- Port forwarding using `kubectl port-forward` command for temporary access.
- Using [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) or [Gateway API](https://gateway-api.sigs.k8s.io/)
  resources (e.g., HTTPRoute) for production access.

You can learn more about exposing Grafana in the [Exposing Grafana](exposing-grafana.md) guide.
