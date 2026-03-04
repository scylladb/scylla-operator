# Monitoring

This page explains how ScyllaDB Operator integrates with Prometheus and Grafana to provide a monitoring stack for ScyllaDB clusters.

## ScyllaDBMonitoring resource

The `ScyllaDBMonitoring` custom resource (v1alpha1) describes a monitoring stack for one or more ScyllaDB clusters. When you create a `ScyllaDBMonitoring`, the Operator provisions:

- **Prometheus** scraping and alerting configuration (and optionally a Prometheus instance).
- **Grafana** with pre-built ScyllaDB dashboards.

:::{note}
`ScyllaDBMonitoring` is a v1alpha1 resource but is considered stable for production use. The `exposeOptions` fields for Prometheus and Grafana are deprecated and will be removed in the next API version.
:::

## Dependency on Prometheus Operator

ScyllaDB Operator does **not** include its own metrics collection engine. It relies on [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) to translate `ServiceMonitor` and `PrometheusRule` resources into Prometheus scraping and alerting configuration.

Prometheus Operator must be installed in the cluster before you create a `ScyllaDBMonitoring` resource. See the [dependency chain](overview.md) in the architecture overview.

## Prometheus modes

The `spec.components.prometheus.mode` field selects how Prometheus is deployed:

### External mode

The Operator creates `ServiceMonitor` and `PrometheusRule` resources but does **not** deploy a Prometheus instance. You bring your own Prometheus (managed by Prometheus Operator) and ensure it is configured to discover the ServiceMonitor and PrometheusRule resources created by the Operator.

This is the recommended mode. It lets you consolidate monitoring across multiple ScyllaDB clusters and other workloads into a single Prometheus deployment.

When using External mode, configure `spec.components.grafana.datasources` so that Grafana knows how to reach your Prometheus instance (URL, TLS, authentication).

### Managed mode (deprecated)

The Operator deploys a dedicated `Prometheus` custom resource (from Prometheus Operator) alongside the ServiceMonitor and PrometheusRule resources. This gives each `ScyllaDBMonitoring` its own isolated Prometheus instance.

:::{caution}
Managed mode is deprecated and will be removed in a future version. Deploy your own Prometheus and use External mode instead.
:::

## How cluster selection works

`ScyllaDBMonitoring` does not reference a ScyllaDB cluster by name. Instead, it uses a **label selector**:

```yaml
spec:
  endpointsSelector:
    matchLabels:
      scylla/cluster: my-cluster
```

The `endpointsSelector` is passed into the `ServiceMonitor` as `spec.selector`. Prometheus discovers all member Services whose labels match the selector and scrapes them.

This design lets a single `ScyllaDBMonitoring` cover multiple clusters (by using a broader selector) or restrict monitoring to a single cluster.

## Metrics endpoints scraped

The `ServiceMonitor` created by the Operator scrapes two endpoints on each matched ScyllaDB member Service:

| Endpoint | Port | Port number | Metrics source |
|----------|------|-------------|----------------|
| ScyllaDB Prometheus | `prometheus` | 9180 | ScyllaDB's built-in Prometheus endpoint. Exports CQL, storage, compaction, and other database metrics. |
| Node Exporter | `node-exporter` | 9100 | OS-level metrics (CPU, memory, disk, network) from the node exporter sidecar. |

Metric relabeling rules extract `cluster`, `dc` (datacenter), and ScyllaDB version information from Kubernetes labels and attach them to every metric.

## Alerting rules

The Operator creates `PrometheusRule` resources that define alerting and recording rules for ScyllaDB:

| PrometheusRule | Purpose |
|----------------|---------|
| Latency rules | Recording rules that precompute latency percentiles for dashboard queries. |
| Alert rules | Alerting rules for common ScyllaDB failure conditions. |
| Table rules | Recording rules that aggregate per-table metrics. |

These rules are labeled with the `ScyllaDBMonitoring` name so that in Managed mode the dedicated Prometheus instance picks them up automatically. In External mode, your Prometheus must be configured to select PrometheusRules with matching labels.

## Grafana

The Operator deploys Grafana as a `Deployment` and preconfigures it with:

- **Datasource provisioning** — points Grafana at the Prometheus instance (Managed mode uses the co-deployed instance; External mode uses the URL from `spec.components.grafana.datasources`).
- **Dashboard provisioning** — injects ScyllaDB dashboards from the [scylla-monitoring](https://github.com/scylladb/scylla-monitoring/) project as gzip-compressed ConfigMaps.
- **Admin credentials** — an auto-generated admin password stored in a Secret.

A `ClusterIP` Service named `<name>-grafana` is created for each `ScyllaDBMonitoring`. To access Grafana from outside the cluster, use `kubectl port-forward` for temporary access or create an Ingress / Gateway API resource for production access.

### Monitoring type and dashboards

The `spec.type` field selects which set of dashboards is provisioned:

| Type | Dashboards included |
|------|---------------------|
| **`SaaS`** (default) | A single ScyllaDB overview dashboard. Suitable when you do not control the underlying infrastructure. |
| **`Platform`** | Full set of dashboards: overview, CQL, detailed, advanced, OS, keyspace, and Alternator. Use when you manage the Kubernetes nodes and want infrastructure-level visibility. Also includes a ScyllaDB Manager dashboard. |

Dashboards are versioned per ScyllaDB release. The Operator bundles dashboards for multiple ScyllaDB versions and selects the appropriate set based on your cluster's version.

## Resources created — summary

| Resource | Managed mode | External mode |
|----------|-------------|---------------|
| `ServiceMonitor` | ✓ | ✓ |
| `PrometheusRule` (latency) | ✓ | ✓ |
| `PrometheusRule` (alerts) | ✓ | ✓ |
| `PrometheusRule` (table) | ✓ | ✓ |
| `Prometheus` (Prometheus Operator CR) | ✓ | — |
| Prometheus ServiceAccount, RoleBinding, Service | ✓ | — |
| Grafana Deployment, Service, ConfigMaps, Secret | ✓ | ✓ |

## Operator-level configuration

The Prometheus and Grafana versions used by the Operator are pinned in the Operator's configuration:

| Setting | Example value |
|---------|---------------|
| `operator.prometheusVersion` | `v3.6.0` |
| `operator.grafanaImage` | `docker.io/grafana/grafana:12.2.0` |

These values are stored in `assets/config/config.yaml` and propagated through `ScyllaOperatorConfig` status.

## Related pages

- [Overview](overview.md) — dependency chain showing where Prometheus Operator fits.
- [Security](security.md) — TLS between Grafana and Prometheus (mTLS in Managed mode).
