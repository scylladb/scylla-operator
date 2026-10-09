# Recover from a stuck rollout

Procedure to recover a rack whose nodes can't start with a change made to the `ScyllaCluster` or `ScyllaDBDatacenter`, and whose StatefulSet doesn't pick up the fixed or reverted spec.

## When to use this procedure

Use this guide when all of the following hold:

- The `ScyllaCluster` or `ScyllaDBDatacenter` reports `Progressing=True` with the reason `WaitingForStatefulSetRollout` for a long time.
  After ten minutes the condition message says that the rollout hasn't progressed and links this page.
- A Pod of the rack stays unready after a change to the spec: it is `Running` but never becomes `Ready`, or it stays `Pending`.
- You have reverted or fixed the spec, but the rack's StatefulSet still has the old Pod template, for example a lower CPU limit than the one in the spec.

Typical changes that leave a node unable to start are a lower CPU limit on a rack whose nodes hold tablet-based tables (ScyllaDB can't reduce the number of shards of such a node and refuses to start), resources that no Kubernetes node can satisfy, or a ScyllaDB configuration the node rejects.
Check the ScyllaDB log of the unready Pod for the reason:

```shell
kubectl -n <namespace> logs <pod-name> -c scylla | grep -i 'startup failed'
```

## Cause

ScyllaDB Operator applies one change at a time and waits for the rack's StatefulSet to roll out before it applies the next one.
A rollout to a Pod template the nodes can't start with never finishes, so the fixed or reverted spec never reaches the StatefulSet, and the broken Pod is recreated with the same template every time it restarts.

## Recover

1. Fix or revert the spec of the `ScyllaCluster` or `ScyllaDBDatacenter` first. The StatefulSet is recreated from the spec in the next step, so a spec the nodes can't start with brings the same Pod back.

2. Delete the rack's StatefulSet without deleting its Pods:

   ```shell
   kubectl -n <namespace> delete statefulset <cluster-name>-<datacenter-name>-<rack-name> --cascade=orphan
   ```

   ScyllaDB Operator recreates the StatefulSet from the spec right away and the StatefulSet controller adopts the Pods of the rack. The Pods that run with the correct template keep running.

3. With [parallel node operations](../operate/scale-add-remove-racks.md#sequential-and-parallel-node-operations) enabled, the StatefulSet controller replaces the broken Pod on its own, and you are done.
   With parallel node operations disabled, the StatefulSet controller never replaces a Pod that isn't ready, so delete the broken Pod:

   ```shell
   kubectl -n <namespace> delete pod <pod-name>
   ```

   The Pod is recreated with the correct template. Its data volume is kept.

4. Wait for the `ScyllaCluster` or `ScyllaDBDatacenter` to report `Progressing=False` and `Available=True`.

Don't delete the StatefulSet with foreground or background cascading deletion: that deletes every Pod of the rack, and the whole rack restarts instead of the broken node.

If a ScyllaDB version upgrade is in progress, or a node of the rack is bootstrapping, leaving the cluster, in maintenance mode, or being replaced, let that operation finish before you delete the StatefulSet.
The condition message doesn't link this page in those cases, as a Pod can stay unready for long legitimately.

## Prevent it

ScyllaDB Operator rejects a decrease of the CPU limit of the ScyllaDB container of a rack, which is the most common cause.
To move a rack to nodes with fewer CPUs, [add a rack](../operate/scale-add-remove-racks.md) with the smaller nodes and scale the old one down.
If you know the nodes of the rack hold no tablet-based tables, you can force the change by annotating the `ScyllaCluster` or `ScyllaDBDatacenter` with `scylla-operator.scylladb.com/force-scylladb-cpu-decrease: "true"`.
Remove the annotation once the change is applied, so that a later decrease is checked again.
