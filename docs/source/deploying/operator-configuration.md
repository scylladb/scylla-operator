# Operator configuration

This page explains how to configure the ScyllaDB Operator's global settings using the `ScyllaOperatorConfig` resource.

## Overview

`ScyllaOperatorConfig` is a cluster-scoped singleton resource named `cluster`. The Operator creates it automatically on startup if it does not exist. It holds global settings that affect all ScyllaDB clusters managed by the Operator, such as auxiliary container images and the Kubernetes cluster domain.

Most users do not need to modify this resource. The Operator ships with sensible defaults that are updated automatically when you upgrade the Operator.

:::{tip}
You can inspect all available fields with:

```shell
kubectl explain --api-version='scylla.scylladb.com/v1alpha1' ScyllaOperatorConfig.spec
```
:::

## View the current configuration

```shell
kubectl get scyllaoperatorconfig cluster -o yaml
```

The `status` section shows the resolved values that are actually in use, including auto-discovered defaults:

```yaml
status:
  scyllaDBUtilsImage: docker.io/scylladb/scylla:2025.1.9@sha256:...
  bashToolsImage: registry.access.redhat.com/ubi9/ubi:9.5-...@sha256:...
  grafanaImage: docker.io/grafana/grafana:12.2.0@sha256:...
  prometheusVersion: v3.6.0
  clusterDomain: cluster.local
```

## Configurable fields

| Spec field | Description | Default |
|------------|-------------|---------|
| `scyllaUtilsImage` | ScyllaDB image used for running utility scripts (perftune, sysctl). Determines which tuning scripts are used for performance optimization. | Latest ScyllaDB Open Source image. |
| `configuredClusterDomain` | Kubernetes cluster domain. Must be a fully qualified domain name. | Auto-discovered via DNS lookup of `kubernetes.default.svc`. |
| `unsupportedBashToolsImageOverride` | Override the Bash tools image. **Unsupported** — for advanced use only. | UBI 9 image. |
| `unsupportedGrafanaImageOverride` | Override the Grafana image. **Unsupported** — for advanced use only. | Official Grafana image. |
| `unsupportedPrometheusVersionOverride` | Override the Prometheus version. **Unsupported** — for advanced use only. | Latest tested Prometheus version. |

:::{caution}
Fields prefixed with `unsupported` are not covered by the regular support policy. Use them only if you have a specific reason and understand the implications.
:::

## Tuning with ScyllaDB Enterprise

By default, the Operator uses performance tuning scripts from the ScyllaDB Open Source image. If you run ScyllaDB Enterprise, set `scyllaUtilsImage` to an Enterprise image to use its tuning scripts:

:::{code-block} yaml
:substitutions:
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaOperatorConfig
metadata:
  name: cluster
spec:
  scyllaUtilsImage: "{{enterpriseImageRepository}}:{{scyllaDBImageTag}}"
:::

Apply the change:

```shell
kubectl apply --server-side -f scyllaoperatorconfig.yaml
```

The NodeConfig DaemonSet picks up the new image and uses Enterprise-specific tuning scripts on the next reconciliation.

## Setting the cluster domain

The Operator auto-discovers the Kubernetes cluster domain by performing a DNS CNAME lookup for `kubernetes.default.svc`. If your cluster uses a non-standard domain or the auto-discovery does not work, set it explicitly:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaOperatorConfig
metadata:
  name: cluster
spec:
  configuredClusterDomain: my-cluster.local
```

The cluster domain is used to construct DNS names for ScyllaDB services and inter-node communication.

## How settings propagate

ScyllaOperatorConfig settings are consumed by several Operator controllers:

| Consumer | Setting used |
|----------|-------------|
| NodeConfig controller | `scyllaDBUtilsImage` — configures the tuning DaemonSet with the correct ScyllaDB image for `perftune.py` and resource limits. |
| ScyllaDBMonitoring controller | `grafanaImage`, `prometheusVersion` — configures the monitoring stack. |
| ScyllaDBCluster controller | `clusterDomain` — constructs DNS names for multi-datacenter communication. |

Changes to `ScyllaOperatorConfig` trigger reconciliation in all dependent controllers. You do not need to restart the Operator.

## Related pages

- [Node configuration](node-configuration.md) — performance tuning that uses the `scyllaUtilsImage`.
- [Monitoring](monitoring.md) — monitoring stack that uses the Grafana and Prometheus settings.
- [Tuning architecture](../architecture/tuning.md) — how tuning scripts are executed.
- [Deploying a single-DC cluster](single-dc-cluster.md) — creating a ScyllaCluster.
