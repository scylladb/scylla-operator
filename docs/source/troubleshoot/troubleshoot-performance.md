# Troubleshoot performance

Diagnose performance degradation in ScyllaDB clusters running on Kubernetes.

## Quick performance checklist

Before diving into detailed diagnostics, verify these common performance prerequisites:

```bash
NAMESPACE=scylla
POD=<pod-name>

# 1. Check CPU pinning
kubectl -n "${NAMESPACE}" exec -it "${POD}" -c scylla -- cat /proc/self/status | grep Cpus_allowed_list

# 2. Check resource limits (should be Guaranteed QoS)
kubectl -n "${NAMESPACE}" get pod "${POD}" -o jsonpath='{.status.qosClass}'

# 3. Check io_properties
kubectl -n "${NAMESPACE}" exec -it "${POD}" -c scylla -- cat /etc/scylla.d/io_properties.yaml

# 4. Check for large partition warnings
kubectl -n "${NAMESPACE}" logs "${POD}" -c scylla | grep -i "large partition"
```

## CPU pinning not active

### Symptoms

- Higher-than-expected tail latencies.
- Inconsistent throughput across nodes.

### Diagnosis

Verify CPU pinning:

```bash
# Check QoS class — must be Guaranteed
kubectl -n scylla get pod <pod-name> -o jsonpath='{.status.qosClass}'

# Check kubelet CPU manager state (via NodeConfig pod)
kubectl get pods -A -l app=node-config -o wide
kubectl exec -it <nodeconfig-pod> -c node-config -- \
  cat /host/var/lib/kubelet/cpu_manager_state
```

### Resolution

- Ensure CPU requests equal CPU limits in the ScyllaCluster spec.
- Verify the kubelet is configured with `--cpu-manager-policy=static`.
- See [Configure CPU pinning](../deploy-scylladb/configure-cpu-pinning.md).

## Missing or incorrect io_properties

### Symptoms

- ScyllaDB uses default I/O scheduler settings instead of optimized values.
- Lower-than-expected disk throughput.

### Diagnosis

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- cat /etc/scylla.d/io_properties.yaml
```

If the file is missing or contains default values, I/O tuning was not applied.

### Resolution

See [Configuring io_properties](../operate/configure-io-properties.md).

## NodeConfig tuning not applied

### Symptoms

- Suboptimal RAID configuration.
- Missing filesystem tuning (XFS).
- Incorrect sysctl values.

### Diagnosis

```bash
# Check NodeConfig status
kubectl get nodeconfig -o wide

# Verify tuning completed on the node
kubectl get pods -A -l app=node-config -o wide
kubectl logs <nodeconfig-pod> -c node-config
```

### Resolution

See [Configure nodes](../deploy-scylladb/configure-nodes.md) and [Production checklist](../deploy-scylladb/production-checklist.md).

## Co-located workloads causing contention

### Symptoms

- Performance varies depending on what other pods are running on the same node.
- Noisy-neighbor effects.

### Diagnosis

```bash
# Check what else is running on the same node
NODE=$(kubectl -n scylla get pod <pod-name> -o jsonpath='{.spec.nodeName}')
kubectl get pods --all-namespaces --field-selector spec.nodeName="${NODE}" -o wide
```

### Resolution

- Use dedicated node pools with taints and tolerations.
- See [Dedicated node pools](../deploy-scylladb/set-up-dedicated-node-pools.md).

## High compaction backlog

### Symptoms

- Increasing read and write latencies.
- `nodetool compactionstats` shows many pending compactions.

### Diagnosis

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool compactionstats
```

### Resolution

- Wait for compactions to complete.
- Verify that CPU and I/O resources are sufficient.
- Consider temporarily adjusting compaction throughput:
  ```bash
  kubectl -n scylla exec -it <pod-name> -c scylla -- \
    nodetool setcompactionthroughput 256
  ```

## Large partitions

### Symptoms

- Timeouts on specific queries.
- Garbage collection pauses.

### Diagnosis

```bash
# Check logs for large partition warnings
kubectl -n scylla logs <pod-name> -c scylla | grep -i "large"

# Query system tables
kubectl -n scylla exec -it <pod-name> -c scylla -- \
  cqlsh -e "SELECT * FROM system.large_partitions LIMIT 10;"
```

### Resolution

Large partitions are a data modeling issue.
See the [ScyllaDB documentation on large partitions](https://docs.scylladb.com/stable/troubleshoot/large-partition-table.html) for guidance on redesigning the schema.

## Collecting performance diagnostics

When filing a support ticket about performance:

1. Collect a [must-gather archive](collect-debugging-information/must-gather.md).
2. Include monitoring dashboards (Grafana screenshots or exported data) — see [Set up monitoring](../deploy-scylladb/set-up-monitoring.md).
3. Include `nodetool tablestats` output for affected tables:
   ```bash
   kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool tablestats <keyspace>.<table>
   ```
4. Include `nodetool compactionstats` output.
5. Describe the workload pattern (read/write ratio, partition key distribution, query patterns).

## Related pages

- [CPU pinning](../deploy-scylladb/configure-cpu-pinning.md)
- [Configuring io_properties](../operate/configure-io-properties.md)
- [Node configuration](../deploy-scylladb/configure-nodes.md)
- [Production checklist](../deploy-scylladb/production-checklist.md)
- [Diagnostic flowchart](diagnostic-flowchart.md)
