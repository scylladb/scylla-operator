# Configure nodes

ScyllaDB Operator uses the `NodeConfig` custom resource to prepare dedicated nodes for ScyllaDB.
`NodeConfig` handles local disk setup (RAID, filesystems, mounts) and kernel tuning (sysctls) automatically.

## What NodeConfig does

When you create a `NodeConfig` resource, the operator deploys a `DaemonSet` on matching nodes that:

1. **Configures local disks** — creates a RAID0 array from NVMe devices, formats it with XFS, and mounts it to a well-known path.
2. **Tunes kernel parameters** — sets sysctls for high-throughput I/O (`fs.aio-max-nr`, `fs.file-max`, `vm.swappiness`, etc.).
3. **Runs performance tuning** — executes `ContainerPerftune` Jobs for IRQ balancing and other low-latency optimizations.

## Matching placement

`NodeConfig` targets nodes using `placement` (with `nodeSelector` and `tolerations`).
These must match the same nodes where ScyllaDB Pods will run — if `NodeConfig` and `ScyllaCluster` target different node sets, performance tuning and disk setup will not apply to ScyllaDB Pods.

The recommended convention is label `scylla.scylladb.com/node-type: scylla` and taint `scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule`.
Both `NodeConfig` placement and `ScyllaCluster` rack placement must reference the same label and tolerate the same taint.

## Apply a NodeConfig

`NodeConfig` manifests are platform-specific because disk device paths and naming conventions differ.

::::{tabs}

:::{tab} GKE

```{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/nodeconfig.yaml
```

:::

:::{tab} EKS

```{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/eks/nodeconfig.yaml
```

:::

:::{tab} OKE

```{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/oke/nodeconfig.yaml
```

:::

:::{tab} OpenShift (ROSA)

```{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/openshift/rosa/nodeconfig.yaml
```

:::

::::

Wait for `NodeConfig` to finish reconciling:

:::{code-block} console
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
:::

## Disk setup

`NodeConfig` creates RAID arrays from local NVMe instance storage, formats them with XFS, and mounts them for the Local CSI Driver.

The setup pipeline runs in order: loop devices (if configured) → RAID arrays → filesystems → mounts.
After this, the Local CSI Driver can provision `PersistentVolumes` from directories on the mount point.

### Platform differences

::::{tabs}

:::{tab} GKE

Devices are matched by path only:

```yaml
raids:
- name: nvmes
  type: RAID0
  RAID0:
    devices:
      nameRegex: ^/dev/nvme\d+n\d+$
```

:::

:::{tab} EKS

EKS nodes may have an NVMe root volume alongside instance storage.
Use `modelRegex` to select only instance storage devices:

```yaml
raids:
- name: nvmes
  type: RAID0
  RAID0:
    devices:
      modelRegex: Amazon EC2 NVMe Instance Storage
      nameRegex: ^/dev/nvme\d+n\d+$
```

:::

:::{tab} OKE

OKE `DenseIO` shapes expose NVMe devices at `/dev/nvme*`.
Devices are matched by path only (same as GKE):

```yaml
raids:
- name: nvmes
  type: RAID0
  RAID0:
    devices:
      nameRegex: ^/dev/nvme\d+n\d+$
```

:::

:::{tab} OpenShift (ROSA)

ROSA on AWS uses the same instance families as EKS.
Use `modelRegex` to select only instance storage devices:

```yaml
raids:
- name: nvmes
  type: RAID0
  RAID0:
    devices:
      modelRegex: Amazon EC2 NVMe Instance Storage
      nameRegex: ^/dev/nvme\d+n\d+$
```

:::

:::{tab} Loop device (development)

For environments without local NVMe storage, `NodeConfig` can create a loop-backed device:

```yaml
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
```

:::

::::

:::{note}
The `prjquota` mount option is **required** for the Local CSI Driver.
It enables XFS project quotas, which the driver uses to enforce per-volume capacity limits.
:::

### XFS online discard

On SSD-backed storage, enabling `discard` allows the filesystem to issue TRIM commands in real time, helping the SSD controller maintain write performance.
This is preferred over periodic `fstrim` which can cause latency spikes on a busy ScyllaDB node.

:::{code-block} yaml
mounts:
- device: /dev/md/nvmes
  mountPoint: /var/lib/persistent-volumes
  unsupportedOptions:
  - prjquota
  - discard
:::

## Kernel parameters

Add the following to the `sysctls` field in your `NodeConfig` manifest.
These are the recommended values for ScyllaDB workloads:

:::{code-block} yaml
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
:::

### fs.nr_open and RLIMIT_NOFILE

ScyllaDB opens a large number of file descriptors — one per shard for each SSTable, plus connections and internal handles.
The kernel parameter `fs.nr_open` sets the **maximum value** that `RLIMIT_NOFILE` (the per-process file descriptor limit) can be raised to.

The Operator automatically raises `RLIMIT_NOFILE` on the ScyllaDB process to the value of `fs.nr_open`.
If `fs.nr_open` is too low (the default is 1,048,576), ScyllaDB may fail to open files under heavy load.

## Verify NodeConfig

Check that the `NodeConfig` reports healthy status:

:::{code-block} console
kubectl get nodeconfigs.scylla.scylladb.com
:::

Expected output:

:::{code-block}
NAME                  AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb-nodepool-1   True        False         False      5m
:::

### Verify host-level configuration

After `NodeConfig` is ready, confirm the disk setup and sysctls on a node-tuning Pod.

Find the Pod name first:

:::{code-block} console
kubectl get pods -n scylla-operator-node-tuning
:::

Then run the verification commands, replacing `<node-tuning-pod>` with an actual Pod name from the output above:

:::{code-block} console
# Verify RAID array
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- lsblk

# Verify XFS filesystem
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- blkid /dev/md0

# Verify mount
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- df -h /mnt/hostfs/mnt/raid-disks/

# Verify sysctls
kubectl -n scylla-operator-node-tuning exec -it <node-tuning-pod> -- sysctl fs.nr_open
:::

## Install Local CSI Driver

The Local CSI Driver is needed for provisioning `PersistentVolumes` for `ScyllaClusters` using the mounted storage:

:::{code-block} console
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

Wait for the driver to roll out:

:::{code-block} console
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
:::
