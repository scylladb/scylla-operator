# pkg/internalapi

Internal types and constants shared across controllers. Three concerns: **condition naming**, **sidecar runtime coordination**, and **datacenter upgrade state**.

## Files

| File | Purpose |
|---|---|
| `conditions.go` | Condition name formats, reason constants, name-builder functions |
| `nodeconfig.go` | `SidecarRuntimeConfig` — operator↔sidecar coordination via ConfigMap |
| `statusreport.go` | `NodeStatusReport` — node status encoded into pod annotation |
| `upgrade.go` | `UpgradePhase`, `DatacenterUpgradeContext` — upgrade state machine payload |

---

## Condition naming

### Formats (use `fmt.Sprintf`)

```go
NodeAvailableConditionFormat   = "Node%sAvailable"
NodeProgressingConditionFormat = "Node%sProgressing"
NodeDegradedConditionFormat    = "Node%sDegraded"

NodeSetupAvailableConditionFormat   = "NodeSetup%sAvailable"
NodeSetupProgressingConditionFormat = "NodeSetup%sProgressing"
NodeSetupDegradedConditionFormat    = "NodeSetup%sDegraded"

NodeTuneAvailableConditionFormat   = "NodeTune%sAvailable"
NodeTuneProgressingConditionFormat = "NodeTune%sProgressing"
NodeTuneDegradedConditionFormat    = "NodeTune%sDegraded"
```

Example: `fmt.Sprintf(NodeSetupAvailableConditionFormat, "node-0")` → `"NodeSetupnode-0Available"`

### Datacenter condition builders

```go
// Pre-built package-level vars of type func(string) string
var MakeDatacenterAvailableCondition   func(dcName string) string
var MakeDatacenterProgressingCondition func(dcName string) string
var MakeDatacenterDegradedCondition    func(dcName string) string
// Pattern: "Datacenter<dcName><ConditionType>"

// Build your own for other condition types
MakeDatacenterConditionFunc(conditionType string) func(dcName string) string
```

### Controller/finalizer condition builders

```go
MakeKindControllerCondition(kind, conditionType string) string
// → "<kind>Controller<conditionType>"
// e.g. MakeKindControllerCondition("Service", "Progressing") → "ServiceControllerProgressing"

MakeKindFinalizerCondition(kind, conditionType string) string
// → "<kind>Finalizer<conditionType>"
```

### Reason constants

```go
AsExpectedReason        = "AsExpected"       // nominal state (Available=true, Degraded=false)
ErrorReason             = "Error"            // error state
ProgressingReason       = "Progressing"      // in progress
AwaitingConditionReason = "AwaitingCondition" // placeholder for missing child conditions
```

**Always use these constants for `metav1.Condition.Reason`** — do not hardcode strings.

### Condition aggregation chain

```
Child controllers set:
  NodeSetup<node>Available/Progressing/Degraded
  NodeTune<node>Available/Progressing/Degraded
    ↓
nodeconfig controller aggregates into:
  Node<node>Available/Progressing/Degraded
    ↓
NodeConfig top-level Available/Progressing/Degraded
```

When a child condition is missing, `nodeconfig/status.go` synthesizes an `Unknown` condition with `AwaitingConditionReason` so aggregation always has inputs.

---

## SidecarRuntimeConfig

Exchanged via ConfigMap key `naming.ScyllaRuntimeConfigKey`. Small JSON struct.

```go
type SidecarRuntimeConfig struct {
    ContainerID        string   `json:"containerID"`
    MatchingNodeConfigs []string `json:"matchingNodeConfigs"`
    BlockingNodeConfigs []string `json:"blockingNodeConfigs"`
}
```

- `ContainerID` — operator detects container restarts by comparing this value.
- `MatchingNodeConfigs` — NodeConfig names that currently apply to this pod.
- `BlockingNodeConfigs` — NodeConfig names blocking this pod from proceeding.

Read via `controllerhelpers.GetSidecarRuntimeConfigFromConfigMap(cm)`. Written by `pkg/controller/nodeconfigpod`.

---

## NodeStatusReport

Encoded into pod annotation `naming.NodeStatusReportAnnotation`.

```go
type NodeStatusReport struct {
    ObservedNodes []scyllav1alpha1.ObservedNodeStatus `json:"observedNodes,omitempty"`
    Error         *string                             `json:"error,omitempty"`
}

func (nsr *NodeStatusReport) Encode() ([]byte, error)
func (nsr *NodeStatusReport) Decode(reader io.Reader) error
```

Written by `pkg/controller/statusreport` (runs in sidecar), read by `pkg/controller/scylladbdatacenter`.

---

## DatacenterUpgradeContext

Upgrade state machine payload stored in a ConfigMap (`naming.UpgradeContextConfigMapName(sdc)`).

```go
type UpgradePhase string

const (
    PreHooksUpgradePhase    UpgradePhase = "PreHooks"
    RolloutInitUpgradePhase UpgradePhase = "RolloutInit"
    RolloutRunUpgradePhase  UpgradePhase = "RolloutRun"
    PostHooksUpgradePhase   UpgradePhase = "PostHooks"
)

type DatacenterUpgradeContext struct {
    State             UpgradePhase `json:"state"`
    FromVersion       string       `json:"fromVersion"`
    ToVersion         string       `json:"toVersion"`
    SystemSnapshotTag string       `json:"systemSnapshotTag"`
    DataSnapshotTag   string       `json:"dataSnapshotTag"`
}

func (uc *DatacenterUpgradeContext) Encode() ([]byte, error)
func (uc *DatacenterUpgradeContext) Decode(reader io.Reader) error
```

Lifecycle: `PreHooks → RolloutInit → RolloutRun → PostHooks` → ConfigMap deleted when upgrade is complete.

**Always use `Encode`/`Decode`** — do not hand-roll JSON for this type.

---

## Hard rules

- **Never hardcode condition type strings** — always use the format constants or builder functions from this package.
- **Never hardcode reason strings** — use the reason constants (`AsExpectedReason`, etc.).
- **`DatacenterUpgradeContext`** must be serialized via its `Encode`/`Decode` methods.
- **`SidecarRuntimeConfig`** is read via `controllerhelpers.GetSidecarRuntimeConfigFromConfigMap` — don't deserialize it manually.
