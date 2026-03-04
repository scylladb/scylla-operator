# Storage

ScyllaDB is a high-performance database that benefits significantly from fast, low-latency storage.
This page explains how ScyllaDB Operator integrates with Kubernetes storage, the tradeoffs between local and network-attached storage, and how the ScyllaDB Local CSI Driver works.

## Local vs. network-attached storage

ScyllaDB works with both local and network-attached storage provisioners.
However, **local NVMe storage provides substantially better performance** and is the recommended choice for production deployments.

| | Local NVMe storage | Network-attached storage (e.g., EBS, Persistent Disk) |
|---|---|---|
| **Latency** | Microseconds — direct access to physical drives on the host | Milliseconds — network round-trip to remote block device |
| **Throughput** | Full drive bandwidth, often multiple NVMes in RAID0 | Limited by network bandwidth and provider throttling |
| **Data durability on node loss** | Data is lost when the node is lost. ScyllaDB's replication across multiple nodes provides durability. | Data survives node loss. PersistentVolume can be reattached to a new node. |
| **Dynamic provisioning** | Requires a local provisioner (e.g., ScyllaDB Local CSI Driver) | Natively supported by most cloud providers |
| **Setup complexity** | Requires NodeConfig for RAID/filesystem setup and a local provisioner | Minimal — use the cloud provider's default StorageClass |

:::{tip}
For production workloads, use **local NVMe storage** with the ScyllaDB Local CSI Driver.
Network-attached storage is acceptable for development and evaluation but is not recommended for latency-sensitive workloads.
:::

## Setting up local disks with NodeConfig

When a Kubernetes node with local disks is created, the storage is typically uninitialized.
Even when the platform mounts the disks, it usually does not configure RAID or format them with the correct filesystem.
ScyllaDB requires storage formatted with **XFS**.

The `NodeConfig` custom resource automates disk preparation on Kubernetes nodes.
It can:

- Create **RAID0 arrays** from multiple NVMe devices.
- Format the array with **XFS**.
- Mount it at a specified path (typically `/var/lib/persistent-volumes`).
- Apply **mount options** such as `prjquota` (required by the Local CSI Driver for quota enforcement).

For configuration details, see [Node Configuration](../deploying/node-configuration.md).

## ScyllaDB Local CSI Driver

The [ScyllaDB Local CSI Driver](https://github.com/scylladb/local-csi-driver) is a [Container Storage Interface (CSI)](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/) driver that enables dynamic provisioning of PersistentVolumes on local disks.

### How it works

The driver runs as a privileged DaemonSet on every ScyllaDB node.
When a ScyllaDB pod is scheduled and its PersistentVolumeClaim needs to be bound, the driver:

1. Creates a **directory** on the local filesystem (under `/var/lib/persistent-volumes` by default).
2. Sets an **XFS project quota** on the directory to enforce the volume size limit specified in the PVC.
3. Provisions a **PersistentVolume** backed by this directory and binds it to the PVC.

This allows multiple ScyllaDB pods on the same node to share the underlying disk while each having an isolated, quota-limited volume.

### StorageClass

The driver uses the `scylladb-local-xfs` StorageClass:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: scylladb-local-xfs
provisioner: local.csi.scylladb.com
volumeBindingMode: WaitForFirstConsumer
parameters:
  csi.storage.k8s.io/fstype: xfs
```

The `WaitForFirstConsumer` binding mode ensures the volume is provisioned on the same node where the ScyllaDB pod is scheduled.

### Requirements

- The underlying filesystem must be **XFS** with the `prjquota` mount option enabled (configured via NodeConfig).
- The driver must be deployed as a DaemonSet on all nodes that run ScyllaDB pods.
- Node selector `scylla.scylladb.com/node-type: scylla` is used by default to target ScyllaDB nodes.

### Limitations

- The Local CSI Driver provisions **directories**, not block devices.
- Storage capacity is shared across all volumes on the same node. XFS project quotas enforce per-volume limits, but total capacity is bounded by the underlying disk.

## Alternative: SIG Storage Static Local Volume Provisioner

The [Kubernetes SIG Storage Local Persistence Volume Static Provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner) is an alternative that turns local disks into dedicated PersistentVolumes.
Unlike the ScyllaDB Local CSI Driver, it does not support dynamic provisioning — each PersistentVolume must be set up manually.

Use this provisioner if you need each ScyllaDB pod to have exclusive access to an entire disk or partition rather than sharing a disk via quota-limited directories.

## Using network-attached storage

If you choose network-attached storage, use the StorageClass provided by your cloud platform (e.g., `pd-ssd` on GKE, `gp3` on EKS).
Set the `storageClassName` in your ScyllaCluster rack's `storage` field.
No NodeConfig or Local CSI Driver is needed in this case.

:::{caution}
Network-attached storage adds latency to every I/O operation.
ScyllaDB's tail latencies and throughput will be significantly impacted.
This setup is not recommended for production.
:::
