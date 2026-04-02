# Set up dedicated node pools

This page explains how to create dedicated Kubernetes node pools for ScyllaDB and configure all ScyllaDB Operator components to target them.

## Why dedicate nodes to ScyllaDB

ScyllaDB is designed to consume all available CPU, memory, and I/O on its host. Sharing nodes with other workloads leads to unpredictable latency, resource contention, and interference from network or disk IRQs. Running ScyllaDB on dedicated nodes ensures:

- **Consistent performance** — no noisy neighbours competing for CPU cycles or memory bandwidth.
- **Effective CPU pinning** — the kubelet's static CPU manager policy can assign exclusive cores to ScyllaDB only when the node is not overcommitted.
- **Correct performance tuning** — NodeConfig tunes IRQ affinity, clocksource, and sysctls at the host level. These settings affect all pods on the node.

## Recommended pattern

The pattern used throughout all ScyllaDB Operator documentation and examples consists of two Kubernetes primitives:

| Kind | Key | Value |
|------|-----|-------|
| Label | `scylla.scylladb.com/node-type` | `scylla` |
| Taint | `scylla-operator.scylladb.com/dedicated` | `scyllaclusters:NoSchedule` |

The **label** lets NodeConfig, the Local CSI Driver DaemonSet, and monitoring target only ScyllaDB nodes. The **taint** prevents non-ScyllaDB workloads from being scheduled onto those nodes.

All ScyllaDB Operator components (ScyllaCluster, NodeConfig) must include:
- A **node selector** or **node affinity** that requires the label.
- A **toleration** that matches the taint.

:::{note}
These values are conventions used throughout the documentation and examples. You may use different labels and taints as long as you adjust all selectors, tolerations, and NodeConfig placements accordingly.
:::

## Step 1: Create the dedicated node pool

::::{tabs}
:::{group-tab} GKE

```shell
gcloud container \
  node-pools create 'scylladb-pool' \
  --region='us-central1' \
  --node-locations='us-central1-a,us-central1-b,us-central1-c' \
  --cluster='scylladb-cluster' \
  --node-version="latest" \
  --machine-type='n2-standard-8' \
  --num-nodes='1' \
  --disk-type='pd-ssd' --disk-size='20' \
  --local-nvme-ssd-block='count=1' \
  --image-type='UBUNTU_CONTAINERD' \
  --system-config-from-file='systemconfig.yaml' \
  --no-enable-autoupgrade \
  --no-enable-autorepair \
  --node-labels='scylla.scylladb.com/node-type=scylla' \
  --node-taints='scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule'
```

Key flags:
- `--image-type='UBUNTU_CONTAINERD'` — GKE Container OS is not supported (see [Prerequisites](../../install-operator/prerequisites.md)).
- `--local-nvme-ssd-block` — attaches local NVMe SSDs for ScyllaDB data.
- `--system-config-from-file` — enables the kubelet static CPU manager policy (see [CPU pinning](configure-cpu-pinning.md)).
- `--no-enable-autoupgrade --no-enable-autorepair` — prevents GKE from disrupting ScyllaDB nodes.
:::

:::{group-tab} EKS

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: scylladb-cluster
  region: us-east-1
nodeGroups:
- name: scylla-pool
  instanceType: i4i.2xlarge
  desiredCapacity: 3
  amiFamily: AmazonLinux2023
  labels:
    scylla.scylladb.com/node-type: scylla
  taints:
    scylla-operator.scylladb.com/dedicated: "scyllaclusters:NoSchedule"
  kubeletExtraConfig:
    cpuManagerPolicy: static
  availabilityZones:
  - us-east-1a
  - us-east-1b
  - us-east-1c
```

Key settings:
- `instanceType: i4i.2xlarge` — instance types with local NVMe SSDs are recommended.
- `amiFamily: AmazonLinux2023` — Bottlerocket is not supported (see [Prerequisites](../../install-operator/prerequisites.md)).
- `cpuManagerPolicy: static` — enables CPU pinning (see [CPU pinning](configure-cpu-pinning.md)).

Create the cluster with:

```shell
eksctl create cluster -f cluster.yaml
```
:::

:::{group-tab} Generic

For other Kubernetes distributions, label and taint the nodes manually:

```shell
kubectl label node <node-name> scylla.scylladb.com/node-type=scylla
kubectl taint node <node-name> scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule
```

Repeat for each node in the dedicated pool.
:::
::::

## Step 2: Configure ScyllaCluster placement

Set `placement` on each rack in your ScyllaCluster to require the dedicated nodes and tolerate the taint:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  datacenter:
    racks:
    - name: us-east-1a
      members: 3
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
      # ... storage, resources, etc.
```

When spreading across availability zones, add a zone selector to the same `matchExpressions` list:

```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: topology.kubernetes.io/zone
        operator: In
        values:
        - us-east-1a
      - key: scylla.scylladb.com/node-type
        operator: In
        values:
        - scylla
```

For multi-DC clusters using multiple `ScyllaCluster` resources, configure placement on each `ScyllaCluster` independently. Each datacenter's `ScyllaCluster` sets its own rack-level placement to target the dedicated node pool in its Kubernetes cluster.

## Step 3: Configure NodeConfig placement

NodeConfig uses `nodeSelector` (a simpler form) instead of `nodeAffinity`. Both achieve the same result — they target the same set of nodes:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
  # ... localDiskSetup, sysctls
```

:::{warning}
If NodeConfig and ScyllaCluster target different nodes, performance tuning and disk setup will not apply to the nodes running ScyllaDB. Always verify that the placement selectors match.
:::

## Verifying the setup

Check that your dedicated nodes have the correct label and taint:

```shell
kubectl get nodes -l scylla.scylladb.com/node-type=scylla
```

Verify the taint:

```shell
kubectl get nodes -l scylla.scylladb.com/node-type=scylla -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}'
```

After deploying, confirm that ScyllaDB pods are running only on dedicated nodes:

```shell
kubectl get pods -l scylla/cluster=scylladb -o wide
```

The `NODE` column should show only nodes from your dedicated pool.

## Related pages

- [Configure nodes](configure-nodes.md) — configuring disk setup, sysctls, and performance tuning.
- [Configure CPU pinning](configure-cpu-pinning.md) — configuring CPU exclusivity for ScyllaDB containers.
- [Deploy a single-DC cluster](../deploy-single-dc-cluster.md) — creating a ScyllaCluster with placement.
- [Deploy a multi-DC cluster](../deploy-multi-dc-cluster.md) — multi-DC cluster deployment with placement considerations.
- [Prerequisites](../../install-operator/prerequisites.md) — supported platforms and node requirements.
- [Production checklist](../production-checklist.md) — verify all production settings.
