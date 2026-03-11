# Replace nodes

Replace a dead or unhealthy ScyllaDB node by labelling its member Service, causing the Operator to provision a fresh node in its place.

## When to replace a node

Replace a node when it is permanently unavailable — the Kubernetes node has been lost, the underlying disk has failed, or the ScyllaDB process is unable to start.
Replacement streams data from other replicas to a new pod, restoring the cluster to full health.

:::{note}
Replacement is for **permanently failed** nodes.
If a node is temporarily down (for example, during a network partition or host reboot), wait for it to come back.
Replacing a node that is still alive causes two nodes to own the same token range until the situation is resolved.
:::

## How it works

1. You apply the `scylla/replace=""` label to the **member Service** of the failed node.
2. The Operator records the old node's Host ID, deletes the PVC and evicts the pod.
3. The StatefulSet controller creates a new pod with a fresh PVC.
4. ScyllaDB starts on the new pod with the `--replace-node-first-boot` flag referencing the old Host ID.
5. The new node joins the cluster, takes ownership of the old node's token range, and streams data from other replicas.
6. Once the new node is Ready, the Operator removes the replace labels from the Service.

:::{note}
The Operator uses **Host ID–based replacement** (available since ScyllaDB OS 5.2 and ScyllaDB Enterprise 2023.1).
Older replacement via `replace_address_first_boot` is deprecated.
:::

## Automatic orphaned node replacement

When a Kubernetes node is permanently removed (for example, a node pool scale-down or a cloud instance termination), the PersistentVolume bound to the ScyllaDB pod becomes orphaned — it references a node that no longer exists.

The Operator's orphaned PV controller detects this condition and automatically applies the `scylla/replace=""` label on the affected Service, triggering replacement without manual intervention.

To disable this behaviour, set `automaticOrphanedNodeCleanup: false` in the ScyllaCluster spec, or `disableAutomaticOrphanedNodeReplacement: true` in the ScyllaDBCluster / ScyllaDBDatacenter spec.

## Replace a dead node in a ScyllaCluster

### Step 1: Identify the failed node

Run `nodetool status` from a healthy node and look for status `DN` (Down and Normal):

```bash
kubectl -n scylla exec scylladb-us-east-1a-0 -c scylla -- nodetool status
```

```
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load     Tokens Owns Host ID                              Rack
UN 10.43.125.110  74.63 KB 256    ?    8ebd6114-969c-44af-a978-87a4a6c65c3e us-east-1a
UN 10.43.231.189  91.03 KB 256    ?    35d0cb19-35ef-482b-92a4-b63eee4527e5 us-east-1a
DN 10.43.43.51    74.77 KB 256    ?    1ffa7a82-c41c-4706-8f5f-4d45a39c7003 us-east-1a
```

### Step 2: Find the corresponding Service

Match the IP address of the `DN` node to its member Service:

```bash
kubectl -n scylla get svc -l scylla/cluster=scylladb -o wide
```

Identify the Service with the matching ClusterIP (in this example, `10.43.43.51` corresponds to `scylladb-us-east-1a-2`).

### Step 3: Drain the Kubernetes node (if still accessible)

If the failed Kubernetes node is still present in the cluster, drain it to release any remaining resources:

```bash
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
```

The ScyllaDB pod should enter `Pending` state after the drain.

:::{note}
If the Kubernetes node has already been removed (for example, a terminated cloud instance), skip this step.
:::

### Step 4: Trigger the replacement

Apply the replace label to the member Service:

```bash
kubectl -n scylla label svc scylladb-us-east-1a-2 scylla/replace=""
```

The Operator deletes the PVC and pod, then the StatefulSet recreates the pod on an available node.
The new node starts with the replace flag and begins streaming data from other replicas.

:::{caution}
Replace **one node at a time**.
Wait for each replacement to complete before starting the next.
:::

### Step 5: Wait for the replacement to complete

Monitor the pod status:

```bash
kubectl -n scylla get pods -w
```

The new pod initially shows fewer ready containers while ScyllaDB bootstraps and streams data.
Once streaming completes, the pod becomes fully Ready.

Wait for the cluster conditions:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

### Step 6: Verify and repair

Confirm all nodes report `UN`:

```bash
kubectl -n scylla exec scylladb-us-east-1a-0 -c scylla -- nodetool status
```

Run a repair to ensure data consistency:

```bash
kubectl -n scylla exec scylladb-us-east-1a-0 -c scylla -- nodetool repair
```

Or use ScyllaDB Manager scheduled repair tasks for automated repair.

## Replace a dead node in a ScyllaDBCluster

The procedure is the same — apply the `scylla/replace=""` label to the member Service in the **worker cluster** where the failed node is located.

1. Connect to the worker cluster context:
   ```bash
   kubectl --context="${WORKER_CONTEXT}" -n scylla get svc -l scylla/cluster=dev-cluster -o wide
   ```

2. Apply the replace label:
   ```bash
   kubectl --context="${WORKER_CONTEXT}" -n scylla label svc dev-cluster-us-east-1a-2 scylla/replace=""
   ```

3. Wait for the rollout on the control plane cluster:
   ```bash
   kubectl --context="${CONTROL_PLANE_CONTEXT}" -n scylla wait --timeout=30m --for='condition=Progressing=False' scylladbcluster.scylla.scylladb.com/dev-cluster
   kubectl --context="${CONTROL_PLANE_CONTEXT}" -n scylla wait --timeout=30m --for='condition=Available=True' scylladbcluster.scylla.scylladb.com/dev-cluster
   ```

## When replacement fails

If the replacement gets stuck — for example, the new pod enters `CrashLoopBackOff` or streaming cannot complete — see [Recovering from a failed replace](../troubleshoot/recover-from-failed-replace.md) for a step-by-step fallback procedure.

## Key considerations

| Consideration | Detail |
|---|---|
| Data streaming | Replacement streams data from other replicas. Duration depends on data size and network bandwidth. |
| One at a time | Replace one node at a time. Concurrent replacements risk exhausting cluster resources and violating quorum. |
| Repair after replace | Always repair after replacement to fix any inconsistencies from the streaming process. |
| PVC deletion | The Operator deletes the PVC during replacement. All local data on the failed node is discarded. |
| Host ID preservation | The new node inherits the old node's token range but gets a **new** Host ID. The old Host ID is removed from the ring. |
| Replication factor | Ensure your replication factor is at least 2 (ideally 3) so that data is available from other replicas during streaming. |

## Related pages

- [Scaling](scale-cluster.md) — adding or removing nodes without replacement
- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — pod identity and ordinal management
- [Recovering from a failed replace](../troubleshoot/recover-from-failed-replace.md) — fallback when replacement is stuck
- [Rolling restart](perform-rolling-restart.md) — restarting nodes without replacement
