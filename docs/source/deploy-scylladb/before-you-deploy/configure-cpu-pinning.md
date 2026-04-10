# Configure CPU pinning

This page explains how to configure CPU pinning for ScyllaDB on Kubernetes so that each ScyllaDB container gets exclusive CPU cores and network/disk IRQs are steered away from those cores.

## Why CPU pinning matters

ScyllaDB is a shard-per-core database — it assigns each CPU core a dedicated share of data and I/O. When the kernel schedules other processes or handles interrupts on those same cores, it disrupts ScyllaDB's per-core processing and causes latency spikes. CPU pinning ensures:

- The kubelet assigns **exclusive CPU cores** to the ScyllaDB container.
- The `perftune.py` tuning script configures **IRQ affinity** so that network and disk interrupts are handled by CPUs **not** assigned to ScyllaDB.
- ScyllaDB starts with `--overprovisioned=0`, telling it to assume dedicated cores and optimize accordingly.

## Prerequisites

CPU pinning requires three things to work together:

| Requirement | Who configures it |
|-------------|-------------------|
| Kubelet static CPU manager policy | Platform administrator (node pool configuration) |
| Guaranteed QoS class on the ScyllaDB pod | ScyllaCluster author |
| Performance tuning enabled in NodeConfig | NodeConfig author (enabled by default) |

If any of these is missing, CPU pinning silently does not apply.

## Step 1: Configure the kubelet

The kubelet must run with `cpuManagerPolicy: static`. This tells the kubelet to assign exclusive CPUs to containers that request integer CPU amounts in pods with Guaranteed QoS class.

::::{tabs}
:::{group-tab} GKE

Create a system config file:

```shell
cat > systemconfig.yaml <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
```

Pass it when creating the node pool:

```shell
gcloud container node-pools create scylladb-pool \
  --system-config-from-file='systemconfig.yaml' \
  ...
```
:::

:::{group-tab} EKS

Set `kubeletExtraConfig` in the eksctl cluster configuration:

```yaml
nodeGroups:
- name: scylla-pool
  instanceType: i4i.2xlarge
  kubeletExtraConfig:
    cpuManagerPolicy: static
  ...
```
:::

:::{group-tab} OpenShift

Create a `KubeletConfig` resource targeting the ScyllaDB MachineConfigPool:

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: KubeletConfig
metadata:
  name: scylladb-cpumanager
spec:
  kubeletConfig:
    cpuManagerPolicy: static
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/scylladb: ""
```
:::

:::{group-tab} Generic

Edit the kubelet configuration file (typically `/var/lib/kubelet/config.yaml`) on each dedicated node:

```yaml
cpuManagerPolicy: static
```

Restart the kubelet after the change.
:::
::::

## Step 2: Set Guaranteed QoS class

A pod receives Guaranteed QoS class when **every container** has CPU and memory requests equal to its limits. For ScyllaDB pods, this means both the ScyllaDB container (`resources`) and the ScyllaDB Manager Agent sidecar (`agentResources`) must have matching requests and limits.

:::{caution}
The default `agentResources` has **only requests** (50m CPU, 10Mi memory) and **no limits**. If you set `resources` with matching requests and limits but forget to also set `agentResources` limits, the pod will be **Burstable** QoS and CPU pinning will silently not work.
:::

### ScyllaCluster example

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  datacenter:
    racks:
    - name: us-east-1a
      members: 3
      resources:
        requests:
          cpu: 4
          memory: 32Gi
        limits:
          cpu: 4
          memory: 32Gi
      agentResources:
        requests:
          cpu: 50m
          memory: 10Mi
        limits:
          cpu: 50m
          memory: 10Mi
      # ... storage, placement
```

:::{note}
Use **integer CPU values** (e.g., `4`, not `4000m`) for the ScyllaDB container. The kubelet only assigns exclusive cores for integer CPU requests. Millicpu values result in a shared CPU pool.
:::

## Step 3: Verify CPU pinning

### Check QoS class

```shell
kubectl get pod <pod-name> -o jsonpath='{.status.qosClass}'
```

Expected output: `Guaranteed`

### Check ScyllaDB startup flags

```shell
kubectl logs <pod-name> -c scylla | grep -E 'overprovisioned|smp|cpuset'
```

When CPU pinning is active, ScyllaDB starts with `--overprovisioned=0` and a specific `--cpuset` matching the pinned cores. When pinning is **not** active, you will see `--overprovisioned=1`.

### Check cgroup CPU assignment

```shell
kubectl exec <pod-name> -c scylla -- cat /sys/fs/cgroup/cpuset.cpus.effective
```

On a properly pinned pod, this shows specific CPU IDs (e.g., `2-5`). On a non-pinned pod, it shows all CPUs on the node.

### Check perftune job

The Operator creates a `ContainerPerftune` Job for each Guaranteed QoS ScyllaDB pod. Check if it ran successfully:

```shell
kubectl -n scylla-operator-node-tuning get jobs -l scylla-operator.scylladb.com/node-config-job-type=ContainerPerftune
```

If no jobs appear, the pod likely did not receive Guaranteed QoS class.

## How it works internally

When a ScyllaDB pod has Guaranteed QoS:

1. The kubelet assigns exclusive CPU cores to the pod's containers.
2. The NodeConfig DaemonSet's `ContainerPerftune` Job queries the kubelet's [PodResources API](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#monitoring-device-plugin-resources) to discover which CPUs are assigned to the ScyllaDB container.
3. It computes the IRQ CPU mask as the **complement**: all host CPUs minus ScyllaDB CPUs.
4. It runs `perftune.py --irq-cpu-mask=<mask> --tune=net --tune=disks` to configure IRQ affinity on the host.
5. The sidecar ignition mechanism blocks ScyllaDB startup until tuning completes (see [Ignition](../../understand/ignition.md)).
6. ScyllaDB starts with `--overprovisioned=0 --smp=<count> --cpuset=<cpus>`.

## Common pitfalls

| Symptom | Cause | Fix |
|---------|-------|-----|
| `qosClass: Burstable` | `agentResources` has no limits, or requests ≠ limits on any container. | Set matching requests and limits on **both** `resources` and `agentResources`. |
| `--overprovisioned=1` in logs | Pod is not Guaranteed QoS, or kubelet is not configured with static CPU policy. | Check QoS class and kubelet configuration. |
| No `ContainerPerftune` job | Pod is Burstable — tuning is skipped entirely for non-Guaranteed pods. | Fix QoS class. |
| `cpuset.cpus.effective` shows all CPUs | Kubelet is not using static CPU policy, or CPU request is fractional (e.g., `500m`). | Enable `cpuManagerPolicy: static` on the kubelet and use integer CPU values. |
| Latency spikes despite pinning | IRQ affinity not set — check perftune job logs. | Verify NodeConfig `disableOptimizations` is `false` (default) and the perftune job completed. |

## Related pages

- [Tuning architecture](../../understand/tuning.md) — how node-level and container-level tuning work.
- [Set up dedicated node pools](set-up-dedicated-node-pools.md) — setting up nodes with the correct kubelet configuration.
- [Configure nodes](configure-nodes.md) — NodeConfig resource for disk and kernel tuning.
- [Deploy your first cluster](../deploy-your-first-cluster.md) — setting resource requests and limits.
- [Production checklist](../production-checklist.md) — CPU pinning as a production requirement.
