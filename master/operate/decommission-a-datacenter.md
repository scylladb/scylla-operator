# Decommission a datacenter

Remove an entire datacenter from a multi-datacenter ScyllaDB cluster without losing data or disrupting the remaining datacenters.

## When to use this procedure

This document will guide you through the process of removing the datacenter of your choice from your multi-datacenter ScyllaDB cluster.

This procedure applies to multi-datacenter ScyllaDB clusters deployed as one `ScyllaCluster` per datacenter, joined with `externalSeeds`, as described in [Deploy a multi-datacenter ScyllaDB cluster](https://operator.docs.scylladb.com/master/deploy-scylladb/deploy-multi-datacenter-cluster.md).

#### WARNING
Decommissioning a datacenter is irreversible.
Once the datacenter’s replicas are dropped from keyspace replication and its nodes are decommissioned, the only way to get the datacenter back is to re-add it as a new datacenter and stream all data again.

## How it works

ScyllaDB Operator only automates operations within a single datacenter.
Removing a whole datacenter is a manual, cross-datacenter procedure that combines ScyllaDB-level steps (repair, replication changes) with Operator-level steps (scaling racks to zero, updating seeds).
It follows the upstream [Decommissioning a Data Center](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/decommissioning-data-center.html) procedure, with `nodetool decommission` replaced by an Operator-driven scale-down:
when you scale a rack down, the Operator decommissions the leaving nodes, waits for their data to stream away, and then deletes each pod, its PVC, and its Service.
Whether the leaving nodes are decommissioned one at a time or all at once depends on [parallel node operations](https://operator.docs.scylladb.com/master/operate/scale-add-remove-racks.md#sequential-and-parallel-node-operations).

## Prerequisites

- All nodes in all datacenters are up and in `UN` state (`nodetool status`).
- The remaining datacenters have enough capacity and a high enough replication factor to serve your workload on their own.
- No ScyllaDB Manager tasks depend on the datacenter being removed.
  If ScyllaDB Manager is deployed in the Kubernetes cluster hosting the datacenter you are removing, migrate it to one of the remaining datacenters first.
- `kubectl` access to all Kubernetes clusters involved.

Throughout this guide, the datacenter being decommissioned is `us-east-2`, deployed as a `ScyllaCluster` named `scylla-cluster` in namespace `scylla` of the Kubernetes cluster reachable via context `${CONTEXT_DC_TO_DECOMMISSION}`.
The remaining datacenter is `us-east-1`, reachable via `${CONTEXT_DC1}`.
Export the contexts before you begin:

```bash
export CONTEXT_DC1= ... # kubeconfig context of the Kubernetes cluster hosting a remaining DC
export CONTEXT_DC_TO_DECOMMISSION= ... # kubeconfig context of the Kubernetes cluster hosting the DC being decommissioned
```

## Procedure

### Step 1: Stop client traffic to the datacenter

Reconfigure your applications so that they no longer connect to the nodes of the datacenter being decommissioned:

- Point the datacenter-aware load balancing policy of your drivers (the “local DC”) at one of the remaining datacenters.
- Use local consistency levels (`LOCAL_ONE`, `LOCAL_QUORUM`) instead of global ones (`ALL`, `EACH_QUORUM`, `QUORUM`), so that requests do not wait for replicas in the datacenter being removed.

Keep clients away from the datacenter for the rest of the procedure.

### Step 2: Repair the cluster

Repair makes sure that the remaining datacenters hold an up-to-date copy of every write the decommissioned datacenter has seen.

For keyspaces backed by tablets (the default since ScyllaDB 2025.2), run a single cluster-wide repair from any node:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool cluster repair
```

For vnode-based keyspaces (`tablets = {'enabled': false}`), run a primary-range repair on **every node of the datacenter being decommissioned**, one node at a time:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool repair -pr
```

### Step 3: Remove the datacenter from keyspace replication

List your keyspaces and their replication settings.
You can run `cqlsh` from any node:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- cqlsh -e "SELECT keyspace_name, replication FROM system_schema.keyspaces"
```

For **every** keyspace that lists the datacenter being decommissioned in its replication map — including system keyspaces such as `audit`, if present — alter the keyspace so that the datacenter no longer holds replicas.

For a vnode-based keyspace, drop the datacenter in a single statement by leaving it out of the replication map:

```cql
/* In the commmand below, change `ks_vnodes` to the name of your vnode-based keyspace. */
/* Edit the list of datacenters in the `replication` object to match your remaining datacenters. */

ALTER KEYSPACE ks_vnodes WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, };
```

For a tablets-based keyspace, ScyllaDB only allows changing the replication factor of one datacenter at a time, and only by one — attempting a larger change is rejected with `Only one DC's RF can be changed at a time and not by more than 1`.
Step the replication factor down to zero one `ALTER` at a time, ending with an explicit `0` (omitting the datacenter is rejected with `Attempted to implicitly drop replicas in datacenter ...`):

```cql
/* In the commmands below, change `ks_tablets` to the name of your tablet-based keyspace. */
/* Edit the list of datacenters in the `replication` object to match your remaining datacenters. */

/* Reduce us-east-2 from 3 to 2 */
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 2};
/* Reduce us-east-2 from 2 to 1 */
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 1};
/* Reduce us-east-2 from 1 to 0 */
ALTER KEYSPACE ks_tablets WITH replication = {'class': 'NetworkTopologyStrategy', 'us-east-1': 3, 'us-east-2': 0};
``

After the final `ALTER`, ScyllaDB migrates the keyspace's tablets out of the datacenter in the background.

:::{warning}
Do not perform any reads or writes that involve the decommissioned datacenter after this step, and wait for each `ALTER` to complete before issuing the next one.
:::

### Step 4: Scale the datacenter down to zero nodes

Set `members: 0` on every rack of the datacenter's `ScyllaCluster`:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla edit scyllaclusters.scylla.scylladb.com/scylla-cluster
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

The Operator decommissions the nodes — one at a time or all at once, depending on [parallel node operations](https://operator.docs.scylladb.com/master/operate/scale-add-remove-racks.md#sequential-and-parallel-node-operations) — streaming any remaining data away before deleting each pod, its PVC, and its Service.
Wait for the scale-down to finish — with large datasets this can take a long time:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla wait --timeout=60m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
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

### Step 5: Remove the datacenter from the seeds of the remaining datacenters

If any of the remaining datacenters lists addresses of the decommissioned datacenter’s nodes in `spec.externalSeeds`, remove them:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla edit scyllaclusters.scylla.scylladb.com/scylla-cluster
```

Stale seeds would otherwise point at addresses that no longer belong to the cluster and may even get reused by unrelated workloads.

#### NOTE
`externalSeeds` is part of the pod template, so changing it triggers a rolling restart of the datacenter’s nodes, one node at a time.

### Step 6: Delete the ScyllaCluster and clean up

Delete the `ScyllaCluster` of the decommissioned datacenter:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla delete scyllaclusters.scylla.scylladb.com/scylla-cluster
```

The Operator already deleted the PVCs of the decommissioned nodes during the scale-down, but auxiliary objects (Secrets, ConfigMaps) may remain.
If the namespace was dedicated to this datacenter, delete it:

```bash
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" -n=scylla get all,pvc
kubectl --context="${CONTEXT_DC_TO_DECOMMISSION}" delete namespace scylla
```

You can now also remove any `ScyllaDBMonitoring`, the ScyllaDB Manager deployment, the ScyllaDB Operator, and the Kubernetes cluster itself, if nothing else uses them.

Finally, verify the health of the cluster from one of the remaining datacenters and spot-check your data with a local consistency level:

```bash
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- nodetool status
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- cqlsh -e "CONSISTENCY LOCAL_QUORUM; SELECT * FROM ks_tablets.t LIMIT 10"
```

## Related pages

- [Deploy a multi-datacenter ScyllaDB cluster](https://operator.docs.scylladb.com/master/deploy-scylladb/deploy-multi-datacenter-cluster.md) — the deployment this procedure reverses
- [Scale, add, remove racks](https://operator.docs.scylladb.com/master/operate/scale-add-remove-racks.md) — scale-down mechanics and rack removal within a datacenter
- [Decommissioning a Data Center](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/decommissioning-data-center.html) — the upstream ScyllaDB procedure
- [nodetool alternatives](https://operator.docs.scylladb.com/master/reference/nodetool-alternatives.md) — a cheat-sheet of `nodetool` commands that are usable with Operator-managed ScyllaDB clusters
