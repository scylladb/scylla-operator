# Investigate pod restarts

Determine why a ScyllaDB pod or container restarted and collect the evidence needed for diagnosis or a support ticket.

## Identify that a restart occurred

Check the restart count:

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node
```

A non-zero `RESTARTS` column indicates that one or more containers in the pod have restarted.

You can also compare the container start time against the pod creation time.
If the container started significantly later than the pod was created, the container has restarted:

```bash
kubectl -n scylla get pod <pod-name> -o jsonpath='Pod created: {.metadata.creationTimestamp}{"\n"}Container started: {.status.containerStatuses[?(@.name=="scylla")].state.running.startedAt}{"\n"}'
```

## Determine the restart reason

### Container status

```bash
kubectl -n scylla get pod <pod-name> -o jsonpath='{.status.containerStatuses}' | jq .
```

Key fields:

| Field | Description |
|---|---|
| `restartCount` | Total number of restarts for this container |
| `lastState.terminated.reason` | Why the container stopped (`OOMKilled`, `Error`, `Completed`) |
| `lastState.terminated.exitCode` | Process exit code (`137` = SIGKILL / OOMKilled, `1` = error) |
| `lastState.terminated.finishedAt` | Timestamp of the last termination |

### Pod events

```bash
kubectl -n scylla describe pod <pod-name>
```

Look for these events in the `Events` section:

| Event | Meaning |
|---|---|
| `Killing` | Container was killed (by kubelet or OOM killer) |
| `BackOff` | Container is in `CrashLoopBackOff` — restarting repeatedly |
| `OOMKilling` | Container exceeded its memory limit |
| `Unhealthy` | Liveness probe failed — kubelet killed the container |
| `FailedScheduling` | Pod cannot be placed on any node |

## Distinguish restart causes

### OOMKilled

**Indicators:**
- `lastState.terminated.reason: OOMKilled`
- `lastState.terminated.exitCode: 137`

**Common causes:**
- Memory limit too low for the workload.
- ScyllaDB memory allocation exceeds the container limit.

**Resolution:**
- Increase the memory limit in the ScyllaCluster spec.
- Review ScyllaDB memory usage via monitoring dashboards.

### Liveness probe failure

**Indicators:**
- Event: `Unhealthy` with `Liveness probe failed`
- Container restarted without `OOMKilled` reason.

**Common causes:**
- ScyllaDB unresponsive due to long GC pauses or compaction stalls.
- Node overloaded — too many concurrent operations.

**Resolution:**
- Check ScyllaDB logs for compaction or GC warnings.
- Review resource allocation (CPU, memory).
- Check for large partition warnings in logs.

### CrashLoopBackOff

**Indicators:**
- Pod status: `CrashLoopBackOff`
- Event: `BackOff`

**Common causes:**
- ScyllaDB fails to start — corrupt SSTables, invalid configuration, wrong seeds.
- Disk permission issues.
- Missing or invalid `io_properties.yaml`.

**Resolution:**
- Check previous container logs: `kubectl -n scylla logs <pod-name> -c scylla --previous`
- Verify configuration with `kubectl -n scylla describe scyllacluster <cluster-name>`

### Node eviction

**Indicators:**
- Pod event: `Evicted`
- Node conditions show `MemoryPressure` or `DiskPressure`.

**Cause:** The Kubernetes node is under resource pressure and the kubelet evicted the pod.

**Resolution:**
- Check node conditions: `kubectl describe node <node-name>`
- Ensure dedicated node pools with appropriate taints prevent co-scheduling with other workloads.
- See [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md).

## Collect evidence

When filing a support ticket or investigating further, collect:

1. **Previous container logs:**
   ```bash
   kubectl -n scylla logs <pod-name> -c scylla --previous
   ```

2. **Full pod YAML with status:**
   ```bash
   kubectl -n scylla get pod <pod-name> -o yaml
   ```

3. **Pod events:**
   ```bash
   kubectl -n scylla describe pod <pod-name>
   ```

4. **must-gather archive:**

   Collect a must-gather archive for escalation or offline analysis:

   ```bash
   podman run -it --pull=always --rm \
     -v="${KUBECONFIG}:/kubeconfig:ro,Z" \
     -v="$(pwd):/workspace:Z" \
     --workdir=/workspace \
     docker.io/scylladb/scylla-operator:latest \
     must-gather --kubeconfig=/kubeconfig
   ```

   The archive contains the previous pod logs (`logs.previous`) needed to diagnose restarts.
   See [must-gather](collect-debugging-information/must-gather.md) for full options (including Docker and `--all-resources`) and [must-gather contents](collect-debugging-information/must-gather-contents.md) for a guide to interpreting the files.

## Related pages

- [Diagnostic flowchart](diagnostic-flowchart.md)
- [Node not starting](diagnose-node-not-starting.md)
- [Cluster health](check-cluster-health.md)
- [Collecting debugging information](collect-debugging-information/index.md)
