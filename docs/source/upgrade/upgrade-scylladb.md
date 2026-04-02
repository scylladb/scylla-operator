# Upgrade ScyllaDB

Upgrade the ScyllaDB version running in your cluster by changing the image reference in the `ScyllaCluster` spec.

:::{warning}
ScyllaDB version upgrades must be performed **consecutively** — do not skip any major or minor version on the upgrade path.
Ensure the entire cluster has completed one upgrade before starting the next.
For details, refer to [ScyllaDB upgrade procedures](https://enterprise.docs.scylladb.com/stable/upgrade/index.html#upgrade-upgrade-procedures).
:::

:::{warning}
While the cluster remains operational throughout the upgrade, applications requiring strict consistency levels (such as `QUORUM`) may experience transient unavailability.
This can occur if the cluster topology view has not yet fully converged across all nodes before the next node is restarted.
Schedule upgrades during periods of low application traffic.
:::

:::{caution}
Before upgrading ScyllaDB, ensure the target version is supported by the version of ScyllaDB Operator you are using.
Refer to the [support matrix](../reference/releases.md) for version compatibility.
:::

## Before you begin

1. Verify the current ScyllaDB version:
   ```bash
   kubectl -n scylla exec scylla-us-east-1a-0 -c scylla -- nodetool version
   ```

2. Ensure the cluster is healthy — all nodes report `UN` (Up and Normal):
   ```bash
   kubectl -n scylla exec scylla-us-east-1a-0 -c scylla -- nodetool status
   ```

3. Check that no other operations are in progress — the cluster should not be scaling, replacing nodes, or in the middle of another upgrade:
   ```bash
   kubectl -n scylla get scyllaclusters.scylla.scylladb.com scylla -o wide
   ```

4. Review the [ScyllaDB release notes](https://www.scylladb.com/product/release-notes/) for the target version.

## How the upgrade works

The Operator detects a version change and chooses one of two upgrade strategies:

| Change type | Example | Strategy |
|---|---|---|
| Patch version only | `5.4.0` → `5.4.1` | Standard rolling update — pods are restarted with the new image one at a time in reverse ordinal order. |
| Major or minor version | `5.4.0` → `6.0.0` or `5.4.0` → `5.5.0` | Partition-based rollout with pre-upgrade hooks (see below). |

### Partition-based rollout (major/minor upgrades)

For major or minor version changes, the Operator runs additional safety steps:

1. **Schema agreement** — waits for all nodes to agree on the schema before starting.
2. **System keyspace snapshots** — snapshots the `system` and `system_schema` keyspaces on all nodes.
3. **Per-node upgrade** — for each node, one at a time:
   - Enables maintenance mode.
   - Drains the node (flushes memtables and stops accepting requests).
   - Snapshots all data keyspaces.
   - Disables maintenance mode.
   - Deletes the pod so the StatefulSet recreates it with the new image.
   - Waits for the new pod to become Ready.
   - Cleans up the data snapshot.
4. **Post-upgrade cleanup** — removes system keyspace snapshots from all nodes.

The Operator upgrades one rack at a time and one node at a time within each rack, preserving cluster availability.

## Upgrade via GitOps / kubectl

Change the ScyllaDB image reference in your manifest and apply it.\n\nSet `spec.version` to the target ScyllaDB image tag:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
  namespace: scylla
spec:
  version: "6.0.0"          # target ScyllaDB version tag
  # ...
```

Apply the manifest:

```bash
kubectl apply -f scyllacluster.yaml
```

Wait for the rollout to complete:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl -n scylla wait --timeout=30m --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

## Upgrade via Helm

Upgrade the Helm release with the target image tag:

```bash
helm upgrade scylla scylla/scylla --reuse-values --set=scyllaImage.tag=6.0.0
```

Wait for the rollout:

```bash
kubectl -n scylla wait --timeout=30m --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl -n scylla wait --timeout=30m --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl -n scylla wait --timeout=30m --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

:::{note}
In multi-DC clusters using multiple `ScyllaCluster` resources, upgrade each datacenter's `ScyllaCluster` independently, following ScyllaDB's rolling upgrade order (one DC at a time).
:::

## Verify the upgrade

After the rollout completes, verify that all nodes are running the new version:

```bash
NAMESPACE=scylla
CLUSTER_NAME=scylladb

pods=$(kubectl -n "${NAMESPACE}" get pods \
    -l scylla/cluster="${CLUSTER_NAME}" \
    -l scylla-operator.scylladb.com/pod-type=scylladb-node \
    -o name)

for pod in ${pods}; do
    kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool version
done
```

Confirm all nodes report `UN` (Up and Normal):

```bash
kubectl -n scylla exec scylladb-us-east-1a-0 -c scylla -- nodetool status
```

Expected output:

```
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d us-east-1a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 us-east-1b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 us-east-1c
```

## Monitor upgrade progress

For major/minor version upgrades, the ScyllaCluster status includes an `upgrade` section that tracks the current state:

```bash
kubectl -n scylla describe scyllacluster scylladb
```

Look for the `Upgrade` section in the status:

```
Status:
  Upgrade:
    Current Node:         scylladb-us-east-1a-2
    Current Rack:         us-east-1a
    Data Snapshot Tag:    so_data_20241228135002UTC
    From Version:         5.4.0
    State:                RolloutRun
    System Snapshot Tag:  so_system_20241228135002UTC
    To Version:           6.0.0
```

| Field | Description |
|---|---|
| `State` | Current phase: `PreHooks`, `RolloutInit`, `RolloutRun`, or `PostHooks`. |
| `From Version` | ScyllaDB version before the upgrade. |
| `To Version` | Target ScyllaDB version. |
| `Current Rack` | Rack currently being upgraded. |
| `Current Node` | Node currently being upgraded. |
| `System Snapshot Tag` | Name of the system keyspace snapshot (taken at the start). |
| `Data Snapshot Tag` | Name of the data keyspace snapshot (taken per node). |

## Recovering from a failed upgrade

If the upgrade gets stuck — for example, a pod fails to start after being recreated with the new image — follow these steps:

1. **Identify the failing pod**:
   ```bash
   kubectl -n scylla get pods
   kubectl -n scylla logs <pod-name> -c scylla --previous
   ```

2. **Scale down the Operator** to prevent it from interfering while you diagnose:
   ```bash
   kubectl -n scylla-operator scale deployment/scylla-operator --replicas=0
   ```

3. **Resolve the issue**.
   Common causes include incompatible configuration, corrupt SSTables, or insufficient resources.
   System and data snapshots are available on each node under the tags shown in the upgrade status — you can use them to recover data if needed.

4. **Once the pod is healthy**, scale the Operator back up:
   ```bash
   kubectl -n scylla-operator scale deployment/scylla-operator --replicas=2
   ```

   The Operator resumes the upgrade from where it left off.

:::{caution}
Do not manually change the ScyllaDB version tag back to the old version while an upgrade is in progress.
Let the Operator finish the current upgrade or resolve the issue before making further changes.
:::

## Related pages

- [Upgrade ScyllaDB Operator](upgrade-operator.md) — upgrading the Operator itself
- [Upgrade Kubernetes](upgrade-kubernetes.md) — upgrading the Kubernetes control plane and node pools
- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — partition-based rollout mechanics
- [Perform a rolling restart](../operate/perform-rolling-restart.md) — restarting all nodes without changing the ScyllaDB version
- [Replace nodes](../operate/replace-nodes.md) — replacing a specific node after a failed upgrade
