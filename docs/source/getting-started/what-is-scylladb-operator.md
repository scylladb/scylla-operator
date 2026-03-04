# What Is ScyllaDB Operator?

ScyllaDB Operator is a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that automates the deployment and lifecycle management of [ScyllaDB](https://www.scylladb.com) clusters on Kubernetes.

## What problems does it solve?

Running ScyllaDB on Kubernetes without an operator requires manually handling many tasks that the Operator automates:

| Concern | Without the Operator | With ScyllaDB Operator |
|---|---|---|
| **Provisioning** | Manually create StatefulSets, Services, ConfigMaps, PVCs, and configure ScyllaDB seed lists, snitch settings, and rack/DC placement. | Declare a `ScyllaCluster` or `ScyllaDBCluster` resource. The Operator creates and configures all Kubernetes objects. |
| **Scaling** | Manually adjust StatefulSet replicas, update seed lists, and run `nodetool cleanup` on existing nodes after the new node finishes streaming. | Change the `members` count in the spec. The Operator handles the StatefulSet update and runs cleanup automatically when token ownership changes. |
| **Upgrades** | Manually update the container image, coordinate a rolling restart, and verify each node rejoins before proceeding to the next. | Update the image field in the spec. The Operator performs a rolling upgrade one node at a time, verifying health at each step. |
| **Node replacement** | Manually drain the node, remove it from the cluster, recreate the pod, and pass the `replace_address_first_boot` flag. | Label the node's Service for replacement. The Operator handles draining, decommissioning, and bootstrapping the replacement. |
| **Performance tuning** | SSH into each Kubernetes node to run `perftune`, configure IRQ affinity, set sysctls, and prepare RAID/filesystem layouts. | Create a `NodeConfig` resource. The Operator deploys privileged DaemonSets and Jobs that tune each node and prepare disks automatically. |
| **Repairs and backups** | Install and configure ScyllaDB Manager separately, register clusters manually, and schedule tasks. | Define repair and backup tasks in the cluster spec or as `ScyllaDBManagerTask` resources. The Operator deploys Manager and registers clusters. |
| **Monitoring** | Deploy and configure Prometheus, Grafana, ServiceMonitors, and dashboards manually. | Create a `ScyllaDBMonitoring` resource. The Operator provisions the monitoring stack with ScyllaDB-specific dashboards. |
| **TLS** | Generate and distribute certificates, configure ScyllaDB to use them, and handle rotation. | Enable TLS in the spec. The Operator uses cert-manager to provision and rotate certificates automatically. |

## How it works

ScyllaDB Operator extends the Kubernetes API with [CustomResourceDefinitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and runs controllers that continuously reconcile the desired state you declare with the actual state of the cluster.

The Operator ships as a single binary that includes:

- **Controllers** that watch CRDs and create or update the underlying Kubernetes objects (StatefulSets, Services, ConfigMaps, Jobs, PodDisruptionBudgets, and more).
- **Admission webhooks** that validate and default your resource specs before they are persisted.
- **A sidecar** that runs inside each ScyllaDB pod to configure and manage the ScyllaDB process.
- **Node setup agents** that run on Kubernetes nodes to prepare disks and tune the host.

## Single-DC and multi-DC deployment models

ScyllaDB Operator supports two deployment models:

### Single-datacenter: `ScyllaCluster` (stable)

`ScyllaCluster` is a stable (`scylla.scylladb.com/v1`) API that manages a ScyllaDB cluster within a single Kubernetes cluster. Each `ScyllaCluster` represents one datacenter. You define racks, and the Operator creates a StatefulSet per rack.

Use `ScyllaCluster` when your ScyllaDB cluster runs entirely within one Kubernetes cluster.

### Multi-datacenter: `ScyllaDBCluster` (v1alpha1)

`ScyllaDBCluster` is a `scylla.scylladb.com/v1alpha1` API that orchestrates a ScyllaDB cluster spanning multiple Kubernetes clusters. It coordinates with `RemoteKubernetesCluster` resources to manage datacenters in remote clusters.

:::{caution}
`ScyllaDBCluster` uses the `v1alpha1` API version. The API may change in incompatible ways in future releases. Evaluate carefully before using it in production.
:::

Use `ScyllaDBCluster` when your ScyllaDB cluster needs to span multiple Kubernetes clusters or multiple geographic regions.

## Key resources

The Operator provides the following custom resources:

| Resource | API version | Scope | Purpose |
|---|---|---|---|
| `ScyllaCluster` | `v1` | Namespaced | Single-datacenter ScyllaDB cluster |
| `ScyllaDBCluster` | `v1alpha1` | Namespaced | Multi-datacenter ScyllaDB cluster |
| `ScyllaDBDatacenter` | `v1alpha1` | Namespaced | A ScyllaDB datacenter within a single Kubernetes cluster |
| `NodeConfig` | `v1alpha1` | Cluster | Node-level disk setup, RAID, filesystem, and performance tuning |
| `ScyllaOperatorConfig` | `v1alpha1` | Cluster | Global Operator configuration (images, cluster domain) |
| `ScyllaDBMonitoring` | `v1alpha1` | Namespaced | Monitoring stack (Prometheus + Grafana) for ScyllaDB |
| `RemoteKubernetesCluster` | `v1alpha1` | Cluster | Connection to a remote Kubernetes cluster for multi-DC |
| `ScyllaDBManagerTask` | `v1alpha1` | Namespaced | Backup or repair task managed by ScyllaDB Manager |

## Supported platforms

ScyllaDB Operator is tested and supported on:

- **Google Kubernetes Engine (GKE)** with Ubuntu node images
- **Amazon Elastic Kubernetes Service (EKS)** with Amazon Linux node images
- **Architectures**: amd64 and arm64

For the full support matrix including supported Kubernetes versions, ScyllaDB versions, and known platform incompatibilities, see [Releases](../reference/releases.md).

## Next steps

- Try ScyllaDB Operator with the [GKE Quickstart](quickstart-gke.md) or [EKS Quickstart](quickstart-eks.md).
- Learn about the Operator's [architecture](../architecture/index.md).
- [Install](../installation/index.md) ScyllaDB Operator in your Kubernetes cluster.
