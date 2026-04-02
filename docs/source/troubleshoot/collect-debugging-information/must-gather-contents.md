# must-gather contents

Reference for the diagnostic files captured by `must-gather` and how to interpret them.

## Output directory structure

```
must-gather-<timestamp>/
├── namespaces/
│   └── <namespace>/
│       ├── pods/
│       │   └── <pod-name>/
│       │       ├── <container>/
│       │       │   ├── logs/
│       │       │   │   ├── current.log
│       │       │   │   └── previous.log
│       │       │   ├── nodetool-status.log
│       │       │   ├── nodetool-status.log.stderr
│       │       │   ├── nodetool-gossipinfo.log
│       │       │   ├── df.log
│       │       │   ├── io_properties.yaml
│       │       │   └── scylla-rlimits.log
│       │       └── pod.yaml
│       ├── scyllaclusters/
│       ├── statefulsets/
│       ├── services/
│       └── ...
├── cluster-scoped/
│   ├── nodes/
│   ├── crds/
│   └── ...
└── ...
```

## Per-node diagnostic files

These files are collected from inside each ScyllaDB pod and NodeConfig pod.

### nodetool-status.log

Cluster topology snapshot — which nodes are up or down, their load, host IDs, and rack placement.

```
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens  Owns  Host ID                               Rack
UN  10.244.1.5   1.2 GB     256     ?     a1b2c3d4-e5f6-7890-abcd-ef1234567890  us-east-1a
DN  10.244.2.6   1.1 GB     256     ?     b2c3d4e5-f6a7-8901-bcde-f12345678901  us-east-1a
```

**How to read it:**
- `UN` = Up/Normal — healthy.
- `DN` = Down/Normal — node is unreachable.
- `UJ` = Up/Joining — node is streaming data to join.
- Compare across pods — if different pods show different views, there may be a gossip partition.

### nodetool-gossipinfo.log

Gossip protocol state per node — heartbeat generation, status, schema version, host ID, tokens, and addresses.

**Look for:**
- Different `SCHEMA` values across nodes → schema disagreement.
- `STATUS` other than `NORMAL` → node in transition.
- Stale `HEARTBEAT` generation → possible zombie node.

### df.log

Disk space usage (`df -h`) for all mounted filesystems inside the container.

**Look for:**
- Data volume usage approaching 100% — ScyllaDB may stop accepting writes.
- Commitlog volume usage — if separate from data.

### io_properties.yaml

ScyllaDB I/O properties configuration showing disk bandwidth and IOPS parameters.

**Look for:**
- File missing → I/O tuning was not applied; see [Configuring io_properties](../../operate/configure-io-properties.md).
- Values that seem incorrect for the underlying storage type.

### scylla-rlimits.log

OS resource limits of the running ScyllaDB process (`prlimit --pid=$(pidof scylla)`).

**Key limits:**

| Limit | Field | What to check |
|---|---|---|
| Open files | `NOFILE` | Should be at least 200000 for production; low values cause "too many open files" errors |
| Address space | `AS` | Should be unlimited |
| Core dump size | `CORE` | Should be unlimited if coredumps are configured |
| Locked memory | `MEMLOCK` | Should be unlimited for ScyllaDB |

### kubelet-cpu_manager_state.log

Collected from **NodeConfig pods** (not ScyllaDB pods).
Shows the kubelet CPU Manager state — which CPUs are exclusively assigned to which containers.

**Look for:**
- ScyllaDB containers should appear with dedicated CPU sets.
- If the file is empty or shows `"policyName":"none"`, the kubelet is not configured with static CPU policy.
- See [Configure CPU pinning](../../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md).

## Pod logs

Logs are collected for all containers (init, regular, and ephemeral) in three variants:

| File | Description |
|---|---|
| `current.log` | Logs from the currently running container instance |
| `previous.log` | Logs from the previous container instance (if it restarted) |

Logs are size-limited to prevent excessive archive sizes.

When stderr output is present, it is captured separately in `*.stderr` files alongside the main output file (for example, `nodetool-status.log.stderr`).

## Related pages

- [must-gather](must-gather.md)
- [System tables](system-tables.md)
- [Investigating restarts](../investigate-restarts.md)
