# ScyllaDB Monitoring overview

## Architecture

ScyllaDB [exposes](https://monitoring.docs.scylladb.com/stable/reference/monitoring-apis.html) its metrics in Prometheus format. 
{{productName}} leverages this capability in Kubernetes environments to provide a comprehensive monitoring solution for your ScyllaDB clusters.
{{productName}} supports deploying and managing a complete monitoring stack for your ScyllaDB clusters using the
[ScyllaDBMonitoring](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource.

:::{note}
ScyllaDBMonitoring CRD is still experimental. The API is currently in version `v1alpha1` and may change in future versions.
:::

The monitoring stack that can be deployed with ScyllaDBMonitoring includes the following components:
- Prometheus for metrics collection and alerting (along with scraping and alerting rules targeting ScyllaDB instances).
- Grafana for metrics visualization (with pre-configured dashboards for ScyllaDB).

:::{include} diagrams/monitoring-overview.mmd
:::

The monitoring stack is expected to be created separately for each ScyllaDB cluster, allowing you to have isolated monitoring environments.

## Third-party dependencies

{{productName}} builds on top of the stable and widely adopted open-source projects to provide a robust monitoring solution.

### Prometheus Operator

For deploying and/or configuring Prometheus, {{productName}} relies on the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator).
Depending on configuration, {{productName}} may deploy and manage a Prometheus instance for you, or it can be configured to use
an existing Prometheus instance (managed by the Prometheus Operator) in your cluster. 

The following Prometheus Operator resources are created by the {{productName}} when you deploy a ScyllaDBMonitoring resource:

- [Prometheus](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/prometheus_types.go#L1085) - the Prometheus instance itself.
- [ServiceMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/servicemonitor_types.go#L41) - the resource that defines how to scrape metrics from ScyllaDB nodes.
- [PrometheusRule](https://github.com/prometheus-operator/prometheus-operator/blob/e4c727291acc543dab531bc4aaf16637067c1b86/pkg/apis/monitoring/v1/prometheusrule_types.go#L37) - the resource that defines alerting rules for Prometheus.
  
Prometheus version used in the deployment is tied to the version of {{productName}}. You can find the exact version used in the
[config.yaml](https://github.com/scylladb/scylla-operator/blob/master/assets/config/config.yaml) file under `operator.prometheusVersion` key.

### Grafana

For deploying Grafana, {{productName}} doesn't use any third-party operator. Instead, it manages the Grafana deployment
directly. It makes sure that Grafana is pre-configured with dashboards from [scylla-monitoring](https://github.com/scylladb/scylla-monitoring/).

Grafana image used in the deployment is tied to the version of {{productName}}. You can find the exact image used in the
[config.yaml](https://github.com/scylladb/scylla-operator/blob/master/assets/config/config.yaml) file under `operator.grafanaImage` key.

### Ingress Controller

An Ingress Controller can be used to expose the Grafana outside the Kubernetes cluster.
{{productName}} does not manage the Ingress Controller itself, but it creates an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) resource to expose Grafana
according to the configuration in the ScyllaDBMonitoring resource. You can use any Ingress Controller of your choice.

You can learn more about exposing Grafana in the [ScyllaDB Monitoring setup](exposing-grafana.md) guide.
