# Ignition

This page explains the ignition mechanism that gates ScyllaDB startup until all prerequisites are satisfied.

## Why ignition exists

ScyllaDB must not start until:

- **Node-level tuning is complete** — performance settings (IRQ affinity, sysctl values, CPU frequency governors) must be applied before ScyllaDB begins benchmarking IO or pinning threads. Starting too early produces incorrect IO calibration and suboptimal performance.
- **Network identity is assigned** — the pod must have an IP address, and if LoadBalancer broadcasting is used, the load balancer must have provisioned an ingress address.
- **The container is ready** — the ScyllaDB container must be running (container ID assigned) so that per-container tuning can target it.

Without ignition gating, ScyllaDB could start before tuning DaemonSets finish their work or before the cloud provider assigns a load balancer IP, leading to misconfiguration that is difficult to correct without a restart.

## Signal-file mechanism

Ignition uses a simple file-based signal on the shared emptyDir volume mounted at `/mnt/shared`:

1. The `scylladb-ignition` sidecar container runs the ignition controller, which continuously evaluates prerequisites.
2. When **all** prerequisites are met, the controller creates the file `/mnt/shared/ignition.done`.
3. The `scylla` container's entrypoint polls for this file in a shell loop (`until [[ -f "/mnt/shared/ignition.done" ]]; do sleep 1; done`). Once the file appears, it execs into the sidecar binary that configures and starts ScyllaDB.
4. The ScyllaDB Manager Agent container (when present) uses the same wait loop, ensuring the agent does not start before ScyllaDB is configured.

## Prerequisites evaluated

The ignition controller checks the following conditions. **All** must be true before the signal file is created:

| # | Condition | Why |
|---|-----------|-----|
| 1 | **LoadBalancer ingress available** (only when broadcast address type is `ServiceLoadBalancerIngress`) | The broadcast address cannot be resolved until the cloud provider assigns an external IP or hostname to the member Service. |
| 2 | **Pod has an IP** (`status.podIP` is set) | ScyllaDB needs a listen address. The sidecar cannot resolve `PodIP`-type broadcast addresses without it. |
| 3 | **ScyllaDB container has a container ID** (`containerStatuses[].containerID` is set) | Per-container tuning (CPU pinning, cgroup settings) targets a specific container ID. Tuning cannot complete until the container exists. |
| 4 | **Tuning ConfigMap exists** with matching container ID | A ConfigMap labeled for this pod must exist, created by the tuning infrastructure. Its `ContainerID` field must match the current ScyllaDB container ID, confirming that tuning ran for this specific container instance. |
| 5 | **No blocking NodeConfigs** in the tuning ConfigMap | The ConfigMap must report that all `NodeConfig` resources have completed tuning. If any are still in progress, ignition waits. |

## Cleanup on shutdown

The `scylla` container's `preStop` hook removes the signal file:

```
rm -f /mnt/shared/ignition.done
```

This ensures that if the container restarts (due to a crash or rolling update), the ignition controller must re-evaluate all prerequisites before ScyllaDB starts again. This is important because a container restart assigns a new container ID, invalidating previous tuning results.

## Force override

For debugging or recovery scenarios, the annotation `internal.scylla-operator.scylladb.com/ignition-override` on the node's member Service can override the ignition decision:

| Value | Effect |
|-------|--------|
| `"true"` | Ignition proceeds immediately, bypassing all prerequisite checks. |
| `"false"` | Ignition is blocked indefinitely, regardless of prerequisite status. |

:::{caution}
The force override is an internal mechanism intended for debugging. Forcing ignition to `true` while tuning is incomplete results in degraded performance. Forcing it to `false` prevents the ScyllaDB node from starting.
:::

## Readiness probe

The ignition container exposes a readiness endpoint at `/readyz` on port 42081. It returns HTTP 200 only after the signal file has been created. This allows external tools and monitoring to determine whether a pod has passed the ignition gate.

## Related pages

- [Sidecar and pod anatomy](sidecar.md) — the full container list and how they coordinate.
- [Tuning](tuning.md) — the node and container tuning that must complete before ignition.
- [Bootstrap synchronisation](bootstrap-sync.md) — the init container barrier that runs before ignition.
- [Security](security.md) — TLS certificate provisioning that feeds into ignition prerequisites.
