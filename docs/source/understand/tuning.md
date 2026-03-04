# Tuning

ScyllaDB performs best when it has dedicated CPUs and is not interrupted by network or disk IRQs.
ScyllaDB Operator implements a two-level performance tuning system — **node-level** and **pod/container-level** — that automatically optimizes both the Kubernetes host and each ScyllaDB container.

Performance tuning is enabled when you create a [NodeConfig](../deploy-scylladb/configure-nodes.md) resource that targets your ScyllaDB nodes.

:::{warning}
We recommend testing performance tuning on a pre-production cluster first.
The underlying tuning changes to the host cannot be undone without rebooting the Kubernetes node.
:::

## Node-level tuning

Node-level tuning runs **once per node** when a NodeConfig is created or updated.
It configures the Kubernetes host for optimal ScyllaDB performance.

### What runs

The NodeConfig controller creates a privileged DaemonSet in the `scylla-operator-node-tuning` namespace.
This DaemonSet runs on every node that matches the NodeConfig's placement selectors.
Inside each DaemonSet pod, a controller creates two types of Jobs:

| Job type | What it does |
|---|---|
| **NodePerftune** | Runs the `perftune.py` script (from the ScyllaDB utilities image) with `--tune=system --tune-clock`. This tunes kernel parameters, clock settings, network device settings, disk devices, and spreads IRQs across available CPUs. |
| **NodeSysctls** | Applies the sysctls specified in the NodeConfig spec (e.g., `fs.aio-max-nr`, `fs.nr_open`, `vm.swappiness`) to the host via `sysctl --load`. |

Both jobs run as privileged containers with access to the host filesystem.

### Interaction with ScyllaDB startup

Node-level tuning does **not** block ScyllaDB startup.
ScyllaDB pods can start even if node-level tuning is still in progress.
However, container-level tuning (described below) does block startup and depends on node-level tuning completing first.

## Pod/container-level tuning

Container-level tuning runs **per ScyllaDB pod** on every pod creation or container restart.
It tunes the host specifically for the CPUs assigned to that ScyllaDB container.

### What runs

The same DaemonSet controller that creates node-level jobs also creates container-level jobs for each ScyllaDB pod:

| Job type | What it does |
|---|---|
| **ContainerPerftune** | Runs `perftune.py` with `--tune=net --tune=disks`, setting IRQ affinity masks so that network and disk IRQs are handled by CPUs **not** assigned to the ScyllaDB container. Also configures per-NIC and per-disk settings. |
| **ContainerResourceLimits** | Raises OS-level resource limits (such as the maximum number of open file descriptors) for the ScyllaDB process. |

ContainerPerftune runs only for pods with [Guaranteed QoS class](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed) — because only Guaranteed pods have pinned CPUs.
ContainerResourceLimits runs for all ScyllaDB pods regardless of QoS class.

### How container-level tuning blocks startup

Container-level tuning **blocks ScyllaDB startup via the [ignition](ignition.md) mechanism**.
The flow works as follows:

1. A ScyllaDB pod starts. Its main container polls for an ignition signal file before starting ScyllaDB.
2. The **NodeConfigPod controller** (running in the Operator deployment) detects the new pod and creates a ConfigMap in the pod's namespace containing:
   - The ScyllaDB container's ID.
   - The list of NodeConfigs matching this pod's node.
   - The list of **blocking NodeConfigs** — those where container-level tuning has not yet completed for this container ID.
3. The DaemonSet controller creates ContainerPerftune and ContainerResourceLimits Jobs for the new container.
4. As each job completes, the NodeConfig status is updated with the tuned container ID.
5. The NodeConfigPod controller re-evaluates and updates the ConfigMap. When no blocking NodeConfigs remain, the ConfigMap's blocking list becomes empty.
6. The **ignition controller** (running inside the ScyllaDB pod) sees the empty blocking list and matching container ID, and creates the ignition signal file.
7. ScyllaDB starts.

If the ScyllaDB container restarts, it gets a new container ID.
The old tuning results no longer apply, and the entire container-level tuning cycle repeats.

## CPU pinning

CPU pinning ensures that ScyllaDB uses dedicated CPU cores without sharing them with other processes.
This is achieved through the cooperation of three components:

1. **Kubelet static CPU manager policy**: The kubelet must be configured with `cpuManagerPolicy: static` so that it assigns exclusive CPUs to containers with Guaranteed QoS class.

2. **Guaranteed QoS class**: The ScyllaDB pod must have CPU and memory requests equal to limits for **all** containers in the pod (including the ScyllaDB Manager Agent sidecar). This makes the pod Guaranteed QoS.

3. **IRQ affinity via perftune**: The ContainerPerftune job reads which CPUs are assigned to the ScyllaDB container (via the kubelet PodResources API) and configures network and disk IRQs to be handled by the **remaining** CPUs on the node — ensuring that IRQ processing does not interrupt ScyllaDB.

:::{caution}
For CPU pinning to work, **all** containers in the ScyllaDB pod must have matching requests and limits.
This includes both the ScyllaDB container (`resources`) and the ScyllaDB Manager Agent (`agentResources`).
If any container lacks matching requests and limits, the pod will not receive Guaranteed QoS class and CPUs will not be pinned.
:::

For a step-by-step guide on configuring CPU pinning, see [CPU Pinning](../deploy-scylladb/configure-cpu-pinning.md).

## Tuning namespace and security

All node-level and container-level tuning Jobs run in the `scylla-operator-node-tuning` namespace.
This namespace is created and managed entirely by the Operator and is only accessible to cluster administrators.

The tuning system uses four dedicated ServiceAccounts with minimal privileges:
- `scylla-node-config` — for the DaemonSet pods.
- `perftune` — for NodePerftune and ContainerPerftune Jobs.
- `sysctl-buddy` — for NodeSysctls Jobs.
- `rlimits` — for ContainerResourceLimits Jobs.

The per-pod tuning ConfigMap is created in the **ScyllaDB pod's own namespace** (not the tuning namespace), owned by the pod, and is garbage-collected when the pod is deleted.
