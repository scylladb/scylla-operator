# Troubleshoot performance

## CPU pinning

Diagnose and fix issues with CPU pinning for ScyllaDB Pods.

### Verify CPU pinning

Check QoS class:

:::{code-block} console
kubectl get pods -l 'app.kubernetes.io/name=scylla' \
  -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.qosClass}{"\n"}{end}'
:::

All ScyllaDB Pods should report `Guaranteed`.

Check ScyllaDB startup flags — when CPU pinning is active, ScyllaDB starts with `--overprovisioned=0`:

:::{code-block} console
kubectl logs <pod-name> -c scylla | grep -E 'overprovisioned|smp|cpuset'
:::

Check that the `ContainerPerftune` Job ran for each ScyllaDB Pod:

:::{code-block} console
kubectl -n scylla-operator-node-tuning get jobs \
  -l scylla-operator.scylladb.com/node-config-job-type=ContainerPerftune
:::

### Common issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| `qosClass: Burstable` | `agentResources` has no limits, or requests ≠ limits on any container | Set matching requests and limits on **both** `resources` and `agentResources` |
| `--overprovisioned=1` in logs | Pod is not Guaranteed QoS, or kubelet lacks static CPU policy | Check QoS class and kubelet configuration |
| No `ContainerPerftune` job | Pod is Burstable — tuning is skipped for non-Guaranteed Pods | Fix QoS class |
| `cpuset.cpus.effective` shows all CPUs | Kubelet not using static policy, or CPU request is fractional | Enable `cpuManagerPolicy: static` and use integer CPU values |
| Latency spikes despite pinning | IRQ affinity not set — perftune job failed or did not run | Verify `NodeConfig` `disableOptimizations` is `false` (default) and check perftune job logs |

## See also

- [Configure CPU pinning](../deploy-scylladb/before-you-deploy/configure-cpu-pinning.md) — how to set up CPU pinning.
