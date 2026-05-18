# Set up dedicated node pools

ScyllaDB requires dedicated Kubernetes nodes to guarantee predictable performance.
These nodes should not run any other workloads.

## Why dedicate nodes to ScyllaDB

ScyllaDB is designed to consume all available CPU, memory, and I/O on its host.
Sharing nodes with other workloads leads to:

- **Resource contention** — noisy neighbours competing for CPU cycles and memory bandwidth cause unpredictable latency.
- **CPU pinning failures** — the kubelet's static CPU manager policy can only assign exclusive cores when the node is not overcommitted.
- **Incorrect performance tuning** — `NodeConfig` tunes IRQ affinity, clocksource, and sysctls at the host level, affecting all Pods on the node.

## Requirements

Dedicated ScyllaDB nodes must have:

- **Local NVMe storage** — ScyllaDB stores data on local SSDs for maximum I/O performance.
  Use instance types with directly attached NVMe drives.
- **Sufficient CPU and memory** — see the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for minimum and recommended specifications.
  Plan for at least 2 CPUs reserved for the OS, kubelet, and `DaemonSets`.
- **xfsprogs installed** — `NodeConfig` formats local disks with XFS. The `xfsprogs` package must be available on the node OS image.
  Most managed Kubernetes distributions include it by default, but custom or minimal images may not.

### Recommended instance types

::::{tabs}

:::{tab} GKE
Use `n2-highmem` or `z3-highmem` families with local SSDs.
See [GCE recommendations](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#google-compute-engine-gce).
:::

:::{tab} EKS
Use storage-optimized instances from the `i` family (e.g., `i3en`, `i4i`, `i8g`).
See [AWS recommendations](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws).
:::

:::{tab} OKE
Use `DenseIO` shapes.
See [OCI recommendations](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#oracle-cloud-infrastructure-oci).
:::

:::{tab} OpenShift
Instance type recommendations depend on the underlying cloud provider.
Use the same instance families as the corresponding platform (e.g., `i` family on AWS for ROSA, `n2-highmem` on GCP for OSD).
Configure node pools using [MachineSets](https://docs.openshift.com/container-platform/latest/machine_management/creating_machinesets/creating-machineset-aws.html) or [MachinePools](https://docs.openshift.com/rosa/rosa_cluster_admin/rosa_nodes/rosa-managing-worker-nodes.html) depending on your OpenShift variant.
:::

::::

## Label the nodes

Apply the ScyllaDB node label so that Pod affinity rules can target these nodes:

:::{code-block} console
kubectl label nodes <node-name> scylla.scylladb.com/node-type=scylla
:::

When using managed Kubernetes services, apply the label at node pool creation time to avoid manual labeling:

::::{tabs}

:::{tab} GKE
`gcloud container node-pools create` flag:

```
--node-labels 'scylla.scylladb.com/node-type=scylla'
```

See the [GKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-gke-cluster.md) for a complete example.
:::

:::{tab} EKS
`eksctl create cluster` configuration:

```yaml
labels:
  scylla.scylladb.com/node-type: scylla
```

See the [EKS cluster setup guide](../../install-operator/provision-infrastructure/set-up-eks-cluster.md) for a complete example.
:::

:::{tab} OKE
`oci ce node-pool create` flag:

```
--initial-node-labels '[{"key": "scylla.scylladb.com/node-type", "value": "scylla"}]'
```

See the [OKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-oke-cluster.md) for a complete example.
:::

:::{tab} OpenShift
Configure labels on your node pool using [MachineSets](https://docs.openshift.com/container-platform/latest/machine_management/creating_machinesets/creating-machineset-aws.html) or [MachinePools](https://docs.openshift.com/rosa/rosa_cluster_admin/rosa_nodes/rosa-managing-worker-nodes.html) depending on your OpenShift variant.

```yaml
metadata:
  labels:
    scylla.scylladb.com/node-type: scylla
```
:::

::::

## Taint the nodes

Apply a taint to prevent non-ScyllaDB workloads from being scheduled on the dedicated nodes:

:::{code-block} console
kubectl taint nodes -l 'scylla.scylladb.com/node-type=scylla' \
  scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule --overwrite
:::

The `ScyllaCluster` manifest must include a matching toleration, and the `NodeConfig` placement must tolerate the same taint.
The [reference deployments](../reference-deployments/index.md) include both.

:::{note}
The label `scylla.scylladb.com/node-type: scylla` and taint `scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule` are conventions used throughout the documentation and examples.
You may use different values as long as you adjust all selectors, tolerations, and `NodeConfig` placements accordingly.
:::

## Verify the setup

:::{code-block} console
# Check that nodes have the label
kubectl get nodes -l scylla.scylladb.com/node-type=scylla

# Verify the taint
kubectl get nodes -l scylla.scylladb.com/node-type=scylla \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}'

# After deploying, confirm ScyllaDB Pods run only on dedicated nodes
kubectl get pods -l scylla/cluster=scylladb -o wide
:::

## Platform-specific guides

For end-to-end node pool creation on a specific platform, see the [platform-specific cluster setup guides](../../install-operator/provision-infrastructure/index.md).
