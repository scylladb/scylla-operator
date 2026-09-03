# Decommission a datacenter

Remove an entire datacenter from a multi-datacenter ScyllaDB cluster without losing data or disrupting the remaining datacenters.

## When to use this procedure

This document will guide you through the process of removing the datacenter of your choice from your multi-datacenter ScyllaDB cluster.

This procedure applies to multi-datacenter ScyllaDB clusters deployed as one `ScyllaCluster` per datacenter, joined with `externalSeeds`, as described in [Deploy a multi-datacenter ScyllaDB cluster](../deploy-scylladb/deploy-multi-datacenter-cluster.md).

:::{warning}
Decommissioning a datacenter is irreversible.
Once the datacenter's replicas are dropped from keyspace replication and its nodes are decommissioned, the only way to get the datacenter back is to re-add it as a new datacenter and stream all data again.
:::

:::{caution}
To prevent data loss and preserve cluster integrity, first read the entire procedure cover to cover, then follow the steps precisely in order.
:::

## How it works

ScyllaDB Operator only automates operations within a single datacenter.
Removing a whole datacenter is a manual, cross-datacenter procedure that combines ScyllaDB-level steps (repair, replication changes) with Operator-level steps (scaling racks to zero, updating seeds).
It follows the upstream [Decommissioning a Data Center](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/decommissioning-data-center.html) procedure, with `nodetool decommission` replaced by an Operator-driven scale-down:
when you scale a rack down, the Operator decommissions the highest-ordinal node, waits for its data to stream away, and then deletes the pod, its PVC, and its Service — one node at a time.

## Prerequisites

- All nodes in all datacenters are up and in `UN` state (`nodetool status`).
- The remaining datacenters have enough capacity and a high enough replication factor to serve your workload on their own.
- No ScyllaDB Manager tasks depend on the datacenter being removed.
  If ScyllaDB Manager is deployed in the Kubernetes cluster hosting the datacenter you are removing, migrate it to one of the remaining datacenters first.
- `kubectl` access to all Kubernetes clusters involved.

Throughout this guide, the datacenter being decommissioned is `us-east-2`, deployed as a `ScyllaCluster` named `scylla-cluster` in namespace `scylla` of the Kubernetes cluster reachable via context `${CONTEXT_DC2}`.
The remaining datacenter is `us-east-1`, reachable via `${CONTEXT_DC1}`.
Adjust the names, namespaces, and contexts to your deployment.

## Procedure

### Step 1: Stop client traffic to the datacenter

Reconfigure your applications so that they no longer connect to the nodes of the datacenter being decommissioned:

- Point the datacenter-aware load balancing policy of your drivers (the "local DC") at one of the remaining datacenters.
- Use local consistency levels (`LOCAL_ONE`, `LOCAL_QUORUM`) instead of global ones (`ALL`, `EACH_QUORUM`, `QUORUM`), so that requests do not wait for replicas in the datacenter being removed.

Keep clients away from the datacenter for the rest of the procedure.

:::{note}
[Maintenance mode](use-maintenance-mode.md) is not a substitute for reconfiguring clients.
It only removes a node from its Kubernetes Service endpoints — drivers that discover nodes through ScyllaDB's topology metadata (for example in a [PodIP-based multi-datacenter setup](../deploy-scylladb/deploy-multi-datacenter-cluster.md#networking)) connect to the nodes directly and are not affected.
It also marks the node permanently unready, so the `ScyllaCluster` reports `Available=False`, and Operator-driven changes to racks other than the one being scaled stall until the label is removed.
If your clients do connect through Kubernetes Services and you want to drop nodes from Service endpoints as they are removed, use the [rack-by-rack variant](#alternative-decommission-one-rack-at-a-time-under-maintenance-mode) of Step 4, which keeps maintenance mode scoped to the rack currently being scaled down.
:::

### Step 2: Repair the cluster

Repair makes sure that the remaining datacenters hold an up-to-date copy of every write the decommissioned datacenter has seen.

For keyspaces backed by tablets (the default since ScyllaDB 2025.2), run a single cluster-wide repair from any node:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool cluster repair
```

For vnode-based keyspaces (`tablets = {'enabled': false}`), run a primary-range repair on **every node of the datacenter being decommissioned**, one node at a time:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool repair -pr
```

If you use ScyllaDB Manager, you can run an ad-hoc [repair task](https://manager.docs.scylladb.com/stable/repair/) instead.

### Step 3: Remove the datacenter from keyspace replication

List your keyspaces and their replication settings.
You can run `cqlsh` from any node:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- cqlsh -e "SELECT keyspace_name, replication FROM system_schema.keyspaces"
```

For **every** keyspace that lists the datacenter being decommissioned in its replication map — including system keyspaces such as `audit`, if present — alter the keyspace so that the datacenter no longer holds replicas.

For a vnode-based keyspace, drop the datacenter in a single statement by leaving it out of the replication map:

```cql
ALTER KEYSPACE ks_vnodes WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3};
```

For a tablets-based keyspace, ScyllaDB only allows changing the replication factor of one datacenter at a time, and only by one — attempting a larger change is rejected with `Only one DC's RF can be changed at a time and not by more than 1`.
Step the replication factor down to zero one `ALTER` at a time, ending with an explicit `0` (omitting the datacenter is rejected with `Attempted to implicitly drop replicas in datacenter ...`):

```cql
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 2};
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 1};
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 0};
```

After the final `ALTER`, ScyllaDB migrates the keyspace's tablets out of the datacenter in the background.

:::{warning}
Do not perform any reads or writes that involve the decommissioned datacenter after this step, and wait for each `ALTER` to complete before issuing the next one.
:::

### Step 4: Scale the datacenter down to zero nodes

Set `members: 0` on every rack of the datacenter's `ScyllaCluster`:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla edit scyllaclusters.scylla.scylladb.com/scylla-cluster
```

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  datacenter:
    name: us-east-2
    racks:
      - name: a
        members: 0          # was 1, now 0
      - name: b
        members: 0          # was 1, now 0
      - name: c
        members: 0          # was 1, now 0
```

The Operator decommissions the nodes one at a time, streaming any remaining data away before deleting each pod, its PVC, and its Service.
Wait for the scale-down to finish — with large datasets this can take a long time:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --timeout=60m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```

Verify from one of the remaining datacenters that the decommissioned datacenter is gone from the token ring:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- nodetool status
```

The output must list only the remaining datacenters, with all nodes in `UN` state:

```console
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens       Owns    Host ID                               Rack
UN  10.0.70.195  705 KB     256          ?       494277b9-121c-4af9-bd63-3d0a7b9305f7  c
UN  10.0.59.24   764 KB     256          ?       a3a98e08-0dfd-4a25-a96a-c5ab2f47eb37  b
UN  10.0.19.237  634 KB     256          ?       64b6292a-327f-4128-852a-6004039f402e  a
```

#### Alternative: decommission one rack at a time under maintenance mode

If your clients reach ScyllaDB through Kubernetes Services (rather than connecting to Pod IPs directly), you can combine a rack-by-rack scale-down with [maintenance mode](use-maintenance-mode.md) to drop each rack's nodes from Service endpoints just before they are decommissioned.
Maintenance mode does not stop the ScyllaDB process, so the Operator still decommissions the rack's nodes cleanly, one at a time, streaming their data away as usual.

For each rack of the datacenter, one rack at a time:

1. Wait for the `ScyllaCluster` to settle:

   ```bash
   kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
   ```

2. Enable maintenance mode on all member Services of the rack (rack `a` in this example):

   ```bash
   kubectl --context="${CONTEXT_DC2}" -n=scylla label svc -l='scylla/cluster=scylla-cluster,scylla/rack=a' scylla/node-maintenance=""
   ```

3. Set the rack's `members` to 0 and wait for the scale-down to finish:

   ```bash
   kubectl --context="${CONTEXT_DC2}" -n=scylla wait --timeout=60m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
   ```

:::{warning}
Only the rack currently being scaled down may have nodes in maintenance mode.
A maintenance-mode node in any other rack stalls all Operator-driven changes to the datacenter: the `ScyllaCluster` stays at `Progressing=True` with reason `WaitingForStatefulSetRollout` for that rack's StatefulSet, and the scale-down never starts.
If that happens, remove the `scylla/node-maintenance` label from the other racks' Services to unblock it.
:::

:::{note}
While a rack's nodes are in maintenance mode but not yet scaled down, the `ScyllaCluster` reports `Available=False` with reason `MembersNotReady`.
Use `Progressing=False` to sequence the racks, and expect `Available=True` only once the datacenter has no members left.
:::

### Step 5: Remove the datacenter from the seeds of the remaining datacenters

If any of the remaining datacenters lists addresses of the decommissioned datacenter's nodes in `spec.externalSeeds`, remove them:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla edit scyllaclusters.scylla.scylladb.com/scylla-cluster
```

Stale seeds would otherwise point at addresses that no longer belong to the cluster and may even get reused by unrelated workloads.

:::{note}
`externalSeeds` is part of the pod template, so changing it triggers a rolling restart of the datacenter's nodes, one node at a time.
:::

### Step 6: Delete the ScyllaCluster and clean up

Delete the `ScyllaCluster` of the decommissioned datacenter:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla delete scyllaclusters.scylla.scylladb.com/scylla-cluster
```

The Operator already deleted the PVCs of the decommissioned nodes during the scale-down, but auxiliary objects (Secrets, ConfigMaps) may remain.
If the namespace was dedicated to this datacenter, delete it:

```bash
kubectl --context="${CONTEXT_DC2}" -n=scylla get all,pvc
kubectl --context="${CONTEXT_DC2}" delete namespace scylla
```

You can now also remove any `ScyllaDBMonitoring`, the ScyllaDB Manager deployment, the ScyllaDB Operator, and the Kubernetes cluster itself, if nothing else uses them.

Finally, verify the health of the cluster from one of the remaining datacenters and spot-check your data with a local consistency level:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- nodetool status
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- cqlsh -e "CONSISTENCY LOCAL_QUORUM; SELECT * FROM ks_tablets.t LIMIT 10"
```

## Key considerations

```{list-table}
:header-rows: 1

* - Consideration
  - Detail
* - Order matters
  - Repair and replication changes must happen while the datacenter's nodes are still up; scaling to zero must happen before deleting the `ScyllaCluster`.
* - Irreversible
  - After the replication change and decommission, re-adding the datacenter means bootstrapping it from scratch.
* - Tablets vs vnodes
  - Tablets keyspaces need staged `ALTER KEYSPACE` statements (RF changes of one at a time, explicit `0` at the end) and are repaired with a single `nodetool cluster repair`; vnode keyspaces are dropped in one `ALTER` and repaired with `nodetool repair -pr` on every node of the datacenter.
* - Consistency levels
  - Global consistency levels (`ALL`, `EACH_QUORUM`, `QUORUM`) may fail or block during the procedure; use `LOCAL_*` levels.
* - ScyllaDB Manager
  - If Manager runs in the datacenter being removed, migrate it first; a datacenter that disappears mid-task leaves failing repair/backup tasks behind.
```

## Related pages

- [Deploy a multi-datacenter ScyllaDB cluster](../deploy-scylladb/deploy-multi-datacenter-cluster.md) — the deployment this procedure reverses
- [Scale, add, remove racks](scale-add-remove-racks.md) — scale-down mechanics and rack removal within a datacenter
- [Decommissioning a Data Center](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/decommissioning-data-center.html) — the upstream ScyllaDB procedure
- [nodetool alternatives](../reference/nodetool-alternatives.md) — a cheat-sheet of `nodetool` commands that are usable with Operator-managed ScyllaDB clusters
