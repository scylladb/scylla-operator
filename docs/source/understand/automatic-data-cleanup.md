# Automatic data cleanup

This page explains why ScyllaDB Operator runs automatic data cleanup after scaling operations and how the mechanism works.

## Why cleanup is needed

When a ScyllaDB cluster scales horizontally (nodes are added or removed), the ownership of data tokens changes. Nodes that lose ownership of certain token ranges still hold the corresponding data on disk. This stale data must be removed to:

1. **Reclaim storage** — stale data wastes disk space unnecessarily.
2. **Prevent data resurrection** — if stale data is not removed, it can reappear during repair or read operations, overriding newer deletions.

ScyllaDB handles cleanup automatically for keyspaces that use tablets. However, system keyspaces and standard vnode-based keyspaces are not covered by this automatic mechanism. ScyllaDB Operator fills the gap by triggering cleanup for these remaining keyspaces.

## Trigger mechanism

The Operator tracks the token ring of each ScyllaDB cluster. When the ring changes — because a node was added, removed, or replaced — the Operator compares the current ring state against the last state for which cleanup was completed. If they differ, cleanup Jobs are created for all nodes that were affected by the token redistribution.

Before creating any Jobs, the Operator waits for the cluster to reach a stable state:

- `StatefulSetControllerProgressing` is `False`.
- `Available` is `True`.
- `Degraded` is `False`.

This ensures that cleanup runs only after the scaling operation has fully completed and the cluster is healthy.

### What triggers cleanup

- **Scale-out** — after a new node finishes bootstrapping. Cleanup runs on all nodes except the newly added node (which did not lose any tokens).
- **Scale-in (decommission)** — after a node is removed. The remaining nodes inherit its tokens but technically do not need cleanup (they did not lose tokens). The Operator still triggers cleanup because the token ring changed. This is safe but may cause a brief I/O spike.
- **Initial cluster bootstrap** — nodes are added one by one, and the token ring changes with each addition. Cleanup runs after each node join, but the Jobs may complete quickly on an empty dataset.

## Cleanup Job details

The Operator creates one Kubernetes `Job` per affected node. Each Job runs the `scylla-operator cleanup-job` subcommand, which connects to the ScyllaDB Manager Agent on the target node and requests a cleanup operation through the Manager API. The Job pod authenticates using a Manager auth token mounted from a Secret.

When a cleanup Job completes successfully, the Operator deletes it. If a Job is still running, the `ScyllaCluster` status shows the `JobControllerProgressing` condition set to `True` with a message listing the active Job names.

## Inspecting cleanup status

Check whether cleanup is in progress:

```bash
kubectl get scyllacluster <name> -o jsonpath='{.status.conditions[?(@.type=="JobControllerProgressing")]}' | jq
```

When no cleanup Jobs are running:

```json
{
  "status": "False",
  "type": "JobControllerProgressing",
  "reason": "AsExpected"
}
```

Cleanup Jobs may complete and be deleted before you can observe them. To verify they ran, check Kubernetes events:

```bash
kubectl get events --field-selector reason=JobCreated
```

Events are emitted for Job creation, pod creation, completion, and deletion.

## Known limitations

### Replication factor changes are not detected

Decreasing the replication factor of a keyspace does not change the token ring — the same nodes own the same token ranges, but fewer replicas are needed. The Operator does not detect this and does not trigger cleanup. Run cleanup manually:

```bash
kubectl exec -it service/<cluster-name>-client -c scylla -- nodetool cleanup
```

### Unnecessary cleanup on decommission

When a node is decommissioned, the remaining nodes inherit its tokens. They do not lose any tokens and therefore do not strictly need cleanup. The Operator triggers cleanup anyway because the token ring changed. The operation is safe but adds temporary I/O load.

## Related pages

- [Overview](overview.md) — component diagram and reconciliation model.
- [Sidecar](sidecar.md) — the sidecar that reports node status used for stability checks.
