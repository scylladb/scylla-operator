# Configure nodes

This page explains how to use the `NodeConfig` resource to prepare Kubernetes nodes for running ScyllaDB with optimal performance.

## Why NodeConfig matters

ScyllaDB is a high-performance database that makes intensive use of direct I/O, asynchronous I/O, and NUMA-aware CPU scheduling. Out-of-the-box Kubernetes nodes are not tuned for this workload — kernel parameters are too conservative, NVMe drives are not aggregated into a single volume, and interrupt affinity is not aligned with ScyllaDB's CPU cores.

NodeConfig bridges this gap. It runs a privileged DaemonSet on every matching node and performs:

- **Disk setup** — creates RAID0 arrays from NVMe instance storage, formats them with XFS, and mounts them so the Local CSI Driver can provision PersistentVolumes.
- **Kernel tuning** — sets sysctls like `fs.aio-max-nr` and `fs.nr_open` that ScyllaDB requires for high-throughput I/O.
- **Performance tuning** — runs `perftune.py` to configure IRQ affinity, clock source, and other system-level settings.
- **Container-level tuning** — raises `RLIMIT_NOFILE` and adjusts IRQ CPU masks for individual ScyllaDB containers after they start.

:::{warning}
We recommend trying performance tuning on a pre-production instance first. The underlying tuning scripts change kernel and device settings at the host level. Undoing these changes requires rebooting the Kubernetes node.
:::

## Prerequisites

- ScyllaDB Operator installed ([GitOps](../../install-operator/install-with-gitops.md) or [Helm](../../install-operator/install-with-helm.md))
- Dedicated node pool for ScyllaDB (see [Dedicated node pools](set-up-dedicated-node-pools.md))
- `xfsprogs` available on the host OS (see [Prerequisites](../../install-operator/prerequisites.md))

## Matching NodeConfig to ScyllaDB nodes

NodeConfig targets nodes using `placement`, which supports `nodeSelector`, `tolerations`, and `affinity`. These must match the same nodes where ScyllaDB pods run.

The recommended pattern uses the label `scylla.scylladb.com/node-type: scylla` and the taint `scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule` on dedicated nodes. Both the NodeConfig and the ScyllaCluster must reference the same label and tolerate the same taint.

**NodeConfig placement:**

```yaml
spec:
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

**ScyllaCluster rack placement:**

```yaml
spec:
  datacenter:
    racks:
    - name: us-east-1a
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
```

Both target the same nodes. If NodeConfig and ScyllaCluster target different node sets, performance tuning will not apply to ScyllaDB pods.

:::{tip}
For multi-datacenter clusters using multiple `ScyllaCluster` resources, create a separate NodeConfig in each Kubernetes cluster.
:::

## Configuring disk setup

NodeConfig can create RAID arrays from local NVMe instance storage, format them with XFS, and mount them for the Local CSI Driver.

### RAID, filesystem, and mount

::::{tabs}
:::{group-tab} EKS

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          modelRegex: Amazon EC2 NVMe Instance Storage
          nameRegex: ^/dev/nvme\d+n\d+$
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```
:::

:::{group-tab} GKE

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          nameRegex: ^/dev/nvme\d+n\d+$
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```
:::

:::{group-tab} Loop device (no NVMe)

For development environments without local NVMe storage, NodeConfig can create a loop-backed device:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    loopDevices:
    - name: persistent-volumes
      imagePath: /var/lib/persistent-volumes.img
      size: 80Gi
    filesystems:
    - device: /dev/loops/persistent-volumes
      type: xfs
    mounts:
    - device: /dev/loops/persistent-volumes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```
:::
::::

The setup pipeline runs in order:

1. **Loop devices** (if configured) — creates sparse backing files and sets up loop devices.
2. **RAID arrays** — aggregates matching NVMe devices into a RAID0 array under `/dev/md/<name>`.
3. **Filesystems** — formats the device with XFS (skips if already formatted).
4. **Mounts** — mounts the device at the specified mount point with the given options.

After this, the Local CSI Driver can provision PersistentVolumes from directories on the mount point. Pods requesting the `scylladb-local-xfs` StorageClass receive volumes backed by this storage.

:::{note}
The `prjquota` mount option is **required** for the Local CSI Driver. It enables XFS project quotas, which the driver uses to enforce per-volume capacity limits.
:::

### Enabling XFS online discard

On SSD-backed storage, enabling the `discard` mount option allows the filesystem to issue TRIM commands to the underlying device in real time. This helps the SSD controller maintain write performance and extends drive lifetime by promptly reclaiming freed blocks.

Online discard is preferred over periodic `fstrim` because it avoids the latency spikes and I/O pauses that batch trimming can cause on a busy ScyllaDB node.

To enable online discard, add `discard` to the `unsupportedOptions` list:

```yaml
spec:
  localDiskSetup:
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
      - discard
```

:::{note}
Verify that your storage device supports the `discard` operation before enabling this option. Most modern NVMe SSDs support it, but some cloud instance storage types may not.
:::

## Configuring kernel parameters

NodeConfig can set kernel parameters (sysctls) on matching nodes. The following values are recommended for ScyllaDB workloads:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

:::{caution}
These sysctl values are based on the upstream ScyllaDB recommendations. They may change between ScyllaDB versions. Review the [ScyllaDB system configuration guide](https://docs.scylladb.com/stable/operating-scylla/admin.html) when upgrading.
:::

### Setting `fs.nr_open` for RLIMIT_NOFILE

ScyllaDB opens a large number of file descriptors — one per shard for each SSTable, plus connections, sockets, and internal handles. The kernel parameter `fs.nr_open` sets the **maximum value** that `RLIMIT_NOFILE` (the per-process file descriptor limit) can be raised to.

The Operator's container-level tuning automatically raises `RLIMIT_NOFILE` on the ScyllaDB process to the value of `fs.nr_open`. If `fs.nr_open` is too low (the default is 1,048,576), ScyllaDB may fail to open files under heavy load.

Set `fs.nr_open` to a high value via NodeConfig sysctls:

```yaml
sysctls:
- name: fs.nr_open
  value: "1073741816"
```

This value is applied at the host level. The Operator then uses it as the ceiling when raising `RLIMIT_NOFILE` inside each ScyllaDB container.

## Performance tuning

By default, NodeConfig enables performance tuning on all matching nodes. The tuning runs at two levels:

| Level | What it does |
|-------|-------------|
| **Node-level** | Runs `perftune.py --tune=system --tune-clock` to configure clocksource, IRQ balancing, and kernel I/O scheduler. Applies sysctls from the NodeConfig spec. |
| **Container-level** | Adjusts IRQ CPU affinity masks for each ScyllaDB container based on its cgroup CPU allocation. Raises `RLIMIT_NOFILE` on the ScyllaDB process. |

Container-level tuning runs every time a ScyllaDB container starts and **blocks ScyllaDB startup** until it completes (via the [ignition mechanism](../../understand/ignition.md)). This ensures ScyllaDB always starts with the correct tuning.

To disable performance tuning (not recommended for production):

```yaml
spec:
  disableOptimizations: true
```

For details on how tuning interacts with CPU pinning, see [CPU pinning](configure-cpu-pinning.md) and [Tuning architecture](../../understand/tuning.md).

## Combining disk setup and sysctls

A full production NodeConfig typically combines disk setup, sysctls, and placement in a single resource:

::::{tabs}
:::{group-tab} EKS

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          modelRegex: Amazon EC2 NVMe Instance Storage
          nameRegex: ^/dev/nvme\d+n\d+$
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
      - discard
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```
:::

:::{group-tab} GKE

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          nameRegex: ^/dev/nvme\d+n\d+$
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
      - discard
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```
:::
::::

## Apply and verify

```shell
kubectl apply --server-side -f nodeconfig.yaml
```

Wait for the NodeConfig to finish reconciling:

```shell
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

Check the status of each node:

```shell
kubectl get nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

### Verify node configuration

After the NodeConfig is ready, confirm that the disk setup, filesystem, and sysctls were applied correctly.

**Verify RAID array (if configured):**
```bash
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- lsblk
```
Expected output shows the RAID device (`md0` or similar) and member disks:
```
NAME      MAJ:MIN RM   SIZE RO TYPE  MOUNTPOINT
nvme0n1   259:0    0 1.7T  0 disk
└─md0       9:0    0 3.5T  0 raid0
nvme1n1   259:1    0 1.7T  0 disk
└─md0       9:0    0 3.5T  0 raid0
```

**Verify XFS filesystem:**
```bash
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- blkid /dev/md0
```
Expected output:
```
/dev/md0: UUID="..." TYPE="xfs"
```

**Verify mount point:**
```bash
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- df -h /mnt/hostfs/mnt/raid-disks/
```
Expected output:
```
Filesystem      Size  Used Avail Use% Mounted on
/dev/md0        3.5T   34M  3.5T   1% /mnt/raid-disks
```

**Verify sysctls (if configured):**
```bash
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- sysctl fs.nr_open
```
Expected output:
```
fs.nr_open = 20000500
```

Replace `<node-tuning-pod>` with the name of a pod in the `scylla-operator-node-tuning` namespace: `kubectl get pods -n scylla-operator-node-tuning`.

:::{note}
NodeConfig is not available as a Helm chart. Even if you installed the Operator using Helm, apply NodeConfig using `kubectl`.
:::

## Related pages

- [Dedicated node pools](set-up-dedicated-node-pools.md) — setting up isolated nodes for ScyllaDB.
- [CPU pinning](configure-cpu-pinning.md) — configuring CPU exclusivity for ScyllaDB containers.
- [Tuning architecture](../../understand/tuning.md) — how node-level and container-level tuning work.
- [Storage architecture](../../understand/storage.md) — how NodeConfig, Local CSI Driver, and PVs work together.
- [Production checklist](../production-checklist.md) — verify all production settings.
- [Deploy a single-DC cluster](../deploy-single-dc-cluster.md) — creating a ScyllaCluster.
