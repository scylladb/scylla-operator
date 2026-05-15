# Overview

ScyllaDB Operator provides the `ScyllaDBMonitoring` custom resource to set up a complete monitoring stack for your ScyllaDB clusters, based on Prometheus for metrics collection and Grafana for visualization.

For details on the monitoring architecture, Prometheus modes (External and Managed), and how the components fit together, see [Monitoring](../../understand/monitoring.md).

## Guides

- [Set up ScyllaDB Monitoring](setup.md) — deploy Prometheus and configure `ScyllaDBMonitoring` for your cluster.
- [Set up ScyllaDB Monitoring on OpenShift](external-prometheus-on-openshift.md) — use OpenShift User Workload Monitoring as an external Prometheus source.
- [Expose Grafana](exposing-grafana.md) — make the Grafana dashboard accessible outside the cluster.
