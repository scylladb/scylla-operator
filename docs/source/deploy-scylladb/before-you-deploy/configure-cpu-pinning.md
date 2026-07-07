# Configure CPU pinning

CPU pinning ensures that ScyllaDB threads are bound to specific CPU cores, eliminating context-switch overhead and improving tail latency.

## Why CPU pinning matters

ScyllaDB is a shard-per-core database — it assigns each CPU core a dedicated share of data and I/O.
When the kernel schedules other processes or handles interrupts on those same cores, it disrupts ScyllaDB's per-core processing and causes latency spikes.
CPU pinning ensures:

- The kubelet assigns **exclusive CPU cores** to the ScyllaDB container.
- The [`perftune.py`](https://github.com/scylladb/seastar/blob/master/scripts/perftune.py) tuning script configures **IRQ affinity** so that network and disk interrupts are handled by CPUs **not** assigned to ScyllaDB.
- ScyllaDB starts with `--overprovisioned=0`, telling it to assume dedicated cores and optimize accordingly.

## Prerequisites

CPU pinning requires three things to work together:

```{list-table}
:header-rows: 1

* - Requirement
  - Who configures it
* - Kubelet static CPU manager policy
  - Platform administrator (node pool configuration)
* - Guaranteed QoS class on the ScyllaDB Pod
  - `ScyllaCluster` author
* - Performance tuning enabled in `NodeConfig`
  - `NodeConfig` author (enabled by default)
```

If any of these is missing, CPU pinning silently does not apply.

## Enable the static CPU manager policy

The kubelet on each dedicated ScyllaDB node must be started with `cpuManagerPolicy: static`.
This tells the kubelet to assign exclusive CPUs to containers that request integer CPU amounts in Pods with Guaranteed QoS class.

How you configure this depends on your platform:

::::{tabs}

:::{tab} GKE
Pass a `systemconfig.yaml` file when creating the node pool:

```console
cat > systemconfig.yaml <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
```

```console
gcloud container node-pools create scylladb-pool \
  --system-config-from-file='systemconfig.yaml' \
  ...
```

See the [GKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-gke-cluster.md) for a complete example.
:::

:::{tab} EKS
Set `kubeletExtraConfig` in the eksctl cluster configuration:

```yaml
nodeGroups:
- name: scylla-pool
  kubeletExtraConfig:
    cpuManagerPolicy: static
```

See the [EKS cluster setup guide](../../install-operator/provision-infrastructure/set-up-eks-cluster.md) for a complete example.
:::

:::{tab} OKE
Use a Cloud-Init script that writes a kubelet configuration override before the node joins the cluster.
The [OKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-oke-cluster.md) applies this automatically.
:::

:::{tab} OpenShift
Create a `KubeletConfig` resource targeting the `MachineConfigPool` dedicated to your ScyllaDB nodes.
See [Setting up CPU Manager](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/scalability_and_performance/using-cpu-manager#setting_up_cpu_manager_using-cpu-manager-and-topology-manager) in the Red Hat OpenShift documentation for detailed steps.
:::

::::

:::{warning}
Changing the CPU manager policy on a running node requires draining the node, stopping the kubelet, removing the CPU manager state file (`/var/lib/kubelet/cpu_manager_state`), and restarting the kubelet.
It is much simpler to set the policy at node creation time.
:::

## Set the Guaranteed QoS class

A Pod receives Guaranteed QoS class when **every container** has CPU and memory requests equal to its limits.
For ScyllaDB Pods, this means both the ScyllaDB container (`resources`) and the ScyllaDB Manager Agent sidecar (`agentResources`) must have matching requests and limits.

:::{caution}
The default `agentResources` has **only requests** (50m CPU, 10Mi memory) and **no limits**.
If you set `resources` with matching requests and limits but forget to also set `agentResources` limits, the Pod will be **Burstable** QoS and CPU pinning will silently not work.
:::

Example:

:::{code-block} yaml
spec:
  datacenter:
    racks:
    - name: us-east-1a
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
:::

:::{note}
Use **integer CPU values** (e.g., `4`, not `4000m`) for the ScyllaDB container.
The kubelet only assigns exclusive cores for integer CPU requests.
Millicpu values result in a shared CPU pool.
:::

## Verify

To verify CPU pinning is active and troubleshoot common issues, see [Troubleshoot performance](../../troubleshoot/troubleshoot-performance.md).
