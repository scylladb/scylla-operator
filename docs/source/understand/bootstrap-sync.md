# Bootstrap synchronisation

This page explains why bootstrap synchronisation exists, how the barrier mechanism works, and how node statuses are propagated across the cluster.

## The problem

[ScyllaDB requires that no node in the cluster considers any other node to be down when a new node joins.](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/add-node-to-cluster.html#check-the-status-of-nodes)
If this precondition is not met and a non-idempotent bootstrap operation begins, the coordinator denies the join request and leaves the new node in a state that is difficult to recover from automatically.

In Kubernetes, multiple pods can start simultaneously — for example, when a StatefulSet scales up or when pods are rescheduled after a disruption. Without coordination, a new node could attempt to bootstrap while another node is still restarting and appears down to its peers.

## How the barrier works

When the `BootstrapSynchronisation` feature gate is enabled, the Operator adds an **init container** (`scylladb-bootstrap-barrier`) to every ScyllaDB pod. This init container runs before the ScyllaDB process starts and gates the bootstrap on a precondition check.

### Decision flow

1. **Already bootstrapped?** — The init container inspects the data directory for existing SSTables. If the node has already completed bootstrap (the `bootstrapped` column in `system.local` reads `COMPLETED`), the barrier exits immediately. Restarting nodes are never blocked.

2. **Force annotation set?** — If the node's member Service or the `ScyllaDBDatacenter` carries the annotation `scylla-operator.scylladb.com/force-proceed-to-bootstrap: "true"`, the barrier exits immediately, bypassing the precondition.

3. **Replacing a dead node?** — If the node is being added as a replacement (the replacement annotation is present on the Service), the barrier exits immediately. Replacement has its own prerequisites that are outside the scope of this mechanism.

4. **Precondition check** — The init container watches `ScyllaDBDatacenterNodesStatusReport` resources and evaluates whether every reporting node in the cluster sees every other node as `UP`. The barrier blocks until this condition is satisfied.

## Node status propagation

Node statuses flow through a three-stage pipeline:

### Stage 1 — Sidecar reports per-node status

A `StatusReporter` controller runs inside the sidecar container on every ScyllaDB pod. It periodically calls the local ScyllaDB node's storage service API to get the current gossip view — which nodes are seen as `UP` or `DOWN`. The result is written as a JSON-encoded annotation on the pod.

### Stage 2 — Datacenter controller assembles the report

On each reconciliation, the `ScyllaDBDatacenter` controller reads the status annotation from every pod in the datacenter and assembles them into a single `ScyllaDBDatacenterNodesStatusReport` custom resource. This resource is cluster-scoped and contains a nested structure:

```
ScyllaDBDatacenterNodesStatusReport
└── datacenter (gossip DC name)
    └── rack[]
        └── node[]
            ├── ordinal
            ├── hostID
            └── observedNodeStatuses[]
                ├── hostID (of the observed node)
                └── status ("UP" or "DOWN")
```

Each node entry records how that node sees every other node in the cluster.

### Stage 3 — Cross-datacenter propagation (multi-DC)

For `ScyllaDBCluster` deployments spanning multiple Kubernetes clusters, the `ScyllaDBCluster` controller propagates `ScyllaDBDatacenterNodesStatusReport` resources to remote clusters so that each datacenter's bootstrap barrier can evaluate the global precondition.

## Precondition evaluation

The precondition is satisfied when **every** reporting node (excluding the node being bootstrapped) sees **every** other node as `UP`. Specifically:

- Every node must have a host ID.
- Every node must have submitted a status report.
- In each node's report, every other node's host ID must appear with status `UP`.

If any node is missing a host ID, has not yet reported, does not list another node, or reports another node as `DOWN`, the precondition is not satisfied and the barrier continues to wait.

## Feature gate and version requirements

| Requirement | Value |
|-------------|-------|
| Feature gate | `BootstrapSynchronisation` (alpha, default off since v1.19) |
| Minimum ScyllaDB version | 2025.2 |

The feature gate must be enabled in the Operator's command-line flags. The Operator also checks the ScyllaDB container image version and only adds the init container when the version satisfies `≥ 2025.2.0`.

See [Feature gates](../reference/feature-gates.md) for instructions on enabling feature gates.

## Limitations

- **Node replacement** — Bootstrap synchronisation does not apply to nodes being added as replacements for dead nodes. You must verify the [replacement prerequisites](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/replace-dead-node.html#prerequisites) manually.
- **Manual multi-DC with ScyllaCluster** — When multiple `ScyllaCluster` resources are manually configured as a multi-datacenter cluster, node statuses can only be propagated within a single datacenter. Check node status in all datacenters manually before adding nodes.
- **Alpha status** — The feature is opt-in while ScyllaDB versions prior to 2025.2 are still supported by the Operator.

## Overriding the precondition

In scenarios where you need to bypass the barrier for a specific node or an entire datacenter, apply the force annotation:

::::{tabs}
:::{group-tab} Single node
```bash
kubectl annotate service <member-service-name> \
  scylla-operator.scylladb.com/force-proceed-to-bootstrap=true
```
:::
:::{group-tab} Entire datacenter
```bash
kubectl annotate scylladbdatacenter <name> \
  scylla-operator.scylladb.com/force-proceed-to-bootstrap=true
```
:::
::::

:::{caution}
Forcing bootstrap bypass disables the safety check. Use this only when you are certain the cluster is in a healthy state or when recovering from a known issue.
:::

## Related pages

- [Ignition](ignition.md) — the startup gating mechanism that runs after the bootstrap barrier.
- [Sidecar](sidecar.md) — the sidecar container that runs the StatusReporter.
- [Overview](overview.md) — component diagram and CRD list.
