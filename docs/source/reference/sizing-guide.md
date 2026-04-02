# Sizing guide

This page provides guidance on choosing instance types, setting resource requests and limits, and sizing storage for ScyllaDB on Kubernetes.

For platform-specific compatibility, see the [support matrix](releases.md#support-matrix).

## Resource requests and limits

### ScyllaDB container

Both CPU and memory **requests and limits are required** for the ScyllaDB container. The Operator validates this at admission time.

**How ScyllaDB uses container resources:**

- **CPU** — the Operator passes the container's CPU limit to ScyllaDB via the `--smp` flag. Each CPU core becomes one ScyllaDB shard. Use integer CPU values (for example, `4`, not `4000m`) so the kubelet can assign exclusive CPU cores when CPU pinning is enabled.
- **Memory** — ScyllaDB reads its cgroup memory limit and allocates memory accordingly. The Operator does not pass a `--memory` flag. ScyllaDB reserves a portion for internal bookkeeping and uses the rest for the row cache and other structures.

:::{important}
For CPU pinning to work, the pod must have **Guaranteed** QoS class, which requires requests to equal limits for **every** container in the pod — including sidecar containers. See [Configure CPU pinning](../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md).
:::

**Example (production)**:

```yaml
spec:
  datacenter:
    racks:
    - resources:
        requests:
          cpu: 4
          memory: 32Gi
        limits:
          cpu: 4
          memory: 32Gi
```

**Example (development/testing)**:

```yaml
spec:
  datacenter:
    racks:
    - resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
```

:::{note}
Development examples use `developerMode: true`, which disables performance tuning requirements and allows ScyllaDB to run with minimal resources.
:::

### ScyllaDB Manager Agent sidecar

The Manager Agent container shares the pod with ScyllaDB. Its default resource requirements in the ScyllaCluster (`v1`) API are:

```yaml
agentResources:
  requests:
    cpu: 50m
    memory: 10M
```

These defaults set **requests only** with no limits, which makes the pod **Burstable** QoS. To achieve Guaranteed QoS (required for CPU pinning), you must set explicit limits that match the requests:

```yaml
agentResources:
  requests:
    cpu: 50m
    memory: 10Mi
  limits:
    cpu: 50m
    memory: 10Mi
```

:::{note}
For Guaranteed QoS class (required for CPU pinning), requests must equal limits for all containers in the pod.
See [Configure CPU pinning](../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md).
:::

### Other sidecar containers

The Operator sets fixed resource requests and limits on its internal sidecar containers:

| Container | CPU (req/limit) | Memory (req/limit) |
|---|---|---|
| Operator binary injector (init) | 10m / 10m | 50Mi / 50Mi |
| Sysctl buddy (init) | 10m / 10m | 50Mi / 50Mi |
| ScyllaDB API status probe | 50m / 50m | 40Mi / 40Mi |
| ScyllaDB ignition | 50m / 50m | 40Mi / 40Mi |

These are internal containers managed by the Operator. You cannot change their resource requirements.

## Instance types

The instance type determines the available CPU, memory, storage, and network bandwidth for each ScyllaDB node. ScyllaDB benefits from NVMe local SSDs, high memory, and fast networking.

### GKE

Use `n2-highmem` or `c3d-highmem` instances with local NVMe SSDs attached. GKE local SSDs are 375 GiB each, NVMe, and attached directly to the host. For example:

| Instance type | vCPUs | Memory | Local SSDs | Use case |
|---|---|---|---|---|
| `n2-highmem-8` | 8 | 64 GiB | 1–2 × 375 GiB | Development / testing |
| `n2-highmem-16` | 16 | 128 GiB | 2–4 × 375 GiB | Production |
| `n2-highmem-32` | 32 | 256 GiB | 4–8 × 375 GiB | Large production |
| `c3d-highmem-16` | 16 | 128 GiB | local SSD | New deployments (latest generation) |
| `c3d-highmem-30` | 30 | 240 GiB | local SSD | High performance |

:::{note}
The `c3d` family is the latest generation and is recommended for new GKE deployments. The number of local SSDs is configured at node pool creation time.
:::

For GKE node pool configuration, see [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md).

### EKS

Use `i`-family instances with NVMe instance storage. For example:

| Instance type | vCPUs | Memory | NVMe storage |
|---|---|---|---|
| `i4i.xlarge` | 4 | 32 GiB | 1 × 937 GiB |
| `i4i.2xlarge` | 8 | 64 GiB | 1 × 1,875 GiB |
| `i4i.4xlarge` | 16 | 128 GiB | 1 × 3,750 GiB |

:::{note}
The `i3` family is outdated. Use `i4i` or newer instances.
:::

<!-- TODO: Add more instance families (im4gn for ARM, is4gen for storage-dense workloads). Verify pricing and availability. -->

## Storage sizing

ScyllaDB stores data on local disks. Size your storage based on data volume, replication factor, and compaction overhead:

- **Compaction headroom** — size-tiered compaction (the default) requires up to 50% free disk space during compaction. Leveled compaction requires approximately 10% overhead.
- **Replication factor** — total storage needed is `data size × replication factor × compaction overhead`.
- **Commitlog** — stored on the same volume by default. Allocate additional space accordingly.

Set the storage capacity in the ScyllaCluster spec:

```yaml
spec:
  datacenter:
    racks:
    - storage:
        capacity: 100Gi
```

See [Expand storage volumes](../operate/expand-storage-volumes.md) for expanding storage on an existing cluster.

## Network bandwidth

ScyllaDB generates significant inter-node traffic during streaming (scaling, repair, bootstrap) and replication. Use instance types with at least 10 Gbps network bandwidth for production workloads.

## Minimum requirements

| Resource | Development | Production |
|---|---|---|
| CPU cores per node | 1 | ≥ 4 (integer, for CPU pinning) |
| Memory per node | 1 GiB | ≥ 16 GiB |
| Storage | Any | NVMe SSD, XFS filesystem |
| Nodes per datacenter | 1 | ≥ 3 (for fault tolerance) |
| Network | Any | ≥ 10 Gbps |

<!-- TODO: Cross-check minimum requirements against ScyllaDB upstream system requirements at https://docs.scylladb.com/stable/getting-started/system-requirements.html and validate the memory-per-shard rule (~256 MiB per core for internal bookkeeping). -->

## Related pages

- [Production checklist](../deploy-scylladb/production-checklist.md) — complete checklist for production deployments.
- [Configure CPU pinning](../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md) — configuring dedicated CPUs for ScyllaDB.
- [Configure nodes](../deploy-scylladb/before-you-deploy/configure-nodes.md) — RAID, filesystem, and tuning setup.
- [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) — isolating ScyllaDB on dedicated Kubernetes nodes.
