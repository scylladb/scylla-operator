# Prerequisites

This page lists the requirements that your Kubernetes cluster must meet before you install ScyllaDB Operator.

## Kubernetes version

ScyllaDB Operator requires a conformant Kubernetes cluster. The table below shows the supported Kubernetes versions for each Operator release.

| Operator version | Kubernetes |
|------------------|------------|
| master | 1.31 – 1.34 |
| v1.19 | 1.31 – 1.34 |
| v1.18 | 1.30 – 1.33 |

For a full compatibility matrix covering ScyllaDB versions, ScyllaDB Manager versions, and OpenShift, see [Releases](../reference/releases.md).

:::{caution}
We **strongly** recommend using a [supported Kubernetes environment](#supported-environments). Issues on unsupported environments are unlikely to be addressed.
:::

## Supported environments

The following platforms and OS images are tested and supported:

| Platform | OS image |
|----------|----------|
| GKE | Ubuntu |
| EKS | Amazon Linux |
| OpenShift | 4.20 |

### Known incompatible environments

| Platform | OS image | Issue |
|----------|----------|-------|
| GKE | Container OS | Lacks XFS support, which is required by the Local CSI Driver and NodeConfig. |
| EKS | Bottlerocket | Suspected kernel/cgroups issue that breaks available memory detection for ScyllaDB. |

## Dependencies

### cert-manager

ScyllaDB Operator uses [cert-manager](https://cert-manager.io/) to provision the TLS serving certificate for its webhook server. Without cert-manager, the Operator cannot start because the Kubernetes API server cannot verify webhook connections.

Any cert-manager version that provides the `cert-manager.io/v1` API is sufficient. The bundled third-party manifests ship cert-manager v1.17.4.

### Prometheus Operator (optional)

ScyllaDB Operator does not include its own metrics collection engine. If you want to use `ScyllaDBMonitoring` resources to deploy managed Prometheus and Grafana instances, you need [Prometheus Operator](https://prometheus-operator.dev/) installed in the cluster. The bundled third-party manifests ship Prometheus Operator v0.86.1.

:::{note}
ScyllaDB Operator currently references the Prometheus Operator CRDs (`ServiceMonitor`, `PrometheusRule`, `Prometheus`) in its RBAC rules. If the CRDs are not installed, the Operator may log warnings about missing Prometheus types. These warnings do not affect the core functionality of ScyllaDB Operator (cluster creation, scaling, upgrades, etc.). Support for making the Prometheus Operator dependency fully optional is tracked in [#3075](https://github.com/scylladb/scylla-operator/issues/3075).
:::

## CRI API

ScyllaDB Operator requires **CRI API v1** (Container Runtime Interface). All modern Kubernetes distributions provide this.

## Kubelet static CPU policy

To get the best performance and lowest latency, ScyllaDB pods should run under the kubelet's **static CPU manager policy**. This policy assigns exclusive CPUs to containers with a Guaranteed QoS class (where resource requests equal limits for all containers in the pod).

Without a static CPU policy, the Linux CFS scheduler may move ScyllaDB threads between CPU cores, which increases context-switching overhead and cache misses.

::::{tabs}
:::{group-tab} GKE

Use `--system-config-from-file` when creating the node pool:

```yaml
# system-config.yaml
cpuManagerPolicy: static
```

```bash
gcloud container node-pools create scylla-pool \
  --system-config-from-file=system-config.yaml \
  ...
```
:::

:::{group-tab} EKS

Set `kubeletExtraConfig` in the eksctl nodegroup configuration:

```yaml
managedNodeGroups:
- name: scylla-pool
  ...
  kubeletExtraConfig:
    cpuManagerPolicy: static
```
:::

:::{group-tab} OpenShift

Configure the kubelet via a `KubeletConfig` custom resource:

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: KubeletConfig
metadata:
  name: scylla-cpumanager
spec:
  kubeletConfig:
    cpuManagerPolicy: static
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
```
:::
::::

:::{note}
Configuring the CPU manager policy is required only on nodes where ScyllaDB pods will run. It does not need to be set cluster-wide. See [CPU pinning](../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md) for a full walkthrough.
:::

## Node labels and taints

The installation guides and examples assume that the Kubernetes nodes dedicated to running ScyllaDB carry the following label and taint:

| Kind | Key | Value |
|------|-----|-------|
| Label | `scylla.scylladb.com/node-type` | `scylla` |
| Taint | `scylla-operator.scylladb.com/dedicated` | `scyllaclusters:NoSchedule` |

The **label** lets NodeConfig, DaemonSets, and monitoring target only ScyllaDB nodes. The **taint** prevents non-ScyllaDB workloads from being scheduled onto those nodes. Your ScyllaCluster spec must include a matching `toleration` for the taint and a `nodeSelector` or `nodeAffinity` for the label.

These values are conventions used throughout the documentation and examples. You may use different labels and taints as long as you adjust all selectors, tolerations, and NodeConfig placements accordingly.

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container node-pools create scylla-pool \
  --node-labels='scylla.scylladb.com/node-type=scylla' \
  --node-taints='scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule' \
  ...
```
:::

:::{group-tab} EKS

```yaml
managedNodeGroups:
- name: scylla-pool
  labels:
    scylla.scylladb.com/node-type: scylla
  taints:
  - key: scylla-operator.scylladb.com/dedicated
    value: "scyllaclusters"
    effect: NoSchedule
```
:::
::::

## XFS filesystem tools

ScyllaDB requires storage formatted with **XFS**. The NodeConfig controller uses `mkfs.xfs` (from the `xfsprogs` package) to format local disks. If `xfsprogs` is not installed on the host, NodeConfig disk setup will fail.

Most Kubernetes node images include `xfsprogs` by default. A notable exception is **GKE with Ubuntu images starting from version 1.32.1-gke.1002000**, where the package was removed.

:::{dropdown} Installing xfsprogs on GKE nodes
:open:

Apply a DaemonSet that installs the package on affected nodes:

```bash
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/install-xfsprogs.daemonset.yaml
```

The DaemonSet uses a `nodeSelector` for `scylla.scylladb.com/node-type: scylla`, so it only runs on ScyllaDB nodes.
:::

## Firewall rules

If your cluster runs behind a firewall or uses network policies, ensure that the following ports are accessible between ScyllaDB pods:

| Port | Protocol | Purpose |
|------|----------|---------|
| 7000 | TCP | Inter-node communication |
| 7001 | TCP | Inter-node communication (TLS) |
| 9042 | TCP | CQL native transport |
| 9142 | TCP | CQL native transport (TLS) |
| 19042 | TCP | CQL shard-aware transport |
| 19142 | TCP | CQL shard-aware transport (TLS) |
| 7199 | TCP | JMX |
| 9180 | TCP | Prometheus metrics (ScyllaDB) |
| 10001 | TCP | ScyllaDB Manager Agent API |

For multi-datacenter deployments across Kubernetes clusters, pod-to-pod communication must be routed between clusters. See [Multi-DC infrastructure](set-up-multi-dc-infrastructure.md) for platform-specific firewall configuration.

## Operator resource requirements

The Operator deployment itself has minimal resource requirements:

| Component | CPU request | Memory request |
|-----------|-------------|----------------|
| scylla-operator | 100m | 20Mi |
| webhook-server | 10m | 20Mi |

These are the requests set in the default manifests. The Operator does not set hard limits, allowing it to burst when reconciling many clusters. For most environments, the defaults are sufficient.

## Related pages

- [Releases](../reference/releases.md) — full support matrix with ScyllaDB and platform versions.
- [GitOps installation](install-with-gitops.md) — step-by-step installation using manifests.
- [Install with Helm](install-with-helm.md) — step-by-step installation using Helm charts.
- [Dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) — configuring ScyllaDB to run on dedicated nodes.
- [Node configuration](../deploy-scylladb/before-you-deploy/configure-nodes.md) — setting up NodeConfig for disk and performance tuning.
