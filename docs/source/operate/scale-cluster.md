# Scale a ScyllaDB cluster

Change the number of ScyllaDB nodes in a rack or add entirely new racks to adjust capacity and throughput.

## How scaling works

Each rack in a ScyllaDB cluster maps to a single Kubernetes StatefulSet.
Scaling changes the replica count of that StatefulSet:

- **Scale up** — new pods are appended at the end of the ordinal sequence (highest index).
  After the new node joins the token ring, the Operator automatically triggers a data cleanup on affected nodes.
- **Scale down** — the Operator decommissions the highest-ordinal pod first, streams its data to the remaining nodes, deletes the PVC, and then reduces the replica count.
  Only one node is decommissioned at a time.

Because StatefulSets use `OrderedReady` pod management, you cannot remove an arbitrary node from the middle of a rack.
If a specific node is unhealthy, use [node replacement](replace-nodes.md) instead.

For background on the StatefulSet-per-rack architecture, see [StatefulSets and racks](../understand/statefulsets-and-racks.md).

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
Rack names are immutable — once created, a rack name cannot be changed.
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
Rack names are immutable. After a rack is deleted, you cannot add a rack with the same name.
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

| Consideration | Detail |
|---|---|
| One at a time | The Operator scales down one node at a time per rack, ensuring data is streamed away before the next decommission begins. |
| Automatic cleanup | After scaling completes, the Operator triggers data cleanup Jobs on affected nodes to remove data that no longer belongs to them. |
| PVC retention | PVCs are retained after scale-down. If you scale back up to the same ordinal, the new pod reattaches to the old PVC and the Operator handles it as a node replacement. |
| Replication factor | Ensure you do not scale below the replication factor of your keyspaces. ScyllaDB will refuse queries if replicas become unavailable. |
| PodDisruptionBudget | Each datacenter has a PDB with `maxUnavailable: 1`. This does not block Operator-driven scaling but prevents concurrent pod evictions during node drains. |
| Run repair after scaling | After significant scaling operations, run a repair to ensure data consistency across the new token ranges. |

## Related pages

- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — how StatefulSets map to racks and why mid-set removal is not possible
- [Replace nodes](replace-nodes.md) — replacing a specific unhealthy node without scaling
- [Migrate a rack to a new node pool](migrate-rack-to-new-node-pool.md) — scaling up a new rack and scaling down the old one to migrate infrastructure
- [Perform a rolling restart](perform-rolling-restart.md) — restarting all nodes without changing the cluster size
