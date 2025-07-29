# Generic

Because {{productName}} aims to leverage the best performance available, there are a few extra steps that need to be configured on your Kubernetes cluster.

## Kubelet

### Static CPU policy

By default, *kubelet* uses the CFS quota to enforce pod CPU limits.
When the Kubernetes node runs a lot of CPU-bound Pods, the processes can move over different CPU cores, depending on whether the Pod is throttled and which CPU cores are available.
However, kubelet may be configured to assign CPUs exclusively by setting the CPU manager policy to static.

To get the best performance and latency ScyllaDB Pods should run under [static CPU policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#configuration) to pin cores.

:::{note}
Configuring kubelet options is provider specific.
We provide a few examples for the major ones later in this section, otherwise please consult the documentation for your Kubernetes platform.
:::

## Nodes

### Labels

For the purposes of the installation guides, we assume that the nodes meant to run ScyllaDB (ScyllaClusters) have label `scylla.scylladb.com/node-type=scylla`.
