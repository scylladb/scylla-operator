# What Is ScyllaDB Operator?

ScyllaDB Operator is a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that automates the deployment and lifecycle management of [ScyllaDB](https://www.scylladb.com) clusters on Kubernetes.

## What problems does it solve?

Running ScyllaDB on Kubernetes without an operator requires manually handling many tasks that the Operator automates:

| Concern | Without the Operator | With ScyllaDB Operator |
|---|---|---|
| **Provisioning** | Manually create StatefulSets, Services, ConfigMaps, PVCs, and configure ScyllaDB seed lists, snitch settings, and rack/DC placement. | Declare a `ScyllaCluster` resource. The Operator creates and configures all Kubernetes objects. |
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

`ScyllaCluster` is the stable (`scylla.scylladb.com/v1`) API for deploying ScyllaDB on Kubernetes. Each `ScyllaCluster` resource represents a single datacenter. You define racks within the datacenter, and the Operator creates a StatefulSet per rack.

The Operator automates all single-datacenter operations: provisioning, scaling, upgrades, node replacement, tuning, repairs, backups, and monitoring.

### Multi-datacenter clusters

To deploy a ScyllaDB cluster that spans multiple Kubernetes clusters or geographic regions, create one `ScyllaCluster` resource in each Kubernetes cluster and connect them using the `externalSeeds` field. Each `ScyllaCluster` must use the same `.metadata.name` (which becomes the ScyllaDB cluster name) and specify seed node addresses from the other datacenters in `externalSeeds`.

The Operator manages each datacenter independently. The following must be managed manually when using multiple `ScyllaCluster` resources for multi-datacenter deployments:

- **Seed configuration** — the `externalSeeds` field of each `ScyllaCluster` must be populated with the node addresses of the other datacenters. These addresses must be kept up to date if nodes are replaced or IPs change.
- **ScyllaDB Manager auth token synchronization** — when ScyllaDB Manager is deployed in one datacenter, its auth token Secret must be manually copied to each other datacenter and the Manager Agent pods restarted. See [ScyllaDB Manager](../understand/manager.md).
- **Replication factor configuration** — keyspace replication factors must be updated via CQL after adding a new datacenter, using `ALTER KEYSPACE`.
- **Network peering and firewall rules** — inter-cluster Pod-to-Pod connectivity must be configured at the infrastructure level. See [Set up multi-DC infrastructure](../install-operator/set-up-multi-dc-infrastructure.md).

For step-by-step instructions, see [Deploy a multi-datacenter cluster](../deploy-scylladb/deploy-multi-dc-cluster.md).

## Key resources

The Operator provides the following custom resources:

| Resource | API version | Scope | Purpose |
|---|---|---|---|
| `ScyllaCluster` | `v1` | Namespaced | A ScyllaDB datacenter (one resource per DC; for multi-DC, create one per Kubernetes cluster and connect via `externalSeeds`) |
| `NodeConfig` | `v1alpha1` | Cluster | Node-level disk setup, RAID, filesystem, and performance tuning |
| `ScyllaOperatorConfig` | `v1alpha1` | Cluster | Global Operator configuration (images, cluster domain) |
| `ScyllaDBMonitoring` | `v1alpha1` | Namespaced | Monitoring stack (Prometheus + Grafana) for ScyllaDB |
| `ScyllaDBManagerTask` | `v1alpha1` | Namespaced | Backup or repair task managed by ScyllaDB Manager |

## Supported platforms

ScyllaDB Operator is tested and supported on:

- **Google Kubernetes Engine (GKE)** with Ubuntu node images
- **Amazon Elastic Kubernetes Service (EKS)** with Amazon Linux node images
- **Architectures**: amd64 and arm64

For the full support matrix including supported Kubernetes versions, ScyllaDB versions, and known platform incompatibilities, see [Releases](../reference/releases.md).

## Next steps

- [Install](../install-operator/index.md) ScyllaDB Operator in your Kubernetes cluster.
- [Deploy your first cluster](../deploy-scylladb/deploy-your-first-cluster.md) once the Operator is installed.
- See a complete end-to-end walkthrough for [GKE](../deploy-scylladb/reference-deployment-gke.md), [EKS](../deploy-scylladb/reference-deployment-eks.md), or [OpenShift](../deploy-scylladb/reference-deployment-openshift.md).
- Learn about the Operator's [architecture](../understand/index.md).
