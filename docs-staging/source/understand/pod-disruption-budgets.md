# Pod disruption budgets

This page explains how ScyllaDB Operator uses Kubernetes PodDisruptionBudgets (PDBs) to protect ScyllaDB availability during voluntary disruptions.

## What a PDB does

A [PodDisruptionBudget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-budgets) limits how many pods matching a selector can be voluntarily evicted at the same time. Voluntary disruptions include:

- Kubernetes node drains (maintenance, upgrades).
- Cluster autoscaler scale-down.
- Manual pod evictions via the Eviction API.

PDBs do **not** protect against involuntary disruptions such as hardware failures or OOM kills.

## ScyllaDB cluster PDB

The Operator creates one PDB per ScyllaDB datacenter with:

```yaml
spec:
  maxUnavailable: 1
```

This ensures that **at most one ScyllaDB node** can be voluntarily disrupted at a time across the entire datacenter. The Kubernetes API server blocks eviction requests that would violate this budget.

### Excluding cleanup Jobs

The PDB selector uses the same labels as the ScyllaDB pods but adds a `MatchExpression` that excludes pods with the `batch.kubernetes.io/job-name` label. Kubernetes automatically adds this label to every pod created by a Job. This means cleanup Job pods (see [Automatic data cleanup](automatic-data-cleanup.md)) do not count toward the PDB budget and cannot block node drains.

## Operator and webhook PDBs

The Operator deployment and the webhook server deployment each have their own PDB:

| Component | PDB spec | When created |
|-----------|----------|-------------|
| `scylla-operator` | `minAvailable: 1` | When running with more than one replica |
| `webhook-server` | `minAvailable: 1` | When running with more than one replica |

These PDBs ensure that at least one Operator pod and one webhook pod remain available during node drains, preventing a complete loss of the control plane during cluster maintenance.

## PDB interaction with operations

### Rolling updates

The Operator uses a **partition-based rollout** strategy for StatefulSets. During an upgrade:

1. All StatefulSets are partitioned at their current replica count, preventing any pod from restarting.
2. The partition is decremented by one, allowing a single pod to pick up the new template and restart.
3. The controller waits for the restarted pod to become ready before decrementing the partition again.
4. Only one rack makes progress per reconciliation cycle.

This one-at-a-time rollout naturally respects the `maxUnavailable: 1` PDB because at most one pod is unavailable during each step.

### Scale-down

When scaling down, the Operator decommissions one member at a time. The SidecarController drives the decommission process inside each pod. Because only one pod is being removed at a time, the PDB is not violated.

### Node replacement

Node replacement follows a similar pattern — one node is replaced at a time. The PDB prevents the Kubernetes scheduler from evicting additional ScyllaDB pods while a replacement is in progress.

### Kubernetes node drains

When a Kubernetes node is drained (for example, during a Kubernetes upgrade), the drain process evicts pods through the Eviction API, which respects PDBs. If the ScyllaDB cluster already has one node unavailable (due to a concurrent operation or failure), the PDB blocks further evictions until the first node recovers.

## Related pages

- [Statefulsets and racks](statefulsets-and-racks.md) — rolling update strategy and partition-based rollout.
- [Overview](overview.md) — reconciliation model.
