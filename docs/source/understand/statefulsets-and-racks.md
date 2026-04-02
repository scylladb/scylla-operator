# StatefulSets and racks

This page explains how ScyllaDB Operator maps ScyllaDB topology onto Kubernetes primitives, and how that mapping affects scaling, rolling updates, node replacement, and day-to-day operations.

## One StatefulSet per rack

Each **rack** defined in a `ScyllaCluster` is backed by exactly one Kubernetes **StatefulSet**. The StatefulSet is named using the pattern:

```
<cluster-name>-<datacenter-name>-<rack-name>
```

For example, a ScyllaCluster named `scylla` with datacenter `us-east-1` and rack `a` produces a StatefulSet named `scylla-us-east-1-a`.

Each pod in the StatefulSet is named with an ordinal suffix:

```
scylla-us-east-1-a-0
scylla-us-east-1-a-1
scylla-us-east-1-a-2
```

The `members` field in the rack spec maps directly to the StatefulSet's `replicas` count.

## Pod identity and ordering

StatefulSets provide two guarantees that are essential for running ScyllaDB:

1. **Stable network identity** — each pod gets a predictable DNS name (`<pod>.<headless-service>.<namespace>.svc.cluster.local`) and a dedicated member Service. The network identity survives pod restarts because the pod name (and therefore the Service name) is derived from the StatefulSet name plus a fixed ordinal.

2. **Stable persistent storage** — each pod's PersistentVolumeClaim (PVC) is named deterministically (`data-<pod-name>`) and is **not** deleted when the pod is deleted or the StatefulSet is scaled down. When a pod comes back (restart or scale-up), it reattaches to the same PVC and recovers its data.

The Operator uses `OrderedReady` pod management policy. This means pods are created in order (0, 1, 2, …) and each pod must become Ready before the next one is created. On shutdown, pods are terminated in reverse order.

## Scaling

### Scaling up

When you increase the `members` count of a rack, the Operator increases the StatefulSet's `replicas`. Kubernetes creates new pods at the **end** of the ordinal sequence. For example, if the rack has 3 members (ordinals 0, 1, 2) and you scale to 5, pods with ordinals 3 and 4 are created.

Each new pod joins the ScyllaDB cluster as a new node, takes ownership of a portion of the token range, and begins streaming data from existing nodes.

### Scaling down

When you decrease the `members` count, the Operator removes nodes from the **end** of the ordinal sequence — always the highest ordinal first. Before the pod is deleted:

1. The Operator sets the `scylla/decommissioned` label on the member Service to `"false"`, recording the intent to decommission.
2. The sidecar running inside the pod detects this label and initiates `nodetool decommission`, which redistributes the node's token ranges and streams its data to other nodes.
3. Once decommission completes, the sidecar updates the label to `"true"`.
4. The Operator scales the StatefulSet's `replicas` down by one, which deletes the pod.

The Operator scales down **one member at a time**, regardless of how large the requested scale-down is. This ensures that only one decommission is in progress at any moment, protecting data availability.

:::{important}
You cannot remove an arbitrary node from the middle of a rack's ordinal range by changing the `members` count. Scaling always operates on the tail of the StatefulSet. To remove a specific node (for example, ordinal 1 out of 0, 1, 2), use node replacement instead — see [Replacing nodes](../operate/replace-nodes.md).
:::

### PVC retention

PVCs are **not** deleted when a StatefulSet is scaled down. The PVCs for decommissioned pods remain in the namespace until manually deleted. If you scale the rack back up to the same size, the new pods reuse the existing PVCs and their data.

:::{caution}
After a scale-down followed by a scale-up, the reattached PVC contains stale data from the decommissioned node. ScyllaDB handles this correctly — the new node joins as a replacement and the stale data is cleaned up — but be aware that the PVC is not wiped automatically.
:::

## Rolling updates

When the pod template changes (for example, a ScyllaDB image change or a configuration update), the Operator rolls out the change across all racks.

### Non-upgrade changes

For changes that do not involve a ScyllaDB major or minor version change, the Operator applies the updated StatefulSet spec directly. Kubernetes uses the `RollingUpdate` strategy to restart pods one at a time in **reverse ordinal order** (highest ordinal first). The StatefulSet controller waits for each restarted pod to become Ready before proceeding to the next one.

A stuck pod (one that does not become Ready after the update) **blocks the entire rollout**. No further pods in the rack are updated until the stuck pod recovers. This is a safety mechanism — it prevents a bad configuration from cascading across the entire rack.

### Version upgrades

When the ScyllaDB image version changes by major or minor version, the Operator runs a more controlled upgrade process using **partition-based rollouts**:

1. **Partition all StatefulSets** — set each StatefulSet's `updateStrategy.rollingUpdate.partition` to its current replica count. This applies the new pod template to the StatefulSet without restarting any pods.
2. **Run pre-upgrade hooks** — take system and data snapshots on each node for rollback safety.
3. **Decrement the partition one at a time** — lowering the partition from N to N-1 allows exactly one pod (ordinal N-1) to pick up the new template and restart. The Operator waits for the restarted pod to become Ready, then runs post-node-upgrade hooks (e.g., `nodetool upgradesstables`) before moving to the next pod.
4. **One rack at a time** — the partition can only advance in one rack per reconciliation cycle.
5. **Run post-upgrade hooks** — after all racks have completed the rollout, the Operator cleans up the upgrade context.

This process ensures that only one ScyllaDB node is restarting at any given time and that each node is verified healthy before the next one is updated.

## Operating on a node in the middle

StatefulSets are an ordered sequence. The Operator provides mechanisms for operating on a specific node regardless of its position:

### Node replacement

To replace a node at any ordinal (not just the tail), set the `scylla/replace` label on its member Service. The Operator deletes the pod (and optionally the PVC), then creates a new pod at the same ordinal. The new pod starts ScyllaDB with the `--replace-node-first-boot` flag, which tells ScyllaDB to take over the dead node's token ranges.

Replacement works on any ordinal because the pod identity and PVC are tied to the ordinal — the StatefulSet recreates a pod with the same name at the same position.

### Maintenance mode

Setting the `scylla/node-maintenance` label on a member Service puts that node into maintenance mode. The Operator reconfigures the Service so that traffic is no longer routed to the node, allowing you to perform manual operations on it without affecting client traffic. See [Maintenance mode](../operate/use-maintenance-mode.md).

## Common pitfalls

### Stuck rollout

If a pod fails to become Ready after a rolling update, the rollout stalls. No other pods in the rack are updated. Symptoms:

- `kubectl get statefulset` shows `READY` count lower than `REPLICAS`.
- The `ScyllaCluster` status condition reports `WaitingForStatefulSetRollout`.

To diagnose, check the logs of the failing pod and its events. If the problem is with the new configuration, fix the spec and the rollout resumes automatically. If the pod is stuck for other reasons (crash loop, resource issues), see [Node not starting](../troubleshoot/diagnose-node-not-starting.md).

For log-level changes on a stuck cluster where a rolling restart is not feasible, see [Changing log level](../troubleshoot/change-log-level.md).

### Deleting a StatefulSet

Deleting a StatefulSet with the default cascade policy also deletes all its pods (but not PVCs). If the Operator is running, it immediately recreates the StatefulSet, which recreates the pods. Using `--cascade=orphan` preserves the running pods — this is used during advanced recovery procedures such as [Recovering from a failed replace](../troubleshoot/recover-from-failed-replace.md).

## Related pages

- [Pod disruption budgets](pod-disruption-budgets.md) — how PDBs interact with rolling updates and scale-down.
- [Sidecar](sidecar.md) — what runs inside each pod and how decommission is driven.
- [Scaling](../operate/scale-cluster.md) — how-to guide for scaling operations.
- [Replacing nodes](../operate/replace-nodes.md) — how-to guide for node replacement.
- [Changing log level](../troubleshoot/change-log-level.md) — emergency changes when a rollout is stuck.
