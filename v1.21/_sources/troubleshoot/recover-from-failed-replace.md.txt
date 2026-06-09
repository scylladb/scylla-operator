# Recover from a failed node replace

Step-by-step procedure to recover when a node replace operation fails and leaves ScyllaDB Operator stuck.

:::{warning}
This procedure involves manual cluster-state manipulation and can cause data loss on the affected node.
Follow each step carefully and collect a must-gather archive before making any destructive changes.
:::

## When to use this procedure

Use this guide when:

- A node replace is stuck — the replacement pod is in `CrashLoopBackOff` or stuck joining.
- `nodetool status` shows a node with status `DN` or `?N` that was being replaced.
- ScyllaDB Operator cannot apply further configuration changes because the rollout is blocked.

This guide assumes that all other nodes in the cluster are healthy (`UN`).

For normal node replacement, see [Replace nodes](../operate/replace-nodes.md).

## Prerequisites

- `kubectl` access to the cluster.
- Ability to scale the ScyllaDB Operator Deployment.
- A recent backup — this is a dangerous operation and a data backup is recommended before proceeding.

## Verify the failure

Run `nodetool status` on a healthy node to confirm the stuck state:

```console
$ kubectl -n <namespace> exec <healthy-pod> -c scylla -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
DN 10.152.183.214 466.12 KB 256    ?    09d815de-6f6d-4394-8439-bd8d34231835 examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

Ensure that the culprit entry matches either the old (replaced) node's Host ID or the new (attempting to replace) node's Host ID. Check the logs of the failing pod to determine which one it is.

## Collect a must-gather archive

Before making any changes, capture the current state by collecting a must-gather archive.
See [Collect debugging information](collect-debugging-information/index.md) for instructions.

## Pause ScyllaDB Operator

Note the current replica count, then scale ScyllaDB Operator to zero to prevent reconciliation while you perform manual recovery:

```bash
# Note the current replicas (typically 2)
kubectl -n scylla-operator get deploy scylla-operator -o jsonpath='{.spec.replicas}'

# Scale to zero
kubectl -n scylla-operator scale deploy/scylla-operator --replicas=0 --timeout=5m
```

Wait for ScyllaDB Operator pods to terminate:

```bash
kubectl -n scylla-operator get pods -w
```

## Orphan-delete the rack StatefulSet

Delete the StatefulSet for the affected rack **without deleting its pods**.
This prevents the StatefulSet from recreating the culprit pod when you delete it in the next step.
ScyllaDB Operator will recreate the StatefulSet in its exact form when it resumes.

```bash
kubectl -n <namespace> delete statefulset <cluster>-<dc>-<rack> --cascade=orphan
```

:::{warning}
You **must** use `--cascade=orphan`.
Without this flag, `kubectl delete statefulset` also deletes all pods managed by the StatefulSet, causing unavailability for that rack.
Deletion of a StatefulSet does not delete associated PVCs, so even if you accidentally omit `--cascade=orphan`, data is preserved on the PVCs and Operator will recover the pods when it resumes.
:::

The existing pods continue running.

## Identify Host IDs to remove

Run `nodetool status` and note the Host IDs of:
- The **culprit node** (the failed replacement).
- Any **ghost members** — nodes that appear in the ring but have no corresponding running pod.

```bash
kubectl -n <namespace> exec <healthy-pod> -c scylla -- nodetool status
```

## Stop the culprit node

Delete the pod, PVC, and Service for the failed node:

```bash
# Stop the node that is failing to join the cluster.
# The StatefulSet does not exist, so it will not recreate the pod.
kubectl -n <namespace> delete pod <culprit-pod>

# WARNING: This causes data loss for this node.
# Delete the PersistentVolumeClaim for the data volume held by the culprit node.
kubectl -n <namespace> delete pvc <culprit-pvc>

# Delete the per-node Service.
# ScyllaDB Operator interprets this as a need to provision a brand-new node
# instead of attempting to replace the old one in the rack.
kubectl -n <namespace> delete service <culprit-service>
```

:::{caution}
Deleting the PVC permanently destroys the data on that node.
The data must be replicated on other nodes and will be recovered via streaming when a new node joins.
:::

## Remove ghost members

For each ghost Host ID, run `nodetool removenode` from a healthy node:

```bash
kubectl -n <namespace> exec <healthy-pod> -c scylla -- nodetool removenode <host-id>
```

Repeat as necessary for each ghost member.

Verify that only healthy nodes remain:

```console
$ kubectl -n <namespace> exec <healthy-pod> -c scylla -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

All remaining nodes should show `UN`.

## Resume ScyllaDB Operator

Scale ScyllaDB Operator back to its original replica count:

```bash
# Replace N with the number of replicas noted earlier (typically 2).
kubectl -n scylla-operator scale deploy/scylla-operator --replicas=<N> --timeout=5m
```

ScyllaDB Operator will:
1. Recreate the (identical) StatefulSet for the rack.
2. Recreate the per-node Service.
3. Create a new Pod and PVC for the removed node.
4. The new node joins the cluster as a fresh member and streams data from peers.

## Verify recovery

Wait for the new pod to become ready:

```bash
kubectl -n <namespace> get pods -w
```

Verify cluster health:

```console
$ kubectl -n <namespace> exec <healthy-pod> -c scylla -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
UN 10.152.183.234 493.66 KB 256    ?    NEW-UUID-DIFFERENT-THAN-BEFORE-abcde examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

All nodes should show `UN`. Note that the recovered node has a new Host ID.

## Multi-datacenter note

In a multi-datacenter deployment using multiple `ScyllaCluster` resources, perform this procedure in the Kubernetes cluster hosting the failed node's datacenter.

## What if something goes wrong

| Situation | Recovery |
|---|---|
| Accidentally deleted StatefulSet without `--cascade=orphan` | Pods are deleted (causing rack unavailability), but PVCs survive. When ScyllaDB Operator resumes and recreates the StatefulSet, new pods are created and reattach to existing PVCs. Data is preserved. |
| ScyllaDB Operator fails to recreate the StatefulSet | Manually recreate it from the must-gather archive (the StatefulSet YAML is captured). |
| New node fails to join | Repeat the procedure — verify that all ghost members were removed. Check seed configuration and network connectivity. |

## Related pages

- [Replace nodes](../operate/replace-nodes.md)
- [StatefulSets and racks](../understand/statefulsets-and-racks.md)
- [Collect debugging information](collect-debugging-information/index.md)
