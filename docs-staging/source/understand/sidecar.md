# Sidecar and pod anatomy

This page describes every container that runs inside a ScyllaDB pod, how they coordinate through shared volumes, and what the sidecar subcommand does.

## Container overview

A ScyllaDB pod contains init containers that run sequentially before the main workload, followed by long-running containers that run concurrently.

### Init containers (in order)

| # | Name | Image | Purpose | Conditional |
|---|------|-------|---------|-------------|
| 1 | `scylla-operator-binary-injector` | Operator | Copies the `scylla-operator` binary into the `/mnt/shared` volume so that other containers can use it without bundling the Operator image. | Always present |
| 2 | `sysctl-buddy` | Operator | Applies sysctl settings from a pod annotation. Used during migration from legacy tuning. | Only when the sysctls annotation is set |
| 3 | `scylladb-bootstrap-barrier` | ScyllaDB | Blocks until all nodes in the cluster report each other as UP, preventing a new node from bootstrapping into an unhealthy cluster. See [Bootstrap synchronisation](bootstrap-sync.md). | Only when the `BootstrapSynchronisation` feature gate is enabled and the ScyllaDB version is ≥ 2025.2 |

### Long-running containers

| # | Name | Image | Purpose | Conditional |
|---|------|-------|---------|-------------|
| 1 | `scylla` | ScyllaDB | The main container. Waits for the ignition signal, then execs the sidecar binary which configures and starts ScyllaDB. | Always present |
| 2 | `scylladb-api-status-probe` | Operator | Translates ScyllaDB's native HTTP API status into Kubernetes health-check endpoints (`/healthz`, `/readyz`) on port 8080. | Always present |
| 3 | `scylladb-ignition` | Operator | Evaluates startup prerequisites (tuning done, IPs assigned, container running) and creates the ignition signal file when all are met. See [Ignition](ignition.md). | Always present |
| 4 | `scylla-manager-agent` | Manager Agent | Runs the ScyllaDB Manager Agent for backup, repair, and health-check operations. Waits for the ignition signal before starting. | Only when Manager integration is configured |

## Shared volumes

Containers coordinate through volumes mounted into the pod:

| Volume | Mount path | Purpose |
|--------|-----------|---------|
| `shared` (emptyDir) | `/mnt/shared` | Carries the Operator binary (injected by init container 1) and the ignition signal file (`/mnt/shared/ignition.done`). Shared by all containers. |
| Data PVC | `/var/lib/scylla` | Persistent storage for ScyllaDB data files. Mounted read-write in the main container, read-only in the bootstrap barrier. |
| Managed ScyllaDB config (ConfigMap) | `/var/run/configmaps/scylla-operator.scylladb.com/scylladb/managed-config/scylla.yaml` | Operator-generated `scylla.yaml` with cluster name, snitch, TLS, and other managed settings. |
| User ScyllaDB config (ConfigMap) | `/var/run/configmaps/scylla-operator.scylladb.com/scylladb/config/scylla.yaml` | User-provided `scylla.yaml` overrides. |
| Rack config (ConfigMap) | Snitch properties path | Datacenter and rack assignment for the gossip endpoint snitch. |
| Agent auth token (Secret) | Agent config path | Manager Agent authentication token. |

Additional TLS-related volumes (serving certificates, client CA, admin user credentials, Alternator certificates) are mounted when the `AutomaticTLSCertificates` feature gate is enabled or when Alternator is configured.

## The ignition signal file

The `/mnt/shared/ignition.done` file is the coordination point between the ignition sidecar and the main container:

1. The `scylla` container's entrypoint loops, sleeping until the file appears.
2. The `scylladb-ignition` container evaluates prerequisites and creates the file when all conditions are satisfied.
3. On container termination, the `scylla` container's shutdown trap removes the file so that on restart the ignition sidecar must re-evaluate.
4. The Manager Agent container uses the same wait loop before starting the agent process.

This mechanism ensures that ScyllaDB never starts before node-level tuning is complete and network addresses are assigned. See [Ignition](ignition.md) for the full list of prerequisites.

## The sidecar subcommand

When the ignition signal file appears, the `scylla` container execs into `/mnt/shared/scylla-operator sidecar`. This subcommand runs inside the main container for the lifetime of the pod and is responsible for:

### 1. Resolving the node identity

The sidecar creates a `Member` identity by querying the pod's own Service and pod status. It resolves broadcast addresses according to the configured `broadcastOptions`:

- `PodIP` → reads `status.podIP` (or selects by IP family from `status.podIPs`).
- `ServiceClusterIP` → reads `spec.clusterIP` from the member Service.
- `ServiceLoadBalancerIngress` → reads the first entry in `status.loadBalancer.ingress`.

Two addresses are resolved independently: one for inter-node communication (`broadcast_address`) and one for CQL clients (`broadcast_rpc_address`).

### 2. Configuring ScyllaDB

The sidecar merges configuration from multiple sources:

1. **Default `scylla.yaml`** shipped with the ScyllaDB image.
2. **Operator-managed config** from the managed ConfigMap — sets `cluster_name`, `rpc_address`, `endpoint_snitch`, TLS options, and Alternator settings.
3. **User-provided config** from the user's ConfigMap — overlaid on top, but operator-managed keys take precedence.

The merged configuration is written to disk. The sidecar also:

- Sets up **rack and datacenter properties** for the gossip endpoint snitch.
- Copies pre-computed **IO properties** if available.
- Configures **CPU pinning** arguments when the pod has a Guaranteed QoS class and the kubelet uses a static CPU manager policy.

### 3. Starting ScyllaDB

The sidecar builds the ScyllaDB command line (`/usr/bin/scylla`) with arguments including:

- `--broadcast-address` and `--broadcast-rpc-address` (from the resolved identity).
- `--listen-address=0.0.0.0` (or `::` for IPv6).
- `--developer-mode` flag.
- Any additional ScyllaDB arguments from `spec.additionalScyllaDBArguments`.

ScyllaDB is started as a child process with `Pdeathsig` set so that it terminates if the sidecar exits.

### 4. Running internal controllers

While ScyllaDB runs, the sidecar starts two controllers concurrently:

#### SidecarController

Watches the node's own member Service and keeps it in sync with ScyllaDB's runtime state:

- Queries ScyllaDB for the node's **host ID** and **token ring membership**.
- Updates the Service annotations with the current host ID and token ring hash. These annotations are consumed by the datacenter controller to detect token ring changes (for [automatic data cleanup](automatic-data-cleanup.md)) and to track node identity.
- Drives **decommission**: when the Service carries the decommission label, the SidecarController executes `nodetool decommission`, monitoring the node's state transitions (`NORMAL` → `LEAVING` → `DECOMMISSIONED`).

#### StatusReporter

Periodically polls the ScyllaDB storage service API and writes the node's gossip view (which other nodes are `UP` or `DOWN`) as an annotation on the pod. This annotation is consumed by the internal datacenter controller to build `ScyllaDBDatacenterNodesStatusReport` resources (an internal CRD used for coordinating operations) that feed into [bootstrap synchronisation](bootstrap-sync.md).

## The probe sidecar

The `scylladb-api-status-probe` container runs an HTTP server on port 8080 that translates ScyllaDB's native HTTP API into Kubernetes probe responses:

| Endpoint | Kubernetes probe | What it checks |
|----------|-----------------|----------------|
| `/healthz` | Startup and liveness | ScyllaDB process is alive and responsive. |
| `/readyz` | Readiness | ScyllaDB node is ready to serve traffic. |

The `scylla` container's health probes point to these endpoints rather than directly to ScyllaDB, because ScyllaDB's native API is not designed to return the HTTP status codes that Kubernetes expects.

## Shutdown sequence

When a pod is terminated:

1. The `scylla` container's `preStop` hook runs `nodetool drain` to flush memtables and stop accepting new connections.
2. The container's trap sends SIGTERM to all child processes (including ScyllaDB) and waits for them to exit.
3. The trap removes `/mnt/shared/ignition.done` so that if the container restarts, ignition must re-evaluate prerequisites.

## Related pages

- [Ignition](ignition.md) — detailed startup prerequisites and the ignition controller.
- [Bootstrap synchronisation](bootstrap-sync.md) — the barrier init container.
- [Tuning](tuning.md) — how node tuning feeds into ignition readiness.
- [Networking](networking.md) — broadcast address types and Service model.
- [Security](security.md) — TLS certificates mounted into pod containers and Manager Agent authentication.
- [StatefulSets and racks](statefulsets-and-racks.md) — pod identity, scaling, and decommission flow.
