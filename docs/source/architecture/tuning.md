# Tuning

ScyllaDB works best when it's pinned to the CPUs and not interrupted.
To get the best performance and latency we recommend you set up your ScyllaDB with [cpu pinning using the static CPU policy](../installation/kubernetes-prerequisites.md#static-cpu-policy).

One of the most common causes of context-switching are network interrupts.
Packets coming to a Kubernetes node need to be processed which requires using CPU shares.  

On a Kubernetes node there is always a couple of other processes running, like kubelet, Kubernetes provider applications, daemons and others. 
These processes require CPU shares, so we cannot dedicate entire node processing power to Scylla, we need to leave space for others.  
We take advantage of it, and we pin IRQs to CPUs not used by any ScyllaDB Pods exclusively.

Performance tuning is enabled by default **when you create a corresponding [NodeConfig](../resources/nodeconfigs.md) for your nodes**.

Because some of the operations it needs to perform are not multitenant or require elevated privileges, the tuning scripts are ran in a dedicated system namespace called `scylla-operator-node-tuning`.
This namespace is created and entirely managed by {{productName}} and only administrators can access it.

The tuning is based around `perftune` script that comes from [scyllaDBUtilsImage](#api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.status). `perftune` executes the performance optmizations like tuning the kernel, network, disk devices, spreading IRQs across CPUs and more. Conceptually, this is run in 2 parts: tuning the [Kubernetes nodes](#kubernetes-nodes) and tuning for [ScyllaDB Pods](#scylladb-pods).

:::{include} ../.internal/tuning-warning.md
:::

## Kubernetes nodes

`perftune` script is executed on the targeted Kubernetes nodes and it tunes kernel, network, disk devices and more.
This is executed right after the tuning is enabled using a [NodeConfig](../resources/nodeconfigs.md)

## ScyllaDB Pods

When a [ScyllaCluster](../resources/scyllaclusters/basics.md) Pod is created (and performance tuning is enabled), the Pod initializes but waits until {{productName}} runs an on-demand Job that will configure the host and the ScyllaDB process accordingly (e.g. spreading IRQs across other CPUs).
Only after that it will actually start running ScyllaDB.

:::{include} ../.internal/tuning-qos-caution.md
:::
