# Recover from a failed node replace

Step-by-step procedure to recover when a node replace operation fails and leaves the Operator stuck.

:::{warning}
This procedure involves manual cluster-state manipulation and can cause data loss on the affected node.
Follow each step carefully and collect a must-gather archive before making any destructive changes.
:::

## When to use this procedure

Use this guide when:

- A node replace is stuck — the replacement pod is in `CrashLoopBackOff` or stuck joining.
- `nodetool status` shows a node with status `DN` or `?N` that was being replaced.
- The Operator cannot apply further configuration changes because the rollout is blocked.

For normal node replacement, see [Replace nodes](../operate/replace-nodes.md).

## Prerequisites

- `kubectl` access to the cluster.
- Ability to scale the Operator deployment.
- A recent backup ([Back up and restore](../operate/back-up-and-restore.md)).

## Step 1: Verify the failure

Run `nodetool status` on a healthy node to confirm the stuck state:

```bash
kubectl -n scylla exec -it <healthy-pod> -c scylla -- nodetool status
```

Look for nodes with status other than `UN` — for example `DN` (Down/Normal) or nodes with `?` status.
Note the **Host ID** of the problematic node.

## Step 2: Collect a must-gather archive

Before making any changes, capture the current state:

```bash
docker run -it --pull=always --rm \
  -v="${KUBECONFIG}:/kubeconfig:ro" \
  -v="$(pwd):/workspace" \
  --workdir=/workspace \
  docker.io/scylladb/scylla-operator:latest \
  must-gather --kubeconfig=/kubeconfig
```

See [must-gather](collect-debugging-information/must-gather.md) for details.

## Step 3: Pause the Operator

Note the current replica count, then scale the Operator to zero:

```bash
# Note the current replicas
kubectl -n scylla-operator get deploy scylla-operator -o jsonpath='{.spec.replicas}'

# Scale to zero
kubectl -n scylla-operator scale deploy scylla-operator --replicas=0
```

Wait for the Operator pods to terminate:

```bash
kubectl -n scylla-operator get pods -w
```

## Step 4: Orphan-delete the rack StatefulSet

Delete the StatefulSet for the affected rack **without deleting its pods**:

```bash
kubectl -n scylla delete statefulset <cluster-name>-<dc>-<rack> --cascade=orphan
```

:::{warning}
You **must** use `--cascade=orphan`.
Without this flag, `kubectl delete statefulset` also deletes all pods managed by the StatefulSet, causing a full outage for that rack.
:::

The existing pods continue running.
The Operator will recreate the StatefulSet when it resumes.

## Step 5: Identify Host IDs to remove

Run `nodetool status` again and note the Host IDs of:
- The **culprit node** (the failed replacement).
- Any **ghost members** — nodes that appear in the ring but have no corresponding running pod.

```bash
kubectl -n scylla exec -it <healthy-pod> -c scylla -- nodetool status
```

Cross-reference with the [ScyllaDB guide for handling failed membership changes](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html).

## Step 6: Stop the culprit node

Delete the pod, PVC, and Service for the failed node:

```bash
# Delete the pod
kubectl -n scylla delete pod <culprit-pod-name>

# Delete the PVC (data loss for this node)
kubectl -n scylla delete pvc <culprit-pvc-name>

# Delete the per-node Service
# This signals the Operator to provision a brand-new node instead of retrying the replace
kubectl -n scylla delete svc <culprit-service-name>
```

:::{caution}
Deleting the PVC permanently destroys the data on that node.
The data must be replicated on other nodes and will be recovered via streaming when a new node joins.
:::

## Step 7: Remove ghost members

For each ghost Host ID, run `nodetool removenode` from a healthy node:

```bash
kubectl -n scylla exec -it <healthy-pod> -c scylla -- \
  nodetool removenode <ghost-host-id>
```

Verify that only healthy nodes remain:

```bash
kubectl -n scylla exec -it <healthy-pod> -c scylla -- nodetool status
```

All remaining nodes should show `UN`.

## Step 8: Resume the Operator

Scale the Operator back to its original replica count:

```bash
kubectl -n scylla-operator scale deploy scylla-operator --replicas=<original-count>
```

The Operator will:
1. Recreate the rack StatefulSet.
2. Recreate the per-node Service.
3. Create a new pod and PVC for the removed node.
4. The new node joins the cluster as a fresh member and streams data from peers.

## Step 9: Verify recovery

Wait for the new pod to become ready:

```bash
kubectl -n scylla get pods -w
```

Verify cluster health:

```bash
kubectl -n scylla exec -it <healthy-pod> -c scylla -- nodetool status
```

All nodes should show `UN`.

After recovery, run a repair to ensure data consistency:

```bash
# If using ScyllaDB Manager, trigger an ad-hoc repair
kubectl -n scylla get scylladbmanagertask
```

## Multi-datacenter note

In a multi-datacenter deployment using multiple `ScyllaCluster` resources, perform this procedure in the Kubernetes cluster hosting the failed node's datacenter.

## What if something goes wrong

| Situation | Recovery |
|---|---|
| Accidentally deleted StatefulSet without `--cascade=orphan` | PVCs survive the deletion. When the Operator resumes and recreates the StatefulSet, new pods are created and reattach to existing PVCs. Data is preserved. |
| Operator fails to recreate the StatefulSet | Manually recreate it from the must-gather archive (the StatefulSet YAML is captured). |
| New node fails to join | Repeat the procedure — verify that all ghost members were removed. Check seed configuration and network connectivity. |

## Related pages

- [Replace nodes](../operate/replace-nodes.md)
- [StatefulSets and racks](../understand/statefulsets-and-racks.md)
- [Collect debugging information](collect-debugging-information/index.md)
- [Diagnostic flowchart](diagnostic-flowchart.md)
