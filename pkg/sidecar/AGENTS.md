# pkg/sidecar

The sidecar runs **inside every Scylla pod** alongside the ScyllaDB binary. It is a separate process from the operator. Its responsibilities:

1. **Prepare Scylla configuration** — merge `scylla.yaml`, snitch properties, IO-tune cache symlink.
2. **Construct runtime arguments** — seeds, smp, cpuset, broadcast addresses.
3. **Launch Scylla** — `exec` the Scylla entrypoint with computed args.
4. **Mirror node state to Kubernetes** — write HostID and token-ring hash into Service annotations; handle decommission via Service labels.
5. **Report node status** — poll the local Scylla HTTP API and store results in pod annotations.

## Files

| File | Purpose |
|---|---|
| `pkg/sidecar/identity/member.go` | `Member` type — pod/service identity, broadcast addresses, seed selection |
| `pkg/sidecar/config/config.go` | `ScyllaConfig` — YAML merge, IO-tune cache, entrypoint Cmd construction |
| `pkg/cmd/operator/sidecar/sidecar.go` | CLI entrypoint: flags, informers, controller + status reporter startup |
| `pkg/cmd/operator/sidecar/statusreporter.go` | `StatusReporter` — polls local Scylla API, enqueues status-report work |
| `pkg/controller/sidecar/controller.go` | Controller: watches the node Service, drives annotation updates |
| `pkg/controller/sidecar/sync.go` | Sync logic: `syncAnnotations`, `decommissionNode` |

## Key types

### `identity.Member`

Encapsulates the pod's Scylla identity.

```go
type Member struct {
    Name, Namespace, Rack, Datacenter, Cluster string
    BroadcastRPCAddress, BroadcastAddress string
    // ...
}

// Compute seeds from cluster Service/Pod listing
func (m *Member) GetSeeds(ctx, coreClient, externalSeeds) ([]string, error)
```

Seed selection logic (in priority order): first UN node in the same datacenter → any UN node → external seeds fallback.

### `config.ScyllaConfig`

```go
func NewScyllaConfig(member *identity.Member, kubeClient, cpuCount, externalSeeds) *ScyllaConfig

// Orchestrates all setup steps and returns a ready-to-start exec.Cmd for Scylla
func (s *ScyllaConfig) Setup(ctx) (*exec.Cmd, error)
```

### Sidecar controller

Watches a single Service (field-selected by `metadata.name`). Reads from the local Scylla HTTP API via `scyllaclient` and writes:
- `naming.HostIDAnnotation` on the Service
- `naming.CurrentTokenRingHashAnnotation` on the Service
- `naming.DecommissionedLabel` on the Service (when decommission completes)

## Launch path

```
pkg/cmd/operator/sidecar/sidecar.go → Run()
  ↓
identity.Member constructed from Service + Pod
  ↓
config.NewScyllaConfig(...).Setup(ctx) → exec.Cmd
  ↓
Start Scylla process (Pdeathsig = SIGKILL)
  + start sidecar controller
  + start status reporter
```

## Configuration sources (merge order, last wins)

1. **Base** — default `/etc/scylla/scylla.yaml` from the image
2. **Operator-managed overrides** — `naming.ScyllaManagedConfigPath`
3. **User ConfigMap** — mounted at `naming.ScyllaConfigDirName/naming.ScyllaConfigName`

For the snitch (`cassandra-rackdc.properties`): operator values for `dc`/`rack`/`prefer_local` always win; only `dc_suffix` is taken from the user config.

Runtime arguments (seeds, smp, cpuset, addresses) are constructed by the sidecar and **cannot** be overridden by user-provided arguments (`mergeArguments` enforces this).

## Files the sidecar mutates

**Local filesystem:**
- `/etc/scylla/scylla.yaml` — overwritten with merged config
- `/etc/scylla/cassandra-rackdc.properties` — overwritten with merged snitch config
- `/etc/scylla.d/<io-properties>` — symlink → `naming.DataDir/<io-properties>` (IO-tune cache)

**Kubernetes API:**
- Service annotations: `naming.HostIDAnnotation`, `naming.CurrentTokenRingHashAnnotation`
- Service label: `naming.DecommissionedLabel` (set to `"true"` on decommission completion)

## Decommission flow

1. Operator sets `naming.DecommissionLabel` on the Service.
2. Sidecar controller detects the label and calls `scyllaClient.Decommission(ctx, host)`.
3. If the node is in `OperationalModeDrained`, restarts Scylla via `supervisorctl restart scylla`.
4. Polls `OperationMode` until decommission is complete.
5. Sets `naming.DecommissionedLabel = "true"` on the Service.

## CLI flags (important ones)

| Flag | Purpose |
|---|---|
| `--service-name` | Name of the pod's Kubernetes Service |
| `--cpu-count` | Number of CPUs to pass as `smp` to Scylla |
| `--external-seeds` | External seed list (fallback when cluster is empty) |
| `--nodes-broadcast-address-type` | How to pick the node broadcast address |
| `--clients-broadcast-address-type` | How to pick the client broadcast address |
| `--ip-family` | IPv4 or IPv6 |
| `--status-report-interval` | How often to poll Scylla for status |

## Separation from the operator

The sidecar and operator are **separate processes**. Do not import sidecar packages from operator controller packages. Communication happens via:
- Kubernetes Service annotations/labels (operator ↔ sidecar)
- ConfigMaps (`SidecarRuntimeConfig` — see `pkg/internalapi`)
- Pod annotations (`NodeStatusReport` — see `pkg/internalapi`)

## Hard rules

- **Never import `pkg/controller/sidecar` from operator controllers.** The sidecar controller is sidecar-only.
- **Never hardcode paths** — use constants from `pkg/naming` (e.g., `naming.ScyllaManagedConfigPath`, `naming.DataDir`).
- **CPU set validation** — `setupEntrypoint` reads `/proc/1/status` to derive the allowed CPU list and warns on mismatches. Do not bypass this.
