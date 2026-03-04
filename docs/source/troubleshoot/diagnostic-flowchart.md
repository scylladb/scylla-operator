# Diagnostic flowchart

Use this decision tree to narrow down whether you are dealing with a **Kubernetes issue**, an **Operator issue**, or a **ScyllaDB issue**, then follow the right troubleshooting guide.

## Step 1: Identify the failure layer

```
Is the ScyllaDB Operator running?
├── No  → Kubernetes or Operator issue → "Operator not running"
└── Yes
    ├── Are ScyllaDB pods being created?
    │   ├── No  → Operator or Kubernetes issue → "Pods not starting"
    │   └── Yes
    │       ├── Are pods restarting?
    │       │   └── Yes → ScyllaDB or Kubernetes issue → "Pod restarts"
    │       ├── Is the cluster healthy (all nodes UN)?
    │       │   ├── No  → ScyllaDB or Operator issue → "Cluster degraded"
    │       │   └── Yes
    │       │       ├── Can clients connect?
    │       │       │   ├── No  → Kubernetes networking or ScyllaDB issue → "Connection failures"
    │       │       │   └── Yes → ScyllaDB issue → "Performance issues"
    │       └── Is an upgrade or rollout stuck?
    │           └── Yes → Operator or ScyllaDB issue → "Upgrade failures"
    └── Is a node replace stuck?
        └── Yes → Operator issue → "Failed replace"
```

As a rule of thumb:
- **Kubernetes issues** affect pod scheduling, networking, and resource allocation. Symptoms include `Pending` pods, `FailedScheduling` events, and service endpoint issues.
- **Operator issues** affect cluster state reconciliation. Symptoms include stuck rollouts, conditions showing `Progressing=True` or `Degraded=True`, and incorrect StatefulSet state.
- **ScyllaDB issues** affect the database itself. Symptoms include `DN` nodes in `nodetool status`, CQL errors, and performance degradation.

## Step 2: Follow the relevant guide

### Operator not running

Check the Operator deployment:

```bash
kubectl -n scylla-operator get deploy scylla-operator
kubectl -n scylla-operator get pods
kubectl -n scylla-operator logs deploy/scylla-operator
```

Common causes:
- Webhook certificate issues — see [Installation troubleshooting](troubleshoot-installation.md).
- CRD not installed — verify with `kubectl get crd scyllaclusters.scylla.scylladb.com`.
- Resource limits too low — check pod events with `kubectl -n scylla-operator describe pod <pod-name>`.

### Pods not starting

Identify the pod state:

```bash
kubectl -n scylla get pods
kubectl -n scylla describe pod <pod-name>
```

| Pod state | Likely cause | Guide |
|---|---|---|
| `Pending` | Scheduling failure (resources, affinity, taints) | [Node not starting](diagnose-node-not-starting.md) |
| `Init:*` | Init container stuck (bootstrap barrier, sysctl) | [Node not starting](diagnose-node-not-starting.md) |
| `CrashLoopBackOff` | ScyllaDB crashing on startup | [Node not starting](diagnose-node-not-starting.md) |
| `Running` but not `Ready` | Readiness probe failure | [Node not starting](diagnose-node-not-starting.md) |

### Cluster degraded

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool status
```

| Node state | Meaning | Guide |
|---|---|---|
| `DN` | Down / Normal — node is unreachable | [Cluster health](check-cluster-health.md) |
| `?N` | Unknown / Normal — gossip issue | [Cluster health](check-cluster-health.md) |
| `UJ` | Up / Joining — node joining (may be stuck) | [Node not starting](diagnose-node-not-starting.md) |
| `UL` | Up / Leaving — decommission in progress | [Cluster health](check-cluster-health.md) |

### Connection failures

```bash
# Verify service exists and has endpoints
kubectl -n scylla get svc <cluster-name>-client
kubectl -n scylla get endpoints <cluster-name>-client

# Test connectivity from inside the cluster
kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -- \
  <cluster-name>-client.<namespace>.svc.cluster.local 9042 \
  -e "SELECT cluster_name FROM system.local;"
```

Common causes:
- Service has no endpoints — cluster is not ready.
- Network policy blocking traffic.
- Authentication required but not configured in client.
- TLS required but client not using TLS.
- LoadBalancer service has no external IP — check with `kubectl -n scylla get svc`.
- DNS resolution failing — verify pod DNS with `kubectl run -it --rm dns-test --image=busybox --restart=Never -- nslookup <cluster-name>-client.<namespace>.svc.cluster.local`.
- For IPv6 issues, see [Troubleshoot IPv6](../set-up-networking/ipv6/troubleshooting.md).
- For external access issues, see [Configure external access](../connect-your-app/configure-external-access.md).

### Upgrade failures

Check the rollout status:

```bash
# ScyllaCluster conditions
kubectl -n scylla get scyllacluster <cluster-name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'

# ScyllaDBDatacenter conditions (v1alpha1 API)
kubectl -n scylla get scylladbdatacenter <dc-name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'

# StatefulSet rollout status
kubectl -n scylla get statefulset -l scylla/cluster=<cluster-name>
```

A finished rollout has `Available=True`, `Progressing=False`, `Degraded=False`. If the rollout is stuck:

| Condition / Reason | Meaning | Action |
|---|---|---|
| `StatefulSetControllerProgressing` with reason `WaitingForStatefulSetRollout` | StatefulSet replicas not yet ready or updated | Check the pod that is not becoming Ready — see [Node not starting](diagnose-node-not-starting.md) |
| `StatefulSetControllerAvailable` with reason `MembersNotReady` | Not all nodes are ready | Check individual pod status and logs |
| `StatefulSetControllerAvailable` with reason `MembersNotUpdated` | Not all nodes match the desired spec | Rollout is still in progress — wait, or check for stuck pods |
| `StatefulSetControllerAvailable` with reason `RacksNotAtDesiredVersion` | Version mismatch between racks | ScyllaDB version upgrade is in progress |
| `StatefulSetControllerProgressing` with reason `RunningUpgradeHooks` | Upgrade hook state machine is active | Upgrade is running pre/post hooks (schema agreement, snapshots) |
| `Degraded=True` with reason `Error` | A controller encountered an error | Check Operator logs for details |

For more details on upgrades, see [Upgrading the Operator](../upgrade/upgrade-operator.md) and [Upgrading ScyllaDB](../upgrade/upgrade-scylladb.md).

### Pod restarts

See [Investigating restarts](investigate-restarts.md).

### Failed replace

See [Recovering from failed replace](recover-from-failed-replace.md).

### Performance issues

See [Performance troubleshooting](troubleshoot-performance.md).

## Quick diagnostic commands

Copy-paste these commands to gather initial diagnostic data. Replace `scylla` with your ScyllaDB namespace if different.

```bash
# Operator status
kubectl -n scylla-operator get deploy,pods

# ScyllaCluster status and conditions
kubectl -n scylla get scyllacluster -o wide
kubectl -n scylla get scyllacluster <cluster-name> -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}){"\n"}{end}'

# Pod status
kubectl -n scylla get pods -o wide

# Recent events (last 20)
kubectl -n scylla get events --sort-by='.lastTimestamp' | tail -20

# Cluster topology (run on any ScyllaDB pod)
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool status

# StatefulSet rollout status
kubectl -n scylla get statefulset -l scylla/cluster=<cluster-name>

# Collect everything for a support ticket
# See: collect-debugging-information/must-gather
```

## Related pages

- [Cluster health](check-cluster-health.md)
- [Node not starting](diagnose-node-not-starting.md)
- [Investigating restarts](investigate-restarts.md)
- [Installation issues](troubleshoot-installation.md)
- [Recovering from failed replace](recover-from-failed-replace.md)
- [Performance](troubleshoot-performance.md)
- [Collecting debugging information](collect-debugging-information/index.md)
