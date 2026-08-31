# Scale, add, remove racks

Change the number of ScyllaDB nodes in a rack or add entirely new racks to adjust capacity and throughput.

## How scaling works

Each rack in a ScyllaDB cluster maps to a single Kubernetes StatefulSet.
Scaling changes the replica count of that StatefulSet:

- **Scale up** — new pods are appended at the end of the ordinal sequence (highest index).
  After the new node joins the token ring, the Operator automatically triggers a [data cleanup](../understand/automatic-data-cleanup.md) on affected nodes.
- **Scale down** — the Operator decommissions the highest-ordinal pod first, streams its data to the remaining nodes, reduces the replica count, and then deletes the PVC and Service.
  Only one node is decommissioned at a time.

Because StatefulSets maintain contiguous pod ordinals and scale down from the highest ordinal, you cannot remove an arbitrary node from the middle of a rack.
If a specific node is unhealthy, use [node replacement](replace-nodes.md) instead.

For background on the StatefulSet-per-rack architecture, see [StatefulSets and racks](../understand/statefulsets-and-racks.md).

## Sequential and parallel node provisioning

The Operator controls how it provisions new nodes with parallel node operations. They determine whether Operator starts ScyllaDB nodes one at a time or all at once: when you create a cluster, add, or scale-out a rack.

With parallel node operations disabled, Operator starts ScyllaDB nodes one at a time. Within a rack, each Pod must become ready before Operator starts the next one. Operator creates racks one at a time. Bringing up a cluster takes as long as the sum of every node's startup time.

With parallel node operations enabled, Operator starts all ScyllaDB nodes at once. Operator starts Pods of a rack without waiting for the previous ones to become ready. Operator creates all racks at the same time. Bringing up a cluster is faster, and the difference grows with the number of nodes.

Parallel node operations provide better performance when your keyspaces are backed by tablets (the default since ScyllaDB 2025.2), as opposed to vnodes. Therefore, we strongly recommend that you only use tablets for your data keyspaces. This is because with vnode-based keyspaces, a joining node streams its data before it finishes joining, which slows down bringing up new nodes. With tablets, the data is moved in the background after the node joins, so the time to bring up new nodes is not affected by data streaming.
Consider setting [`tablets_mode_for_new_keyspaces`](https://docs.scylladb.com/manual/stable/architecture/tablets.html#enabling-tablets) to `enforced` in your [ScyllaDB configuration](../deploy-scylladb/deploy-your-first-cluster.md#create-a-scylladb-configuration) to prevent individual keyspaces from opting out of tablets.

:::{note}
The Operator waits for the cluster to settle before creating new racks. An in-flight scaling operation, configuration update, or version upgrade delays the creation of new nodes either way.
:::

### Configure parallel node operations

:::{tip}
You can enable or disable parallel node operations at any time. Changing it does not disrupt the running nodes.
:::

You can configure parallel node operations with the `spec.enableParallelNodeOperations` field of a `ScyllaCluster`, which accepts `true` and `false`.

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  enableParallelNodeOperations: true
```

:::{caution}
The minimum ScyllaDB version required by Operator for parallel node operations is 2026.2.
The Operator determines the ScyllaDB version from the ScyllaDB container image tag and rejects `true` when the version doesn't satisfy the requirement. An image whose version cannot be determined, such as one pinned by digest, is treated as not supporting parallel bootstrap.
:::

:::{warning}
The field currently only controls how nodes are started. Its scope is expected to widen in a future release, where it will also allow decommissioning nodes in parallel.
Setting it to `true` now takes effect for those operations after you upgrade the Operator, without another change to the spec.
:::

If you don't specify the field, the Operator defaults it to `true` on creation, provided the ScyllaDB version is higher or equal to 2026.2.

Operator keeps bootstrapping the nodes of clusters that already existed before the addition of this feature sequentially. 
It is recommended that you set the field to `true` explicitly to bootstrap new nodes in parallel.

## Bootstrap synchronisation

In Kubernetes, Pods can start simultaneously, and a new node could attempt to bootstrap while another node is still restarting and appears down to its peers. ScyllaDB denies such a join request and leaves the new node in a state that is not recoverable automatically.

Enabling the `BootstrapSynchronisation` feature gate protects against this by holding each node's startup until all nodes in the cluster are UP. 
It is recommended that you enable it whether or not parallel node operations are enabled. See [Bootstrap synchronisation](../understand/bootstrap-sync.md) for details on the mechanism and [Feature gates](../reference/feature-gates.md) for instructions on enabling feature gates.

## Scale a ScyllaCluster

Change `spec.datacenter.racks[].members` to the desired node count and apply:

::::{tabs}
:::{group-tab} Scale up
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3          # was 1, now 3
        storage:
          capacity: 500Gi
```
:::
:::{group-tab} Scale down
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 1          # was 3, now 1
        storage:
          capacity: 500Gi
```
:::
::::

Wait for the operation to complete:

```bash
kubectl -n scylla wait --timeout=10m --for='condition=Available' scyllaclusters.scylla.scylladb.com/scylla
```

Verify with `nodetool status`:

```bash
kubectl -n scylla exec -it scylla-us-east-1a-0 -c scylla -- nodetool status
```

## Add a rack to a ScyllaCluster

Append a new entry to the `spec.datacenter.racks` array.
The Operator creates racks in the order they appear and waits for each rack to be fully ready before creating the next.

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
      - name: us-east-1b         # new rack
        members: 3
        storage:
          capacity: 500Gi
```

:::{note}
Rack names serve as identity — they determine the StatefulSet and Service names.
Choose rack names carefully, as renaming a rack requires removing it and creating a new one.
:::

### Remove a rack

Removing a rack is a two-step process.
You must scale the rack to zero members first, wait for decommissioning to finish, and only then remove the rack definition from the spec.

#### Step 1: Scale the rack down to 0 members

Update the ScyllaCluster spec to set `members: 0` for the rack being removed:

```bash
# Zero-based index of the rack being removed in spec.datacenter.racks. Change it for your cluster.
RACK_INDEX=0

kubectl -n scylla patch scyllacluster scylla --type=json --patch-file=/dev/stdin <<EOF
[{"op": "replace", "path": "/spec/datacenter/racks/${RACK_INDEX}/members", "value": 0}]
EOF
```

Wait for the Operator to decommission all nodes in the rack:

```bash
kubectl -n scylla wait --timeout=30m \
  --for='condition=Available=True' scyllacluster/scylla
```

Verify all pods in the rack are gone:

```bash
kubectl -n scylla get pods -l scylla/rack=<rack-name>
```

Expected output: no pods listed.

Verify no member of the rack is still leaving the cluster:

```bash
kubectl -n scylla describe scyllacluster scylla
```

Under `Status` → `Racks`, the entry of the rack being removed has to look like this:

```console
  Racks:
    us-east-1a:
      Available Members:  0
      Members:            0
      Ready Members:      0
      Stale:              false
      Updated Members:    0
      Version:            2026.2.5
```

A rack that still has a member leaving lists it under `Decommissioning Members`:

```console
  Racks:
    us-east-1a:
      Available Members:  0
      Conditions:
        Status:  True
        Type:    MemberLeaving
      Decommissioning Members:
        Name:           scylla-us-east-1-us-east-1a-0
      Members:          0
      Ready Members:    0
      Stale:            false
      Updated Members:  0
      Version:          2026.2.5
```

A member is listed there from the moment its decommission is requested until it has been decommissioned and its Service and PVC have been removed.

#### Step 2: Remove the rack definition from the spec

Remove the rack entry from `spec.datacenter.racks`:

```bash
# The same rack index as in step 1.
RACK_INDEX=0

kubectl -n scylla patch scyllacluster scylla --type=json --patch-file=/dev/stdin <<EOF
[{"op": "remove", "path": "/spec/datacenter/racks/${RACK_INDEX}"}]
EOF
```

The Operator's admission webhook rejects the removal while the rack still has members leaving the cluster, and names the members that hold it up.
The command then fails with:

```console
The ScyllaCluster "scylla" is invalid: spec.datacenter.racks[0]: Forbidden: rack "us-east-1a" can't be removed because it still has members leaving the cluster: scylla-us-east-1-us-east-1a-0; they have to finish decommissioning and be removed first, please retry later
```

Wait for the named members to be removed and retry.

:::{warning}
Removing a rack is irreversible — any data that was stored on the rack's nodes is streamed away during decommission.
:::

After both steps, verify the cluster is healthy:

```bash
kubectl -n scylla wait --timeout=5m \
  --for='condition=Available=True' scyllacluster/scylla
```

:::{note}
In multi-DC clusters using multiple `ScyllaCluster` resources, each datacenter is scaled independently by editing its own `ScyllaCluster` resource.
:::

## Key considerations

```{list-table}
:header-rows: 1

* - Consideration
  - Detail
* - One at a time
  - The Operator scales down one node at a time per rack, ensuring data is streamed away before the next decommission begins.
* - Automatic cleanup
  - After scaling completes, the Operator triggers data cleanup Jobs on affected nodes to remove data that no longer belongs to them.
* - PVC deletion
  - PVCs are deleted after scale-down. The Operator removes the PVC and Service of each decommissioned node after the replica count is reduced.
* - Replication factor
  - Ensure you do not scale below the replication factor of your keyspaces. ScyllaDB will refuse queries if replicas become unavailable.
* - PodDisruptionBudget
  - Each datacenter has a PDB with `maxUnavailable: 1`. This does not block Operator-driven scaling but prevents concurrent pod evictions during node drains.
* - Run repair after scaling
  - After significant scaling operations, run a repair to ensure data consistency across the new token ranges.
```

## Related pages

- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — how StatefulSets map to racks and why mid-set removal is not possible
- [Bootstrap synchronisation](../understand/bootstrap-sync.md) — the mechanism gating bootstrap on the cluster's node statuses
- [Replace nodes](replace-nodes.md) — replacing a specific unhealthy node without scaling
- [Migrate a rack to a new node pool](migrate-rack-to-new-node-pool.md) — scaling up a new rack and scaling down the old one to migrate infrastructure
- [Perform a rolling restart](perform-rolling-restart.md) — restarting all nodes without changing the cluster size
- [Data distribution with tablets](https://docs.scylladb.com/manual/stable/architecture/tablets.html) — how tablets distribute data and why they speed up topology changes
