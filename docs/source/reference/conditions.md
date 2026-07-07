# Conditions reference

The ScyllaDB Operator uses Kubernetes status conditions to communicate health and progress of managed resources.
This page documents the top-level condition types, their semantics, and aggregation rules.

These conditions apply to all Operator-managed custom resources that report status conditions (e.g. `ScyllaCluster`, `ScyllaDBDatacenter`, `NodeConfig`).

## Top-level conditions

Every condition-reporting resource exposes three top-level conditions:

```{list-table}
:header-rows: 1

* - Condition type
  - Meaning when `True`
  - Meaning when `False`
  - Meaning when `Unknown`
* - `Available`
  - The resource is functional and can serve its purpose
  - The resource is not functional
  - Availability cannot yet be determined
* - `Progressing`
  - The Operator is actively reconciling (rolling update, scaling, version change)
  - Either completed or stuck
  - Reconciliation state cannot yet be determined
* - `Degraded`
  - Something is unhealthy or an error occurred during reconciliation
  - Resource is healthy
  - Health cannot yet be determined
```

A fully healthy, quiescent resource shows: `Available=True`, `Progressing=False`, `Degraded=False`.

A resource can be `Available=True` and `Degraded=True` simultaneously — for example, a `ScyllaCluster` where two of three nodes are healthy can still serve traffic but is not fully healthy.
`Available` reflects whether the resource *can* serve its purpose, not whether it is in a *fully* healthy state.

## Aggregation rules

The top-level conditions are computed from per-controller sub-conditions.
Each internal controller contributes a condition whose type is prefixed with the controller name and ends with `Available`, `Progressing`, or `Degraded` (e.g. `StatefulSetControllerDegraded`, `JobControllerProgressing`).

The aggregation logic is:

- **`Available=True`** requires **all** `*Available` sub-conditions to be `True`.
- **`Progressing=True`** when **any** `*Progressing` sub-condition is `True`.
- **`Degraded=True`** when **any** `*Degraded` sub-condition is `True`.

To inspect all conditions on a `ScyllaCluster` (including sub-conditions), run:

```bash
kubectl -n scylla get scyllacluster <cluster-name> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} — {.reason}: {.message}{"\n"}{end}'
```

## Condition fields

Each condition object contains:

```{list-table}
:header-rows: 1

* - Field
  - Description
* - `type`
  - Condition name (e.g. `Available`, `StatefulSetControllerDegraded`)
* - `status`
  - `True`, `False`, or `Unknown`
* - `reason`
  - A CamelCase machine-readable reason string
* - `message`
  - A human-readable description of the current state
* - `lastTransitionTime`
  - When the condition last changed
* - `observedGeneration`
  - The `metadata.generation` the condition was computed from
```

## Related pages

- [Investigate restarts](../troubleshoot/investigate-restarts.md) — using conditions alongside pod events
