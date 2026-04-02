# Check cluster health

Assess the health of a ScyllaDB cluster using ScyllaCluster conditions, pod status, and `nodetool status`.

## ScyllaCluster conditions

The ScyllaCluster resource reports aggregated health through status conditions:

```bash
kubectl -n scylla get scyllacluster <cluster-name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

### Top-level conditions

| Condition | Meaning when `True` |
|---|---|
| `Available` | At least one rack has all members running and ready |
| `Progressing` | The Operator is actively reconciling (rolling update, scaling, etc.) |
| `Degraded` | One or more members are not ready or an error occurred during reconciliation |

A fully healthy cluster shows `Available=True`, `Progressing=False`, `Degraded=False`.

:::{note}
A cluster can be both `Available` and `Degraded` at the same time — for example, when two out of three nodes are healthy but one is down.
The `Available` condition reflects whether the cluster can serve traffic, not whether it is fully healthy.
:::

### How conditions are aggregated

The top-level `Available`, `Progressing`, and `Degraded` conditions are **aggregated** from per-controller sub-conditions.
Each internal controller reports its own condition (for example, `StatefulSetControllerAvailable`, `ServiceControllerProgressing`, `CertControllerDegraded`).
The Operator finds all conditions whose type ends with the `Available`, `Progressing`, or `Degraded` suffix and merges them:

- **`Available`** is `True` only when **all** `*Available` sub-conditions are `True`.
- **`Progressing`** is `True` when **any** `*Progressing` sub-condition is `True`.
- **`Degraded`** is `True` when **any** `*Degraded` sub-condition is `True`.

To identify which controller is causing a condition, inspect the sub-conditions:

```bash
kubectl -n scylla get scyllacluster <cluster-name> -o jsonpath='{range .status.conditions[*]}{.type}: {.status} - {.reason}: {.message}{"\n"}{end}'
```

Common sub-conditions to look for:

| Sub-condition | What it indicates |
|---|---|
| `StatefulSetControllerAvailable=False` | Not all ScyllaDB members are ready or at the desired version |
| `StatefulSetControllerProgressing=True` | A rolling update, scale, or version upgrade is in progress |
| `StatefulSetControllerDegraded=True` | An error occurred managing StatefulSets |
| `ServiceControllerDegraded=True` | An error occurred managing per-node Services |
| `ConfigControllerDegraded=True` | An error occurred managing ScyllaDB configuration |

In multi-DC clusters using multiple `ScyllaCluster` resources, check conditions on each `ScyllaCluster` independently and cross-check with `nodetool status` for the combined ring view.

## Pod status

List all ScyllaDB pods:

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -o wide
```

| Status | Meaning | Remediation |
|---|---|---|
| `Running` with all containers ready | Node is healthy | Healthy — no action needed |
| `Running` but not all containers ready | Readiness probe failing — node may be starting up or overloaded | Ignition not complete. Check NodeConfig tuning status and LoadBalancer IP. See [diagnose-node-not-starting.md](diagnose-node-not-starting.md) |
| `Pending` | Cannot be scheduled — see [Node not starting](diagnose-node-not-starting.md) | Check scheduling: `kubectl describe pod`. Fix node affinity, tolerations, or resource requests. See [diagnose-node-not-starting.md](diagnose-node-not-starting.md) |
| `CrashLoopBackOff` | ScyllaDB crashing — see [Investigating restarts](investigate-restarts.md) | ScyllaDB crashing on startup. Check logs: `kubectl logs <pod> -c scylla --previous`. See [investigate-restarts.md](investigate-restarts.md) |

Check individual pod details:

```bash
kubectl -n scylla describe pod <pod-name>
```

## nodetool status

Run `nodetool status` on any healthy ScyllaDB pod to see the cluster topology:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool status
```

**Example output:**

```
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens  Owns  Host ID                               Rack
UN  10.244.1.5   1.2 GB     256     ?     a1b2c3d4-e5f6-7890-abcd-ef1234567890  us-east-1a
UN  10.244.2.6   1.1 GB     256     ?     b2c3d4e5-f6a7-8901-bcde-f12345678901  us-east-1a
DN  10.244.3.7   1.3 GB     256     ?     c3d4e5f6-a7b8-9012-cdef-123456789012  us-east-1a
```

### Reading the output

| Column | Description |
|---|---|
| Status (`U`/`D`) | `U` = Up (reachable via gossip), `D` = Down (unreachable) |
| State (`N`/`L`/`J`/`M`) | `N` = Normal, `L` = Leaving (decommissioning), `J` = Joining, `M` = Moving |
| Address | The broadcast address of the node (matches the first IP family in `network.ipFamilies`) |
| Load | Amount of data stored on the node |
| Host ID | Unique identifier for the node in the cluster |

### Common unhealthy states

| State | Meaning | Action |
|---|---|---|
| `DN` | Down / Normal — node crashed or is unreachable | Check pod status, logs, and events. See [Investigating restarts](investigate-restarts.md). |
| `UJ` | Up / Joining — stuck join | Node may be streaming data. Wait or check logs for errors. |
| `UL` | Up / Leaving — stuck decommission | Check for streaming errors. May need to wait for data transfer to complete. |
| `?N` | Unknown — gossip timeout | Network issue between nodes. Check network policies, DNS, and node connectivity. |

## nodetool gossipinfo

For deeper gossip-level diagnostics:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool gossipinfo
```

This shows per-node gossip state including heartbeat generation, schema version, status, host ID, and RPC address.
Look for:
- **Schema disagreements** — different `SCHEMA` values across nodes indicate a schema sync issue.
- **Abnormal `STATUS`** — values other than `NORMAL` indicate a node in transition.
- **Stale heartbeats** — a `HEARTBEAT` generation timestamp far in the past may indicate a zombie node.

## Service and endpoint health

Verify that Kubernetes Services have active endpoints:

```bash
# Client service
kubectl -n scylla get endpoints <cluster-name>-client

# Per-node services
kubectl -n scylla get svc -l scylla/cluster=<cluster-name>
```

If the client Service has no endpoints, no ScyllaDB pods are passing readiness probes.

## Quick health check script

```bash
NAMESPACE=scylla
CLUSTER=my-cluster

echo "=== ScyllaCluster conditions ==="
kubectl -n "${NAMESPACE}" get scyllacluster "${CLUSTER}" \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.message}){"\n"}{end}'

echo ""
echo "=== Pod status ==="
kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER}" -o wide

echo ""
echo "=== nodetool status ==="
kubectl -n "${NAMESPACE}" exec -it "${CLUSTER}-$(
  kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER}" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null | head -1
)" -c scylla -- nodetool status 2>/dev/null || echo "Could not reach any ScyllaDB pod"
```

## Related pages

- [Diagnostic flowchart](diagnostic-flowchart.md)
- [Investigating restarts](investigate-restarts.md)
- [Node not starting](diagnose-node-not-starting.md)
- [Collecting debugging information](collect-debugging-information/index.md)
