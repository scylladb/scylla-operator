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

The Operator controls how it provisions new nodes with bootstrap policies. The policy determines whether Operator bootstraps ScyllaDB nodes one at a time or all at once: when you create a cluster, add, or scale-out a rack.

With the `Sequential` policy, Operator bootstraps ScyllaDB nodes one at a time. Within a rack, each Pod must become ready before Operator starts the next one. Operator creates racks one at a time. Bringing up a cluster takes as long as the sum of every node's startup time.

With the `Parallel` policy, Operator starts all ScyllaDB nodes at once. Operator starts Pods of a rack without waiting for the previous ones to become ready. Operator creates all racks at the same time. Bringing up a cluster is faster, and the difference grows with the number of nodes.

`Parallel` provides better performance when your keyspaces are backed by tablets (the default since ScyllaDB 2025.2), as opposed to vnodes. Therefore, we strongly recommend that you only use tablets for your data keyspaces. This is because with vnode-based keyspaces, a joining node streams its data before it finishes joining, which slows down bringing up new nodes. With tablets, the data is moved in the background after the node joins, so the time to bring up new nodes is not affected by data streaming.
Consider setting [`tablets_mode_for_new_keyspaces`](https://docs.scylladb.com/manual/stable/architecture/tablets.html#enabling-tablets) to `enforced` in your [ScyllaDB configuration](../deploy-scylladb/deploy-your-first-cluster.md#create-a-scylladb-configuration) to prevent individual keyspaces from opting out of tablets.

:::{note}
The Operator waits for the cluster to settle before creating new racks. An in-flight scaling operation, configuration update, or version upgrade delays the creation of new nodes under both policies.
:::

### Configure the bootstrap policy

:::{tip}
You can change the bootstrap policy at any time, in either direction. Changing it does not disrupt the running nodes.
:::

You can configure the bootstrap policy with the `spec.bootstrapPolicy` field of a `ScyllaCluster`, which accepts `Sequential` and `Parallel`.

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  bootstrapPolicy: Parallel
```

:::{caution}
The minimum ScyllaDB version required by Operator for `Parallel` bootstrap policy is 2026.2.
The Operator determines the ScyllaDB version from the ScyllaDB container image tag and rejects `Parallel` when the version doesn't satisfy the requirement. An image whose version cannot be determined, such as one pinned by digest, is treated as not supporting parallel bootstrap.
:::

If you don't specify the field, the Operator defaults to `Parallel` on creation, provided the ScyllaDB version is higher or equal to 2026.2.

Operator keeps bootstrapping the nodes of clusters that already existed before the addition of this feature sequentially. 
It is recommended that you configure the field to `Parallel` explicitly to bootstrap new nodes in parallel.

## Bootstrap synchronisation

In Kubernetes, Pods can start simultaneously, and a new node could attempt to bootstrap while another node is still restarting and appears down to its peers. ScyllaDB denies such a join request and leaves the new node in a state that is not recoverable automatically.

Enabling the `BootstrapSynchronisation` feature gate protects against this by holding each node's startup until all nodes in the cluster are UP. 
It is recommended that you enable it with either bootstrap policy. See [Bootstrap synchronisation](../understand/bootstrap-sync.md) for details on the mechanism and [Feature gates](../reference/feature-gates.md) for instructions on enabling feature gates.

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
kubectl -n scylla patch scyllacluster scylla --type=json \
  -p='[{"op":"replace","path":"/spec/datacenter/racks/<index>/members","value":0}]'
```

Replace `<index>` with the zero-based index of the rack in the `racks` array.

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

#### Step 2: Remove the rack definition from the spec

Remove the rack entry from `spec.datacenter.racks`:

```bash
kubectl -n scylla edit scyllacluster scylla
```

Delete the entire rack entry. Save and apply.

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
