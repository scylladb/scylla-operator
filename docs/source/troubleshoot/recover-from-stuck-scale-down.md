# Recover from a stuck scale-down

Procedure to recover when a scale-down with parallel node operations disabled leaves the Pods of the removed nodes behind.

## When to use this procedure

Use this guide when all of the following hold:

- [Parallel node operations](../operate/scale-add-remove-racks.md#sequential-and-parallel-node-operations) are disabled.
- You scaled a rack down by two or more nodes.
- Pods whose ordinal is equal to or higher than the number of members of the rack keep running unready, and their member Services are removed.
- The `ScyllaCluster` reports `Progressing=True` with the reason `WaitingForStatefulSetRollout` indefinitely.

## Cause

With parallel node operations disabled, each rack's StatefulSet uses the [`OrderedReady` Pod management policy](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#orderedready-pod-management).
Under it, the StatefulSet controller deletes an unready Pod only when no Pod with a lower ordinal is unready.
The Pods of the nodes that have left the cluster never become ready again.
When a Pod that stays in the rack isn't ready while the nodes leave, for example because its node is in [maintenance mode](../operate/use-maintenance-mode.md), the next node leaves before the Pod with the highest ordinal is deleted.
From then on, each leftover Pod blocks the deletion of the leftover Pods above it, so making the Pod that stays in the rack ready again doesn't help.

## Recover

Make sure the Pods that stay in the rack are ready, for example by taking their nodes out of maintenance mode.
Then delete the leftover Pod with the highest ordinal:

```shell
kubectl -n <namespace> delete pod <pod-name>
```

The StatefulSet controller then deletes the remaining leftover Pods, and the scale-down finishes.
Deleting these Pods is safe, because their nodes have already left the cluster.

Enabling parallel node operations on a cluster in this state doesn't recover it, because the Operator applies the change only once the scale-down finishes.

## Prevent it

[Enable parallel node operations](../operate/scale-add-remove-racks.md#configure-parallel-node-operations) before you scale down.
