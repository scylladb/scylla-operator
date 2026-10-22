# Recover from a stuck config change

Use this procedure when a change to a `ScyllaCluster` leaves a ScyllaDB node unable to start, and fixing or reverting the change doesn't unblock the rack.

## When to use this procedure

Use this guide when all of the following hold:

- After a spec change, a Pod of the rack never becomes ready: it is `Running` but not `Ready`, or it stays `Pending`.
- You have fixed or reverted the change, but the Pod still runs with the old Pod template.
- The `Progressing` condition stays `True` and its message names the rack's StatefulSet:

  ```shell
  NAMESPACE=scylla
  CLUSTER=my-cluster
  kubectl -n "${NAMESPACE}" get scyllacluster "${CLUSTER}" -o jsonpath='{.status.conditions[?(@.type=="Progressing")].message}'
  ```

  ```console
  Progressing: StatefulSetControllerProgressing: StatefulSetControllerProgressing: Waiting for StatefulSet "scylla/my-cluster-us-east-1-us-east-1a" to roll out.
  Progressing: JobControllerProgressing: JobControllerProgressing: Waiting for StatefulSet controller to finish progressing with ScyllaCluster "scylla/my-cluster"
  Progressing: JobControllerProgressing: JobControllerProgressing: Waiting for ScyllaCluster "scylla/my-cluster" nodes to be ready
  ```

This procedure doesn't apply during a ScyllaDB version upgrade. Recreating the StatefulSet would make it roll the remaining nodes without the per-node upgrade steps of ScyllaDB Operator.

To find out why the node can't start, list the Pods of the cluster and find the one with fewer ready containers than the others:

```shell
kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER}"
```

```console
NAME                                READY   STATUS    RESTARTS      AGE
my-cluster-us-east-1-us-east-1a-0   5/5     Running   0             2d
my-cluster-us-east-1-us-east-1a-1   5/5     Running   0             2d
my-cluster-us-east-1-us-east-1a-2   4/5     Running   3 (2m ago)    15m
```

Set its name and check its ScyllaDB log:

```shell
POD=my-cluster-us-east-1-us-east-1a-2
kubectl -n "${NAMESPACE}" logs "${POD}" -c scylla | grep -i 'startup failed'
```

For a `Pending` Pod, check the events at the end of `kubectl -n "${NAMESPACE}" describe pod "${POD}"` instead.
Examples of such changes are resources that no Kubernetes node can satisfy, and a ScyllaDB configuration the node rejects.

## Cause

ScyllaDB Operator applies a spec change to one rack at a time, and waits for the rack's StatefulSet to finish rolling out before it applies the next change.
This keeps at most one node of the datacenter down at any time, so that the data stays available throughout the change.
A change to a Pod template the node can't start with never finishes rolling out, so your fix never reaches the StatefulSet.
Deleting the broken Pod alone doesn't help: the StatefulSet recreates it from the same broken template.

## Recover

1. Make sure the spec contains the fix or the revert.
   In the next step the StatefulSet is recreated from the spec, so a broken spec brings the broken Pod back.

2. Set the name of the StatefulSet from the `Progressing` message and delete it, without deleting its Pods:

   ```shell
   STATEFULSET=my-cluster-us-east-1-us-east-1a
   kubectl -n "${NAMESPACE}" delete statefulset "${STATEFULSET}" --cascade=orphan
   ```

   :::{warning}
   Don't leave out `--cascade=orphan`.
   Without it, Kubernetes deletes every Pod of the rack, and the whole rack restarts.
   :::

   ScyllaDB Operator recreates the StatefulSet from the spec within seconds, and the StatefulSet adopts the running Pods.
   Pods that already match the spec keep running.

3. Replace the broken Pod:

   - With [parallel node operations](../operate/scale-add-remove-racks.md#sequential-and-parallel-node-operations) enabled, the StatefulSet replaces it on its own within a few minutes. Skip to the next step.
   - With parallel node operations disabled, the StatefulSet never replaces a Pod that isn't ready, so delete it yourself:

     ```shell
     kubectl -n "${NAMESPACE}" delete pod "${POD}"
     ```

   The new Pod keeps the node's data volume.

4. Wait for the change to finish rolling out:

   ```shell
   kubectl -n "${NAMESPACE}" wait --for='condition=Progressing=False' --timeout=10m "scyllacluster/${CLUSTER}"
   kubectl -n "${NAMESPACE}" wait --for='condition=Available=True' --timeout=10m "scyllacluster/${CLUSTER}"
   ```
