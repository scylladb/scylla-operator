---
name: must-gather-investigation
description: Investigate failed e2e tests from Ginkgo JSON reports and must-gather artifacts, systematically analyzing logs, events, and resource states to identify root causes.
metadata:
  audience: maintainers
---

# Must-Gather Investigation

You are a Kubernetes operator e2e test failure investigator. Your goal is to analyze test artifacts from a failed test run, reconstruct the sequence of events, and identify the root cause.

## Inputs

The user provides:
- A path to the root directory containing `e2e.json` (a Ginkgo JSON report) and associated artifacts. All artifact paths below are relative to this root directory. You should assume you are already in this directory.
- Optionally, a specific test name to investigate

## Phase 1: Extract Failed Tests from e2e.json

The `e2e.json` file is a Ginkgo JSON report. Use `jq` to navigate it.

### Key jq paths

**List all failed tests:**
```bash
jq -r '.[] | .SpecReports[] | select(.State == "failed") | .ContainerHierarchyTexts + [.LeafNodeText] | join(" > ")' e2e.json
```

**Extract failure details for a specific failed test:**
```bash
jq '.[] | .SpecReports[] | select(.State == "failed") | {
  name: (.ContainerHierarchyTexts + [.LeafNodeText] | join(" > ")),
  state: .State,
  startTime: .StartTime,
  endTime: .EndTime,
  runTime: .RunTime,
  failureMessage: .Failure.Message,
  failureLocation: (.Failure.Location.FileName + ":" + (.Failure.Location.LineNumber | tostring)),
  ginkgoOutput: .CapturedGinkgoWriterOutput
}' e2e.json
```

**Extract SpecEvents timeline for a failed test:**
```bash
jq '.[] | .SpecReports[] | select(.State == "failed") | .SpecEvents[] | {type: .SpecEventType, message: .Message, duration: .Duration, codeLocation: (.CodeLocation.FileName + ":" + (.CodeLocation.LineNumber | tostring))}' e2e.json
```

### Key fields in a SpecReport

| Field | Description |
|-------|-------------|
| `.State` | `"passed"`, `"failed"`, `"skipped"`, `"pending"` |
| `.ContainerHierarchyTexts` | Array of Describe/Context block names (outermost first) |
| `.LeafNodeText` | The `It` block name |
| `.StartTime` / `.EndTime` | ISO 8601 timestamps |
| `.RunTime` | Duration in nanoseconds |
| `.Failure.Message` | The assertion error message |
| `.Failure.Location` | `{FileName, LineNumber}` of the failing assertion |
| `.CapturedGinkgoWriterOutput` | All `GinkgoWriter` output during the test (contains namespace names, resource names, log lines) |
| `.SpecEvents[]` | Timeline of `By()` steps, `DeferCleanup` calls, etc. Each has `.SpecEventType`, `.Message`, `.Duration`, `.CodeLocation` |

### Extracting the test namespace

The test namespace is typically logged in `CapturedGinkgoWriterOutput`. Search for patterns like `e2e-test-*` or look for lines containing `"namespace"` or `"ns"`.

```bash
jq -r '.[] | .SpecReports[] | select(.State == "failed") | .CapturedGinkgoWriterOutput' e2e.json | grep -oE 'e2e-test-[a-z0-9-]+'
```

## Phase 2: Locate the Test Source Code

Search `test/e2e/` in the repository for the test name (the `LeafNodeText` or a unique substring from it):

```bash
grep -rn "LEAF_NODE_TEXT_SUBSTRING" test/e2e/
```

Read the test to understand:
- What resources it creates (ScyllaCluster, ScyllaDBDatacenter, etc.)
- What it waits for (rollout, conditions, specific states)
- What it asserts (cleanup jobs completed, connections succeed, etc.)
- Timeout values and polling intervals
- Any `Eventually`/`Consistently` blocks — these are where timeouts cause failures

## Phase 3: Examine Test Namespace Artifacts

Navigate to `e2e/cluster/namespaces/<test-namespace>/`. This contains the state of all resources in the test namespace at the time the must-gather was collected (typically after the test failed, during namespace teardown).

### Priority order for examination

1. **Events** (`events.events.k8s.io/*.yaml`): Chronological record of what happened. Look for warnings, errors, and unusual sequences.

2. **ScyllaCluster / ScyllaDBDatacenter status** (`scyllaclusters.scylla.scylladb.com/*.yaml` or `scylladbdatacenters.scylla.scylladb.com/*.yaml`): Check `.status.conditions` — especially `Available`, `Progressing`, `Degraded`. The `reason` and `message` fields explain why a condition is set.

3. **Pod status and logs** (`pods/<pod-name>/`):
   - `<container>.current` — current container logs
   - `<container>.terminated` — logs from a previous container instance (if it restarted)
   - `<pod-name>.yaml` — full pod spec and status, including conditions, container states, restart counts
   - `df.log` — disk usage (for Scylla data pods)
   - `nodetool-status.log` — Scylla cluster membership
   - `nodetool-gossipinfo.log` — Scylla gossip state

4. **Jobs** (`jobs/*.yaml`): Check `.status` for `completionTime`, `conditions`, `ready`, `active`, `failed` counts. Compare job UIDs with pod `controller-uid` labels to verify ownership.

5. **Services** (`services/*.yaml`): Check annotations — `CurrentTokenRingHash`, `LastCleanedUpTokenRingHash`, `HostID`, etc. Compare across nodes.

6. **StatefulSets** (`statefulsets.apps/*.yaml`): Check `.status.readyReplicas`, `.status.currentRevision`, `.status.updateRevision`.

7. **Other resources**: ConfigMaps, Secrets, PVCs, Ingresses, EndpointSlices — as relevant to the test.

## Phase 4: Examine Operator Logs

Operator logs are at `must-gather/cluster/namespaces/scylla-operator/pods/<operator-pod>/scylla-operator.current`.

These are structured JSON logs (one JSON object per line). Key fields:
- `"ts"` — timestamp
- `"msg"` — log message
- `"controller"` — which controller emitted the log
- `"namespace"` / `"name"` — the resource being reconciled
- `"err"` — error details

Filter by the test namespace to find relevant reconciliation activity:
```bash
grep '<test-namespace>' scylla-operator.current
```

Look for:
- Reconciliation start/end and duration
- Error messages or warnings
- Resource creation, update, deletion events
- Status condition changes
- Queuing and re-queuing patterns

## Phase 5: Examine Infrastructure Logs

Depending on the test, check logs from infrastructure components:

- **HAProxy ingress** (`must-gather/cluster/namespaces/haproxy-ingress/`): Backend configuration, reload events, connection logs
- **Scylla Manager** (`must-gather/cluster/namespaces/scylla-manager/`): Task scheduling, repair/backup operations
- **cert-manager**: Certificate issuance and renewal
- **Scylla Manager Agent** (sidecar in Scylla pods, `scylla-manager-agent.current`): API calls, health checks

## Phase 6: Timeline Reconstruction

Build a chronological timeline from all log sources, correlating timestamps. Include:
- Pod lifecycle events (created, scheduled, started, ready)
- Controller reconciliation actions
- Resource state changes
- The test's own actions (from `CapturedGinkgoWriterOutput` and `SpecEvents`)
- Infrastructure events (reloads, connection attempts)

This timeline is the core artifact for identifying the root cause. It should make the causal chain visible.

## Phase 7: Root Cause Analysis

Trace the causal chain from the failure backward:
1. What assertion failed, and what was the actual vs expected state?
2. Why was the resource/condition in that state?
3. What controller/component was responsible for getting it to the expected state?
4. What prevented it from doing so?
5. Was it a timing issue, a logic bug, an infrastructure failure, or a test design issue?

## Artifact Structure Reference

```
./
├── e2e.json                          # Ginkgo JSON test report
├── junit.e2e.xml                     # JUnit XML test report
├── deploy/                           # Deployment manifests used
│   ├── operator/
│   ├── manager/
│   ├── prometheus-operator/
│   └── haproxy-ingress/
├── e2e/cluster/                      # Resources collected during test execution
│   ├── cluster-scoped/               # Cluster-wide resources
│   │   ├── nodes/
│   │   ├── persistentvolumes/
│   │   └── ...
│   └── namespaces/
│       └── <test-namespace>/         # Test-specific namespace
│           ├── pods/
│           │   └── <pod-name>/
│           │       ├── <container>.current          # Container logs
│           │       ├── <container>.terminated       # Previous container logs
│           │       ├── df.log                       # Disk usage
│           │       ├── nodetool-gossipinfo.log      # Scylla gossip info
│           │       └── nodetool-status.log          # Scylla cluster status
│           ├── events.events.k8s.io/
│           ├── statefulsets.apps/
│           ├── jobs/
│           ├── services/
│           ├── configmaps/
│           ├── secrets/
│           ├── scyllaclusters.scylla.scylladb.com/
│           └── scylladbdatacenters.scylla.scylladb.com/
└── must-gather/cluster/              # Must-gather output
    ├── cluster-scoped/
    └── namespaces/
        ├── scylla-operator/
        │   ├── pods/
        │   │   └── <operator-pod>/
        │   │       └── scylla-operator.current      # Operator logs
        │   ├── events.events.k8s.io/
        │   └── ...
        ├── scylla-manager/
        ├── haproxy-ingress/
        └── ...
```

## Important Notes

- Always reference specific file paths, line numbers, and timestamps when citing evidence.
- Focus on the causal chain — what led to what.
- Consider whether the issue is in the test, the operator, or the infrastructure.
- If you need more information from a file, say what you need and why.
- This skill provides investigation methodology only. The calling agent determines the output format.
