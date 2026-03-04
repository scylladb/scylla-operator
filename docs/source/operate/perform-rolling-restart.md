# Perform a rolling restart

Force a rolling restart of all ScyllaDB nodes in a cluster by changing the `forceRedeploymentReason` field.

## How it works

The Operator uses `forceRedeploymentReason` as a hash input for the StatefulSet pod template.
When you change the value, the pod template hash changes, which triggers the StatefulSet controller to perform a rolling update.
The Operator orchestrates this update one pod at a time, draining each node before terminating it and waiting for the replacement to become ready before proceeding to the next.

The value itself is arbitrary — it only needs to be different from the previous value.
Using a descriptive string (e.g. `"config change 2024-01-15"`) makes it easier to identify why a restart was triggered.

## Restart a ScyllaCluster

Patch the `spec.forceRedeploymentReason` field:

```bash
kubectl -n scylla patch scyllacluster/scylla --type=merge \
  -p='{"spec": {"forceRedeploymentReason": "restart-2024-01-15"}}'
```

Wait for the rolling restart to complete:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla
kubectl -n scylla wait --timeout=30m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla
```

## Restart a ScyllaDBDatacenter

Patch the `spec.forceRedeploymentReason` field:

```bash
kubectl -n scylla patch scylladbdatacenter/scylla --type=merge \
  -p='{"spec": {"forceRedeploymentReason": "restart-2024-01-15"}}'
```

Wait for the rolling restart to complete:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scylladbdatacenters.scylla.scylladb.com/scylla
kubectl -n scylla wait --timeout=30m --for='condition=Degraded=False' scylladbdatacenters.scylla.scylladb.com/scylla
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scylladbdatacenters.scylla.scylladb.com/scylla
```

## Restart a ScyllaDBCluster

For multi-datacenter clusters, `forceRedeploymentReason` is available at two levels:

- **Cluster level** (`spec.forceRedeploymentReason`) — restarts all nodes in every datacenter.
- **Datacenter level** (`spec.datacenters[].forceRedeploymentReason`) — restarts only the nodes in a specific datacenter.

### Restart all datacenters

```bash
kubectl -n scylla patch scylladbcluster/scylla --type=merge \
  -p='{"spec": {"forceRedeploymentReason": "restart-2024-01-15"}}'
```

### Restart a single datacenter

```bash
kubectl -n scylla patch scylladbcluster/scylla --type=json \
  -p='[{"op": "replace", "path": "/spec/datacenters/0/forceRedeploymentReason", "value": "restart-2024-01-15"}]'
```

Wait for the cluster to stabilise:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scylladbclusters.scylla.scylladb.com/scylla
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scylladbclusters.scylla.scylladb.com/scylla
```

## Key considerations

| Consideration | Detail |
|---|---|
| One at a time | The Operator restarts one node at a time per rack. Each node is drained before termination, and the replacement must be ready before the next node is restarted. |
| Unique value | The value of `forceRedeploymentReason` must be different from the current value to trigger a restart. Repeating the same string has no effect. |
| Impact on availability | A rolling restart temporarily reduces the number of available replicas by one. Ensure your replication factor allows for one node to be unavailable. |
| PodDisruptionBudget | The PDB (`maxUnavailable: 1`) ensures that at most one node per datacenter is unavailable at any given time, even during a rolling restart. |

## Related pages

- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — how StatefulSets manage rolling updates with partition-based rollouts
- [Upgrade ScyllaDB](upgrade-scylladb.md) — version upgrades that include an automatic rolling restart
- [Back up and restore](back-up-and-restore.md) — schema restores on older ScyllaDB versions require a rolling restart after the restore completes
