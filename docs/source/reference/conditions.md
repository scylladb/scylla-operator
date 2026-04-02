# ScyllaCluster conditions reference

The ScyllaDB Operator uses Kubernetes status conditions on `ScyllaCluster` resources to communicate health and progress.
This page is a reference for all conditions — their types, semantics, and expected values.

## Top-level aggregated conditions

Every `ScyllaCluster` exposes three top-level aggregated conditions:

| Condition type | Meaning when `True` | Meaning when `False` |
|---|---|---|
| `Available` | At least one rack has all members running and ready | No rack is fully available; the cluster cannot serve traffic |
| `Progressing` | The Operator is actively reconciling (rolling update, scaling, version change) | No active reconciliation in progress |
| `Degraded` | One or more members are unhealthy or an error occurred during reconciliation | All members are healthy and reconciliation is error-free |

A fully healthy, quiescent cluster shows: `Available=True`, `Progressing=False`, `Degraded=False`.

A cluster can be `Available=True` and `Degraded=True` simultaneously — for example, when two of three nodes are healthy.
`Available` reflects whether the cluster *can* serve traffic, not whether it is *fully* healthy.

## Aggregation rules

The top-level conditions are computed from per-controller sub-conditions:

- **`Available=True`** requires **all** `*Available` sub-conditions to be `True`.
- **`Progressing=True`** when **any** `*Progressing` sub-condition is `True`.
- **`Degraded=True`** when **any** `*Degraded` sub-condition is `True`.

## Per-controller sub-conditions

Each internal controller appends a condition whose type ends with `Available`, `Progressing`, or `Degraded`:

| Sub-condition prefix | Controller | What it tracks |
|---|---|---|
| `StatefulSetController` | StatefulSet controller | ScyllaDB member Pods — replica count, readiness, version |
| `ServiceController` | Service controller | Headless and member Services |
| `PodDisruptionBudgetController` | PDB controller | PodDisruptionBudget resources |
| `CertController` | Certificate controller | TLS certificates (if `AutomaticTLSCertificates` is enabled) |
| `IngressController` | Ingress controller | Ingress objects (if configured) |
| `AgentController` | Agent configuration controller | Manager Agent configuration ConfigMaps |
| `ConfigMapController` | ConfigMap controller | Operator-managed ConfigMaps |
| `JobController` | Job controller | Operator-managed Jobs |
| `ServiceAccountController` | ServiceAccount controller | ServiceAccounts and RBAC bindings |
| `NamespaceController` | Namespace controller | Namespace-scoped resources |
| `NodeConfigController` | NodeConfig controller | NodeConfig readiness for hosting ScyllaDB pods |

To see which sub-condition is causing a top-level condition, run:

```bash
kubectl -n scylla get scyllacluster <cluster-name> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} — {.reason}: {.message}{"\n"}{end}'
```

## Condition fields

Each condition has the following fields:

| Field | Description |
|---|---|
| `type` | Condition name (e.g. `Available`, `StatefulSetControllerDegraded`) |
| `status` | `True` or `False` |
| `reason` | A CamelCase machine-readable reason string |
| `message` | A human-readable description of the current state |
| `lastTransitionTime` | When the condition last changed |
| `observedGeneration` | The `metadata.generation` the condition was computed from |

## Related pages

- [Check cluster health](../troubleshoot/check-cluster-health.md) — how-to guide for using conditions to diagnose a cluster
- [Investigate restarts](../troubleshoot/investigate-restarts.md) — using conditions alongside pod events
