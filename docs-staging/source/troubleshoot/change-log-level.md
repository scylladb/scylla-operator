# Change log level on a live cluster

Change the ScyllaDB log level without a full rolling restart.
This is useful when a rolling restart is not feasible — for example, when a StatefulSet is stuck mid-rollout or the cluster is degraded.

## When to use each method

| Scenario | Method | Persistent? |
|---|---|---|
| Normal operations — rolling restart is acceptable | [Spec change](#method-1-spec-change-rolling-restart) | Yes |
| StatefulSet stuck mid-rollout | [REST API](#method-2-rest-api-no-restart) | No — lost on pod restart |
| Cluster degraded — cannot tolerate a rolling restart | [REST API](#method-2-rest-api-no-restart) | No — lost on pod restart |

## Method 1: Spec change (rolling restart)

Add the log level argument to the ScyllaCluster spec:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: my-cluster
  namespace: scylla
spec:
  # ... existing configuration ...
  scyllaArgs: "--default-log-level=debug"
```

Apply the change:

```bash
kubectl apply --server-side -f scylla-cluster.yaml
```

ScyllaDB Operator performs a rolling restart to apply the new argument.

See [Passing ScyllaDB arguments](../operate/pass-scylladb-arguments.md) for details.

## Method 2: REST API (no restart)

Use the ScyllaDB REST API to change the log level on running pods without triggering a rollout.

### Change log level on a single pod

```bash
kubectl -n scylla exec <pod-name> -c scylla -- \
  curl -s -X POST "http://localhost:10000/system/logger/<logger-name>?level=<level>"
```

Where:
- `<logger-name>` is the logger to adjust (e.g., `compaction`, `gossip`, `storage_proxy`)
- `<level>` is the desired level: `error`, `warn`, `info`, `debug`, `trace`

To set **all** loggers at once, omit the logger name from the path:

```bash
kubectl -n scylla exec <pod-name> -c scylla -- \
  curl -s -X POST "http://localhost:10000/system/logger?level=<level>"
```

**Example — set all loggers to debug:**

```bash
kubectl -n scylla exec <pod-name> -c scylla -- \
  curl -s -X POST "http://localhost:10000/system/logger?level=debug"
```

### Change log level on all pods

```bash
NAMESPACE=scylla
CLUSTER=my-cluster

for pod in $(kubectl -n "${NAMESPACE}" get pods \
  -l scylla/cluster="${CLUSTER}" \
  -l scylla-operator.scylladb.com/pod-type=scylladb-node \
  -o jsonpath='{.items[*].metadata.name}'); do
  echo "Setting log level on ${pod}..."
  kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- \
    curl -s -X POST "http://localhost:10000/system/logger?level=debug"
done
```

### Verify the change

Check the level of a specific logger:

```bash
kubectl -n scylla exec <pod-name> -c scylla -- \
  curl -s "http://localhost:10000/system/logger/compaction"
```

The response is the current log level (e.g., `"debug"`).

### Important notes

:::{caution}
REST API log level changes are **ephemeral** — they are lost when the pod restarts.
To make the change persistent, update the ScyllaCluster spec as described in [Method 1](#method-1-spec-change-rolling-restart).
:::

- **Debug and trace levels** generate significantly more log output and can impact performance.
  Revert to `info` once you have collected the needed diagnostics.
- In a stuck rollout scenario, ScyllaDB Operator cannot process spec changes because the rollout is blocked.
  The REST API is the only way to change log levels on pods that are already running.
  See [StatefulSets and racks](../understand/statefulsets-and-racks.md) for why a stuck rollout blocks further updates (partition-based rolling updates).

## Related pages

- [Passing ScyllaDB arguments](../operate/pass-scylladb-arguments.md)
- [StatefulSets and racks](../understand/statefulsets-and-racks.md)
- [Diagnostic flowchart](diagnostic-flowchart.md)
