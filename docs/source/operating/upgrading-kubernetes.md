# Upgrading Kubernetes

Upgrade the Kubernetes control plane and node pools on GKE or EKS while keeping your ScyllaDB clusters available.

:::{caution}
Cloud provider node pool upgrade mechanisms (GKE surge upgrades, EKS managed node group rolling updates) impose **timeout limits on PDB-respecting drains**.
After the timeout (1 hour on GKE, configurable on EKS), pods are forcefully evicted regardless of PodDisruptionBudgets.
ScyllaDB data streaming during decommission or rebuild can easily exceed these limits.

For ScyllaDB node pools, use the [rack migration procedure](#upgrading-scylladb-node-pools) instead of in-place node pool upgrades.
:::

## Before you begin

1. Verify that the target Kubernetes version is supported by your ScyllaDB Operator version.
   Check the [support matrix](../reference/releases.md).

2. Ensure the cluster is healthy — no degraded ScyllaClusters or pending rollouts:
   ```bash
   kubectl get scyllaclusters -A -o wide
   ```

3. Review your cluster's node pools and identify which ones run ScyllaDB:
   ```bash
   kubectl get nodes -l scylla.scylladb.com/node-type=scylla
   ```

## Upgrade the control plane

Control plane upgrades do not affect running workloads — ScyllaDB pods continue running while the Kubernetes API server is updated.

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container clusters upgrade scylladb-cluster \
  --region=us-central1 \
  --master \
  --cluster-version=<target-version>
```

Regional clusters upgrade one control plane replica at a time with no API downtime.
Zonal clusters have a brief API server outage during upgrade, but workloads are unaffected.
:::

:::{group-tab} EKS

```bash
aws eks update-cluster-version \
  --name scylladb-cluster \
  --kubernetes-version <target-version>
```

Wait for the update to complete:

```bash
aws eks wait cluster-active --name scylladb-cluster
```

EKS performs in-place control plane upgrades with no API downtime.
You can only upgrade one minor version at a time.
:::
::::

## Upgrading infrastructure node pools

Infrastructure node pools (running system components, cert-manager, Prometheus, and the Operator itself) can be upgraded using the cloud provider's built-in mechanism.
These workloads tolerate brief disruptions and do not require ScyllaDB-specific streaming.

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container node-pools upgrade infra-pool \
  --region=us-central1 \
  --cluster=scylladb-cluster \
  --node-version=<target-version>
```

The default surge upgrade strategy (`maxSurge=1, maxUnavailable=0`) is safe for infrastructure pools.
:::

:::{group-tab} EKS

```bash
aws eks update-nodegroup-version \
  --cluster-name scylladb-cluster \
  --nodegroup-name infra-pool \
  --kubernetes-version <target-version>
```

EKS managed node groups use a rolling update — one node at a time is drained and replaced.
:::
::::

Wait for the Operator to be available after the infrastructure pool upgrade:

```bash
kubectl -n scylla-operator rollout status deployment/scylla-operator
kubectl -n scylla-operator rollout status deployment/webhook-server
```

## Upgrading ScyllaDB node pools

:::{warning}
Do **not** use GKE's node pool upgrade or EKS managed node group updates on pools running ScyllaDB.

- **GKE** respects PDBs during drain for only 1 hour, then forcefully evicts pods.
- **EKS** has a configurable drain timeout, but pods are still force-evicted when the timeout expires.

ScyllaDB nodes need to fully decommission and stream data before shutdown.
Force-evicting a ScyllaDB pod mid-stream can leave the token ring inconsistent.
:::

Instead, use a **create-and-migrate** approach:

### Step 1: Disable auto-upgrade and auto-repair

Ensure that the cloud provider does not upgrade ScyllaDB node pools automatically.

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container node-pools update scylladb-pool \
  --region=us-central1 \
  --cluster=scylladb-cluster \
  --no-enable-autoupgrade \
  --no-enable-autorepair
```
:::

:::{group-tab} EKS

EKS managed node groups do not auto-upgrade.
If you use a custom AMI launch template, ensure you control the AMI version.
:::
::::

### Step 2: Create a new node pool with the target Kubernetes version

Create a new pool with the same configuration (instance type, labels, taints, local SSDs) but on the target Kubernetes version.

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container node-pools create scylladb-pool-v2 \
  --region=us-central1 \
  --cluster=scylladb-cluster \
  --node-version=<target-version> \
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
:::

:::{group-tab} EKS

Add a new node group to your `eksctl` cluster config and create it:

```yaml
nodeGroups:
- name: scylla-pool-v2
  instanceType: i4i.2xlarge
  desiredCapacity: 3
  amiFamily: AmazonLinux2023
  labels:
    scylla.scylladb.com/node-type: scylla
  taints:
    scylla-operator.scylladb.com/dedicated: "scyllaclusters:NoSchedule"
  kubeletExtraConfig:
    cpuManagerPolicy: static
```

```bash
eksctl create nodegroup -f cluster.yaml --include='scylla-pool-v2'
```
:::
::::

### Step 3: Verify NodeConfig targets the new nodes

Ensure your NodeConfig's `nodeSelector` matches both old and new pools (both have the `scylla.scylladb.com/node-type=scylla` label).
The NodeConfig DaemonSet automatically tunes the new nodes.

Wait for the NodeConfig to report the new nodes as tuned:

```bash
kubectl get nodeconfigs -o wide
```

### Step 4: Migrate racks to the new node pool

Follow the full procedure in [Migrating a rack to a new node pool](migrating-rack-to-new-node-pool.md).

In summary:
1. Add a new rack targeting the new node pool.
2. Scale up the new rack one node at a time.
3. Scale down the old rack one node at a time (each node is properly decommissioned).
4. Remove the old rack from the spec once it has zero members.

This ensures every node is properly decommissioned with data fully streamed to remaining nodes before shutdown.

### Step 5: Delete the old node pool

After all racks have been migrated and the old pool has no ScyllaDB pods:

::::{tabs}
:::{group-tab} GKE

```bash
gcloud container node-pools delete scylladb-pool \
  --region=us-central1 \
  --cluster=scylladb-cluster
```
:::

:::{group-tab} EKS

```bash
eksctl delete nodegroup --cluster=scylladb-cluster --name=scylla-pool
```
:::
::::

### Step 6: Repair

Run a repair after migration to ensure data consistency across the new token ranges:

```bash
kubectl -n scylla exec scylladb-us-east-1a-0 -c scylla -- nodetool repair
```

Or let ScyllaDB Manager handle repairs through scheduled repair tasks.

## PodDisruptionBudget behaviour

The Operator creates one PDB per datacenter with `maxUnavailable: 1`.
This means at most one ScyllaDB pod in a datacenter can be voluntarily disrupted at any time.

During node drains (from `kubectl drain` or cloud provider upgrades), the PDB prevents the kubelet from evicting more than one ScyllaDB pod simultaneously.
However, the PDB only controls **voluntary** disruptions — node crashes, kernel panics, or cloud provider forced terminations bypass the PDB.

For a full explanation of how PDBs interact with ScyllaDB pods, see [Pod disruption budgets](../architecture/pod-disruption-budgets.md).

## Related pages

- [Pod disruption budgets](../architecture/pod-disruption-budgets.md) — how PDBs protect ScyllaDB availability
- [Migrating a rack to a new node pool](migrating-rack-to-new-node-pool.md) — step-by-step rack migration procedure
- [Dedicated node pools](../deploying/dedicated-node-pools.md) — creating node pools with labels and taints
- [Upgrading ScyllaDB Operator](upgrading-operator.md) — upgrading the Operator itself
- [Prerequisites](../installation/prerequisites.md) — supported Kubernetes versions
