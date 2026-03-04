# Migrate a rack to a new node pool

Move a ScyllaDB rack from one Kubernetes node pool to another without downtime, by adding a new rack on the target node pool and gradually migrating data away from the old rack.

## When to use this procedure

Common reasons to migrate a rack to a new node pool:

- Upgrading to a different instance type (e.g. larger machines, newer generation).
- Moving to nodes with different storage configuration.
- Replacing a node pool that uses a deprecated OS image.
- Switching from a shared node pool to a dedicated one.

## Prerequisites

- The new node pool must already exist with appropriate labels, taints, and instance types.
  See [Set up dedicated node pools](../deploy-scylladb/set-up-dedicated-node-pools.md) for setup.
- A `NodeConfig` resource targeting the new node pool must be applied (if your cluster uses tuning, RAID, or filesystem configuration).
- The replication factor of your keyspaces must be large enough to tolerate the temporary imbalance during migration.
  For example, with RF=3 you can safely have one rack temporarily empty.
- The new rack must be in the same datacenter as the old rack.

## Procedure

The migration follows a gradual scale-up / scale-down pattern:
add nodes to the new rack one at a time, then remove nodes from the old rack one at a time.

:::::{tabs}
::::{group-tab} ScyllaCluster

Suppose you have a ScyllaCluster with a rack `us-east-1a` on the old node pool and you want to migrate it to a new node pool labelled `pool: scylladb-new`.

### Step 1: Add the new rack with zero members

Add a new rack to the spec that targets the new node pool.
Set `members: 0` initially — no pods are created yet.

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
      - name: us-east-1a            # old rack
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        placement:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                    - key: pool
                      operator: In
                      values:
                        - scylladb-old
          tolerations:
            - key: role
              operator: Equal
              value: scylladb
              effect: NoSchedule
      - name: us-east-1b            # new rack — starts at 0
        members: 0
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        placement:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                    - key: pool
                      operator: In
                      values:
                        - scylladb-new
          tolerations:
            - key: role
              operator: Equal
              value: scylladb
              effect: NoSchedule
```

Apply the change:

```bash
kubectl -n scylla apply -f scyllacluster.yaml
```

### Step 2: Scale up the new rack one node at a time

Increase the new rack's `members` by 1.
Each new node joins the cluster, takes ownership of a token range, and begins streaming data from existing nodes.

```bash
kubectl -n scylla patch scyllacluster/scylla --type=json \
  -p='[{"op": "replace", "path": "/spec/datacenter/racks/1/members", "value": 1}]'
```

Wait for the new node to be ready:

```bash
kubectl -n scylla wait --timeout=15m --for='condition=Available' scyllaclusters.scylla.scylladb.com/scylla
```

Verify the cluster state:

```bash
kubectl -n scylla exec -it scylla-us-east-1-us-east-1a-0 -c scylla -- nodetool status
```

Repeat this step until the new rack has the same number of members as the old rack.

### Step 3: Scale down the old rack one node at a time

Decrease the old rack's `members` by 1.
The Operator decommissions the highest-ordinal node, streaming its data to the remaining nodes before deleting the pod.

```bash
kubectl -n scylla patch scyllacluster/scylla --type=json \
  -p='[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 2}]'
```

Wait for the decommission to finish:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Available' scyllaclusters.scylla.scylladb.com/scylla
```

Repeat until the old rack has 0 members.

### Step 4: Remove the old rack

Once the old rack has 0 members, remove its definition from the spec:

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
      - name: us-east-1b            # only the new rack remains
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        placement:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                    - key: pool
                      operator: In
                      values:
                        - scylladb-new
          tolerations:
            - key: role
              operator: Equal
              value: scylladb
              effect: NoSchedule
```

:::{warning}
Do not remove a rack from the spec while it still has members.
The Operator rejects this change through validation.
:::

### Step 5: Run a repair

After migration, run a repair to ensure data consistency across the new token ranges:

```bash
kubectl -n scylla exec -it scylla-us-east-1-us-east-1b-0 -c scylla -- nodetool repair -pr
```

Or use a ScyllaDB Manager repair task if Manager is configured.

::::
::::{group-tab} ScyllaDBCluster

For ScyllaDBCluster, the same principle applies.
The `nodes` field on each rack controls the number of members (equivalent to `members` in ScyllaCluster).

### Step 1: Add the new rack with zero nodes

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenters:
    - name: us-east-1
      remoteKubernetesClusterName: us-east-1
      racks:
        - name: a                    # old rack
          nodes: 3
          placement:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: pool
                        operator: In
                        values:
                          - scylladb-old
            tolerations:
              - key: role
                operator: Equal
                value: scylladb
                effect: NoSchedule
        - name: b                    # new rack — starts at 0
          nodes: 0
          placement:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: pool
                        operator: In
                        values:
                          - scylladb-new
            tolerations:
              - key: role
                operator: Equal
                value: scylladb
                effect: NoSchedule
```

### Step 2: Gradually scale up the new rack

Increase `nodes` by 1 at a time on the new rack.
Wait for each new node to become ready and verify with `nodetool status`.

### Step 3: Gradually scale down the old rack

Decrease `nodes` by 1 at a time on the old rack.
Wait for each decommission to complete.

### Step 4: Remove the old rack

Once the old rack has 0 nodes, remove it from the spec.

### Step 5: Run a repair

Run a repair to ensure data consistency.

::::
:::::

## Key considerations

| Consideration | Detail |
|---|---|
| One node at a time | Scale up and down one node at a time to minimise cluster impact. Each join and decommission involves data streaming. |
| Scale up before scaling down | Always add a node to the new rack before removing one from the old rack to maintain capacity. |
| Replication factor | Ensure your replication factor tolerates the temporary rack imbalance. With RF=3 and 3 racks, you can have one rack empty. |
| Streaming impact | Data streaming consumes network bandwidth and disk I/O. Schedule migrations during low-traffic periods. |
| Same datacenter | The new rack must be in the same datacenter as the old rack. Cross-datacenter rack migration is not supported. |
| NodeConfig | If the new node pool requires different tuning (RAID, filesystem, sysctls), ensure a `NodeConfig` targeting the new nodes is applied before starting the migration. |
| Run repair after migration | Always run a repair after the migration completes to ensure all replicas are consistent. |

## Related pages

- [Scale a ScyllaDB cluster](scale-cluster.md) — adding and removing racks, scaling up and down
- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — how racks map to StatefulSets, scaling mechanics, and decommission workflow
- [Set up dedicated node pools](../deploy-scylladb/set-up-dedicated-node-pools.md) — setting up node pools with labels, taints, and NodeConfig
